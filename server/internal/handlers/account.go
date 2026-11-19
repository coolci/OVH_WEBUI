package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

func noOVHRespAccount(c *gin.Context) {
	if statusVal, exists := c.Get(ctxOVHErrStatusKey); exists {
		status, _ := statusVal.(int)
		code := c.GetString(ctxOVHErrCodeKey)
		msg := c.GetString(ctxOVHErrMsgKey)
		if status == http.StatusNotFound || code == "ACCOUNT_PROXY_INVALID" {
			c.JSON(status, gin.H{"error": msg, "code": code})
			return
		}
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "未配置OVH API", "code": "E855DF392"})
}

// ── 带失败计数的并发详情拉取 ──────────────────────────────────────────────
//
// util.go 的 parallelGetDetails / parallelGetStringKeys 把失败位留成 nil,调用方一过滤
// 记录就凭空消失,接口照样 200,日志里的"成功 N 条"也统计不出少了几条。
// util.go 是公共文件不在本次改动范围,所以这里放一份带计数的版本:
// 拿不到的详情等于整条记录丢失,必须让日志和响应能看见。

const (
	ovhDetailRetryDelay = 300 * time.Millisecond
	ovhDetailTimeout    = 20 * time.Second
)

// ovhErrIsTransient 429(OVH 对 /me 有速率限制,这里按 10 并发拉详情很容易撞上)和 5xx
// 属于重试有意义的瞬时错误;403/404 是权限或资源本身的问题,重试只是白白再烧一次配额。
// 非 APIError(连接超时/重置)同样按瞬时处理。
func ovhErrIsTransient(err error) bool {
	var apiErr *ovhsdk.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusTooManyRequests || apiErr.Code >= 500
	}
	return true
}

// ovhGetWithRetry 对瞬时错误退避重试一次。宁可多花 300ms 也不要让一条记录悄悄消失。
//
// 单条详情(含重试)总耗时封顶 ovhDetailTimeout:实测 /me/notification/email/history/{id}
// 偶发 90s 才回 504,而 http client 超时是 60s、504 又算瞬时错误会再等一轮 ——
// 一封邮件就能把整个「邮件历史」请求拖到两分钟以上,前端一直转圈。
// 超时的那条按失败计入(走 X-Partial-Failures / failed 上报),不拖垮整张列表。
func ovhGetWithRetry(client *ovhsdk.Client, path string, out interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), ovhDetailTimeout)
	defer cancel()
	err := client.GetWithContext(ctx, path, out)
	if err == nil || !ovhErrIsTransient(err) || ctx.Err() != nil {
		return err
	}
	time.Sleep(ovhDetailRetryDelay)
	return client.GetWithContext(ctx, path, out)
}

// parallelGetPaths 并发 GET 一批路径,结果按索引对齐(失败位 nil),
// 额外返回失败条数和第一个错误,供调用方上报。
func parallelGetPaths(client *ovhsdk.Client, paths []string, concurrency int) ([]map[string]interface{}, int, error) {
	if concurrency <= 0 {
		concurrency = 10
	}
	results := make([]map[string]interface{}, len(paths))
	errs := make([]error, len(paths))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, path string) {
			defer wg.Done()
			defer func() { <-sem }()
			if path == "" {
				errs[idx] = errUnsafeOVHSegment
				return
			}
			isolated, err := ovh.IsolateClient(client)
			if err != nil {
				errs[idx] = err
				return
			}
			var d map[string]interface{}
			if err := ovhGetWithRetry(isolated, path, &d); err != nil {
				errs[idx] = err
				return
			}
			results[idx] = d
		}(i, p)
	}
	wg.Wait()
	failed := 0
	var firstErr error
	for _, e := range errs {
		if e != nil {
			failed++
			if firstErr == nil {
				firstErr = e
			}
		}
	}
	return results, failed, firstErr
}

// parallelGetDetailsCounted parallelGetDetails 的带计数版本
func parallelGetDetailsCounted(client *ovhsdk.Client, keys []interface{}, pathFn func(interface{}) string, concurrency int) ([]map[string]interface{}, int, error) {
	paths := make([]string, len(keys))
	for i, k := range keys {
		paths[i] = pathFn(k)
	}
	return parallelGetPaths(client, paths, concurrency)
}

// parallelGetStringsCounted parallelGetStringKeys 的带计数版本
func parallelGetStringsCounted(client *ovhsdk.Client, keys []string, pathFn func(string) string, concurrency int) ([]map[string]interface{}, int, error) {
	paths := make([]string, len(keys))
	for i, k := range keys {
		paths[i] = pathFn(k)
	}
	return parallelGetPaths(client, paths, concurrency)
}

// collectDetails 过滤掉失败位,并在有失败时写日志 + 带回响应头,
// 让"列表少了几条"这件事至少能被看见(refunds / email-history 前端要的是裸数组,
// 不能往 body 里塞字段,只能走 header)。
func collectDetails(state *app.State, c *gin.Context, details []map[string]interface{}, failed int, firstErr error, what, category string) []map[string]interface{} {
	list := []map[string]interface{}{}
	for _, d := range details {
		if d != nil {
			list = append(list, d)
		}
	}
	if failed > 0 {
		msg := ""
		if firstErr != nil {
			msg = firstErr.Error()
		}
		state.Logger.Warn(fmt.Sprintf("%s详情有 %d/%d 条拉取失败,返回列表不完整: %s", what, failed, len(details), msg), category)
		c.Header("X-Partial-Failures", strconv.Itoa(failed))
		if len(list) == 0 {
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": what + "详情读取失败，请重试", "failed": failed})
		}
	}
	return list
}

// ── 账单 / 退款列表 ────────────────────────────────────────────────────────

// SubsidiaryMismatchNote 比对「账户配置里的 zone」和「OVH 实际认的 ovhSubsidiary」。
//
// 为什么必须查:zone 是全项目区域推导的唯一输入 —— 它决定目录站点
// (ovh.CatalogBaseURLForSubsidiary)、eco 目录的 ovhSubsidiary 参数、
// 下单的 region 取值。而 nichandle.Nichandle.ovhSubsidiary(EU/US/CA 三区 schema 里都是必填)
// 才是 OVH 自己认的那一个。两者不一致有两档后果:
//   - 同大区不一致(配 FR、实际 IE):目录/价格是另一个国家的,下单价与页面价对不上
//   - 跨大区不一致(配 FR、实际 US):endpoint 也跟着推错,所有调用打到别的站点,
//     表现为"目录里没有这个机型 / 账户突然失效",根因完全看不出来
//
// 只提示不改写:改 zone 是用户对凭据归属的决定,后台偷偷改会让下一次下单换个国家的价格。
// 返回空字符串表示没问题(取不到 ovhSubsidiary 时也当没问题,不制造假警报)。
func SubsidiaryMismatchNote(acc types.OVHAccount, me map[string]interface{}) string {
	actual, _ := me["ovhSubsidiary"].(string)
	actual = strings.ToUpper(strings.TrimSpace(actual))
	if actual == "" {
		return ""
	}
	configured := strings.ToUpper(strings.TrimSpace(acc.Zone))
	if configured == "" {
		configured = ovh.DefaultSubsidiaryForEndpoint(acc.Endpoint)
	}
	if configured == actual {
		return ""
	}
	cfgRegion, actRegion := ovh.SubsidiaryRegion(configured), ovh.SubsidiaryRegion(actual)
	if cfgRegion != actRegion {
		return fmt.Sprintf("账户配置的子公司是 %s(%s 区),但 OVH 返回的 ovhSubsidiary 是 %s(%s 区)——"+
			"这两个区的目录、价格、库存、购物车完全独立,当前配置会把请求打到错误的站点,请把 zone 改成 %s 并使用 endpoint %s",
			configured, cfgRegion, actual, actRegion, actual, ovh.EndpointForSubsidiary(actual))
	}
	return fmt.Sprintf("账户配置的子公司是 %s,但 OVH 返回的 ovhSubsidiary 是 %s(同属 %s 区)——"+
		"站点没打错,但目录和价格会按 %s 取,与实际结算币种/价格可能不一致,建议把 zone 改成 %s",
		configured, actual, cfgRegion, configured, actual)
}

// GetAccountInfo GET /api/ovh/account/info
func GetAccountInfo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var info map[string]interface{}
		if err := client.Get("/me", &info); err != nil {
			state.Logger.Error("获取账户信息失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取账户信息失败: " + ovh.Explain(err)})
			return
		}
		// 这个接口的响应体是 OVH 的 nichandle 原样透传(前端按 OVH 字段渲染),
		// 不能往里塞自定义字段,所以子公司错配只能走 header + 日志。
		if acc, ok := ovhAccountFor(state, c); ok {
			if note := SubsidiaryMismatchNote(acc, info); note != "" {
				state.Logger.Warn("账户 "+acc.Name+" 子公司配置与 OVH 实际归属不一致:"+note, "account_management")
				c.Header("X-Subsidiary-Mismatch", "1")
			}
		}
		state.Logger.Info("成功获取账户信息", "account_management")
		c.JSON(http.StatusOK, info)
	}
}

// GetCreditBalance GET /api/ovh/account/credit-balance
func GetCreditBalance(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var names []string
		if err := client.Get("/me/credit/balance", &names); err != nil {
			state.Logger.Error("获取信用余额失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取信用余额失败: " + ovh.Explain(err)})
			return
		}
		// 并发拉详情
		details, failed, firstErr := parallelGetStringsCounted(client, names, func(n string) string {
			return safePathFn("me", "credit", "balance", n)
		}, 10)
		balances := collectDetails(state, c, details, failed, firstErr, "信用余额", "account_management")
		if c.IsAborted() {
			return
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 个信用余额(共 %d 个,失败 %d 个)", len(balances), len(names), failed), "account_management")
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": balances, "total": len(names), "failed": failed})
	}
}

// GetEmailHistory GET /api/ovh/account/email-history
func GetEmailHistory(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/me/notification/email/history", &ids); err != nil {
			state.Logger.Error("获取邮件历史失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取邮件历史失败: " + ovh.Explain(err)})
			return
		}
		// 倒序
		for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
			ids[i], ids[j] = ids[j], ids[i]
		}
		max := 50
		if len(ids) < max {
			max = len(ids)
		}
		// 并发拉 50 封邮件详情：原 50 * 200ms = 10 秒 -> 5 * 200ms = 1 秒
		details, failed, firstErr := parallelGetDetailsCounted(client, ids[:max], func(k interface{}) string {
			return safePathFn("me", "notification", "email", "history", idToString(k))
		}, 10)
		list := collectDetails(state, c, details, failed, firstErr, "邮件", "account_management")
		if c.IsAborted() {
			return
		}
		// 口径:请求了 max 封、失败 failed 封。len(ids) 是账户的全量历史(可能上千),
		// 写成"总共"会让人以为差额是丢了,其实是我们主动只取最近 max 封。
		state.Logger.Info(fmt.Sprintf("成功获取 %d 封邮件(本次请求最近 %d 封,失败 %d 封;账户历史共 %d 封)",
			len(list), max, failed, len(ids)), "account_management")
		c.JSON(http.StatusOK, list)
	}
}

// GetSubAccounts GET /api/ovh/account/sub-accounts
func GetSubAccounts(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/me/subAccount", &ids); err != nil {
			state.Logger.Error("获取子账户列表失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取子账户列表失败: " + ovh.Explain(err)})
			return
		}
		// 并发拉详情
		details, failed, firstErr := parallelGetDetailsCounted(client, ids, func(k interface{}) string {
			return safePathFn("me", "subAccount", idToString(k))
		}, 10)
		list := collectDetails(state, c, details, failed, firstErr, "子账户", "account_management")
		if c.IsAborted() {
			return
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 个子账户(共 %d 个,失败 %d 个)", len(list), len(ids), failed), "account_management")
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": list, "total": len(ids), "failed": failed})
	}
}

// GetAccountSSHKeys GET /api/ovh/account/ssh-keys
// 拉取当前账户在 OVH 登记的全部 SSH 密钥名称列表（GET /me/sshKey）
func GetAccountSSHKeys(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var keys []string
		if err := client.Get("/me/sshKey", &keys); err != nil {
			respondOVHError(c, err)
			return
		}
		if keys == nil {
			keys = []string{}
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"keys":    keys,
		})
	}
}
