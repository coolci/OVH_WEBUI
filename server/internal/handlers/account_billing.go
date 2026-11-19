package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// /me/bill 与 /me/refund 都只返回 id 数组(schema: string[]),对返回顺序没有任何承诺,
// 所以"最近 20 条"不能靠截断 id 列表得到,必须用 schema 提供的 date.from 过滤 +
// 详情里的 date 字段排序(billing.Bill.date / billing.Refund.date 都是必填 datetime)。
// date.from 在 EU / US / CA 三区的这两个端点上都是合法 query 参数(已逐一核对 schema)。

const (
	accountBillingListSize   = 20  // 最终返回给前端的条数
	accountBillingMaxDetails = 100 // 单次最多拉多少条详情,避免老账户把一次请求拖成分钟级
)

// billingListWindows date.from 的候选窗口(月),从窄到宽
var billingListWindows = []int{1, 3, 12, 36}

// fetchRecentBillingIDs 逐步放宽 date.from,第一个能凑够 want 条的窗口即采用;
// 所有窗口都不够(新账户)才退回不带过滤的全量列表。
// 先窄后宽是为了在账单密集的账户上少拉详情,同时保证拿到的是最近的而不是随机的一批。
//
// 第二个返回值是降级说明:窗口查询失败时不再直接报错,而是退回改动前的裸路径全量列表,
// 让调用方能记日志/提示,但页面照常能开。
// 原因:US 的 /me/refund、/me/bill 的 date.from 在 schema 里是 string 而不是 EU 的 datetime,
// OVH US 认不认 "2026-08-25T00:00:00+00:00" 这种写法无法离线确认;
// 而这两个端点是「退款记录」「账单」两个 tab 的唯一数据源,一旦被拒就是整页 500,
// 代价远大于"少一层服务端过滤、多拉几条详情"。
func fetchRecentBillingIDs(client *ovhsdk.Client, baseSegs []string, want int) ([]string, string, error) {
	basePath, err := buildOVHPath(baseSegs...)
	if err != nil {
		return nil, "", err
	}
	var ids []string
	warn := ""
	for _, months := range billingListWindows {
		from := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02T15:04:05+00:00")
		qPath, qErr := buildOVHPathQuery(url.Values{"date.from": {from}}, baseSegs...)
		if qErr != nil {
			return nil, "", qErr
		}
		var raw []interface{}
		if err := client.Get(qPath, &raw); err != nil {
			warn = fmt.Sprintf("date.from 窗口查询(%s)被拒绝,已降级为全量列表 + 本地排序: %s", from, err.Error())
			ids = nil
			break
		}
		ids = idsToStrings(raw)
		if len(ids) >= want {
			return ids, "", nil
		}
	}
	var raw []interface{}
	if err := client.Get(basePath, &raw); err != nil {
		return nil, warn, err
	}
	if all := idsToStrings(raw); len(all) > len(ids) {
		ids = all
	}
	// 全量列表 OVH 不承诺顺序,而下游只会取前 accountBillingMaxDetails 条拉详情,
	// 取错一批就等于"最近 20 条"里没有真正最近的。id 在这两个端点上都是随时间递增的,
	// 所以先按 id 倒序做个尽力而为的近似(拿到详情后仍会按 date 重新排,这里只影响取样)。
	sortBillingIDsDesc(ids)
	return ids, warn, nil
}

// newestBillingIDs 按 id 倒序后截断。
// GET /me/order、/me/bill、/me/refund 都不保证返回顺序,而 id 随时间递增。
// 截断发生在拉详情之前;不排序就会把窗口里最老的一批当成“最近 N 条”。
func newestBillingIDs(ids []string, max int) ([]string, bool) {
	sortBillingIDsDesc(ids)
	if max > 0 && len(ids) > max {
		return ids[:max], true
	}
	return ids, false
}

// sortBillingIDsDesc 账单/退款 id 倒序:能当整数比就按数值比(避免 "999" > "1000"),
// 否则退回"长的在前、同长按字典序倒序",对 OVH 常见的定长前缀 id 同样成立。
func sortBillingIDsDesc(ids []string) {
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := ids[i], ids[j]
		na, ea := strconv.ParseInt(a, 10, 64)
		nb, eb := strconv.ParseInt(b, 10, 64)
		if ea == nil && eb == nil {
			return na > nb
		}
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		return a > b
	})
}

// idsToStrings OVH 这几个列表端点 schema 写的是 string[],但历史上也出现过数字 id,
// 沿用 idToString 的宽松处理。
func idsToStrings(raw []interface{}) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s := idToString(v); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// sortBillingByDateDesc 按详情的 date 倒序。OVH 的 datetime 带时区偏移,
// 直接字符串比较会把不同偏移的同一时刻排错,所以先按时间解析,解析不出来才退回字符串比较。
func sortBillingByDateDesc(list []map[string]interface{}) {
	sort.SliceStable(list, func(i, j int) bool {
		a, _ := list[i]["date"].(string)
		b, _ := list[j]["date"].(string)
		ta, ea := parseFlexible(a)
		tb, eb := parseFlexible(b)
		if ea == nil && eb == nil {
			return ta.After(tb)
		}
		if ea == nil {
			return true
		}
		if eb == nil {
			return false
		}
		return a > b
	})
}

// GetAccountRefunds GET /api/ovh/account/refunds
func GetAccountRefunds(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		ids, warn, err := fetchRecentBillingIDs(client, []string{"me", "refund"}, accountBillingListSize)
		if err != nil {
			state.Logger.Error("获取退款列表失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取退款列表失败: " + ovh.Explain(err)})
			return
		}
		if warn != "" {
			state.Logger.Warn("退款列表"+warn, "account_management")
		}
		scanIDs, truncated := newestBillingIDs(ids, accountBillingMaxDetails)
		if truncated {
			state.Logger.Warn(fmt.Sprintf("退款记录共 %d 条,只取最新 %d 条排序", len(ids), len(scanIDs)), "account_management")
		}
		// 并发拉详情：10 并发,原 20 * 200ms = 4 秒 -> 2 * 200ms = 0.4 秒
		details, failed, firstErr := parallelGetStringsCounted(client, scanIDs, func(s string) string {
			return safePathFn("me", "refund", s)
		}, 10)
		list := collectDetails(state, c, details, failed, firstErr, "退款", "account_management")
		if c.IsAborted() {
			return
		}
		sortBillingByDateDesc(list)
		if len(list) > accountBillingListSize {
			list = list[:accountBillingListSize]
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 条退款记录(窗口内 %d 条,拉取失败 %d 条)", len(list), len(scanIDs), failed), "account_management")
		c.JSON(http.StatusOK, list)
	}
}

// GetAccountBills GET /api/ovh/account/bills
func GetAccountBills(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		ids, warn, err := fetchRecentBillingIDs(client, []string{"me", "bill"}, accountBillingListSize)
		if err != nil {
			state.Logger.Error("获取账单列表失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取账单列表失败: " + ovh.Explain(err)})
			return
		}
		if warn != "" {
			state.Logger.Warn("账单列表"+warn, "account_management")
		}
		scanIDs, truncated := newestBillingIDs(ids, accountBillingMaxDetails)
		if truncated {
			state.Logger.Warn(fmt.Sprintf("账单共 %d 条,只取最新 %d 条排序", len(ids), len(scanIDs)), "account_management")
		}
		// 并发拉账单详情
		details, failed, firstErr := parallelGetStringsCounted(client, scanIDs, func(s string) string {
			return safePathFn("me", "bill", s)
		}, 10)
		list := collectDetails(state, c, details, failed, firstErr, "账单", "account_management")
		if c.IsAborted() {
			return
		}
		sortBillingByDateDesc(list)
		if len(list) > accountBillingListSize {
			list = list[:accountBillingListSize]
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 条账单记录(窗口内 %d 条,拉取失败 %d 条)", len(list), len(scanIDs), failed), "account_management")
		c.JSON(http.StatusOK, gin.H{
			"status":    "success",
			"data":      list,
			"total":     len(ids),
			"failed":    failed,
			"truncated": truncated,
			// warning 为空字符串表示走的是正常窗口查询;非空说明是降级路径,前端可原样提示
			"warning": warn,
		})
	}
}

// GetAccountOrders GET /api/ovh/account/orders
func GetAccountOrders(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		acc, ok := ovhAccountFor(state, c)
		if !ok {
			noOVHRespAccount(c)
			return
		}
		// 整次列表固定实际账户,不能在等待详情后重新读取默认账户。
		client, err := state.OVH.IsolatedClientFor(acc.ID)
		if err != nil {
			noOVHRespAccount(c)
			return
		}

		limit := 30
		if q := c.Query("limit"); q != "" {
			if n, err := strconv.Atoi(q); err == nil && n > 0 {
				limit = n
			}
		}
		if limit > accountBillingMaxDetails {
			limit = accountBillingMaxDetails
		}

		ids, warn, err := fetchRecentBillingIDs(client, []string{"me", "order"}, limit)
		if err != nil {
			state.Logger.Error("获取订单列表失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取订单列表失败: " + ovh.Explain(err)})
			return
		}
		if warn != "" {
			state.Logger.Warn("订单列表"+warn, "account_management")
		}

		scanIDs, truncated := newestBillingIDs(ids, limit)
		if truncated {
			state.Logger.Warn(fmt.Sprintf("订单共 %d 条,只取最新 %d 条拉取详情", len(ids), len(scanIDs)), "account_management")
		}

		// 并发拉订单详情
		details, failed, firstErr := parallelGetStringsCounted(client, scanIDs, func(s string) string {
			return safePathFn("me", "order", s)
		}, 10)
		list := collectDetails(state, c, details, failed, firstErr, "订单", "account_management")
		if c.IsAborted() {
			return
		}

		accountOrderStatuses(c.Request.Context(), state, acc.ID, list)
		// billing.Order.password 是订单口令,页面用不到,不透传。
		// url 缺失时用控制台深链补齐。官方三区 schema 与上游 v0.2.5 同一口径。
		for _, item := range list {
			item["accountId"] = acc.ID
			delete(item, "password")
			if u, _ := item["url"].(string); strings.TrimSpace(u) == "" {
				if orderIDStr := idToString(item["orderId"]); orderIDStr != "" && acc.Endpoint != "" {
					item["url"] = ovh.ManagerOrderURL(acc.Endpoint, orderIDStr)
				}
			}
		}

		sortBillingByDateDesc(list)
		if len(list) > limit {
			list = list[:limit]
		}

		state.Logger.Info(fmt.Sprintf("成功获取 %d 条订单记录(取样 %d 条,拉取失败 %d 条)", len(list), len(scanIDs), failed), "account_management")
		c.JSON(http.StatusOK, gin.H{
			"status":    "success",
			"data":      list,
			"orders":    list,
			"total":     len(ids),
			"failed":    failed,
			"truncated": truncated,
			"warning":   warn,
		})
	}
}
