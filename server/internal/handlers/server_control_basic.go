package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// noOVHResp 当前账户取不到 OVH 客户端时的统一响应。
//
// 用 412 Precondition Failed 而不是 401:401 的语义是"你的身份凭据不对",
// 前端的 401 拦截器会据此判定会话失效、把用户踢回登录界面。
// 而这里的真实含义是"你还没添加 OVH 账户(或它的凭据不全)"—— 用户访问后端的
// API 密钥完全正确。混用 401 会让没配账户的用户被反复踢出登录页,
// 而他怎么重新输密钥都没用,因为问题根本不在密钥上。
func noOVHResp(c *gin.Context) {
	if statusVal, exists := c.Get(ctxOVHErrStatusKey); exists {
		status, _ := statusVal.(int)
		code := c.GetString(ctxOVHErrCodeKey)
		msg := c.GetString(ctxOVHErrMsgKey)
		if status >= 400 && msg != "" {
			c.JSON(status, gin.H{
				"success": false,
				"error":   msg,
				"code":    code,
			})
			return
		}
	}
	c.JSON(http.StatusPreconditionFailed, gin.H{
		"success": false,
		"error":   "未配置 OVH 账户或凭据不全，请到「设置 → OVH 账户」添加",
		"code":    "NO_OVH_ACCOUNT",
	})
}

// ── 区域门控 ──────────────────────────────────────────────────────────────
//
// EU / US / CA 是三套彼此独立的 OVH 系统，/dedicated/server 命名空间的路径数就不一样
// （live schema 实测：EU 107 条、CA 107 条、US 98 条）。US 少的正好是这 9 条：
//
//	/dedicated/server/log                       /dedicated/server/virtualNetworkInterface/{uuid}
//	/dedicated/server/{sn}/changeContact        /dedicated/server/{sn}/license/windows
//	/dedicated/server/{sn}/features/backupFTP  (+ access / access/{ipBlock} / authorizableBlocks / password)
//
// 其中本域真正会调用的只有 changeContact 和 backupFTP 全家桶，两处都必须提前拦下来，
// 否则 go-ovh 会拿一个本站点不存在的路径去请求，用户看到的是一句 "Got an invalid (or empty) URL"
// 或 OVH 的英文 404，完全看不出"这是美区没有的功能"。
// EU 与 CA 在 /dedicated/server、/ip、/services、/dedicated/installationTemplate 四个命名空间上
// 逐条对比过（路径 + 方法 + 参数 + 返回类型 + 废弃标记），零差异，所以本域不需要 CA 专属门控。
const srvRegionUS = "US"

// srvRegionFor 当前请求所用账户的 OVH 大区(EU / US / CA)。
//
// 一律走 ovh.EndpointRegion，不要在各 handler 里散写 `acc.Endpoint == "ovh-us"`：
// endpoint 是账户表里的自由字符串，除了 ovh-* 还有 kimsufi-* / soyoustart-* 品牌别名，
// 散装字符串比较早晚会漏掉一种写法，而漏掉的后果是美区用户点到一个必然失败的按钮。
// 账户查不到时 acc 是零值，EndpointRegion("") 回落 EU —— 这条路径上紧接着的 ovhClientFor
// 一定会失败并走 noOVHResp，所以不存在"用错误的大区放行"的情况。
func srvRegionFor(state *app.State, c *gin.Context) string {
	acc, _ := ovhAccountFor(state, c)
	return ovh.EndpointRegion(acc.Endpoint)
}

// ListMyServers GET /api/server-control/list
func ListMyServers(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		acc, _ := ovhAccountFor(state, c)
		cacheScope := accountCacheFingerprint(acc)
		var names []string
		if err := client.Get("/dedicated/server", &names); err != nil {
			state.Logger.Error("获取服务器列表失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info(fmt.Sprintf("获取服务器列表成功，共 %d 台", len(names)), "server_control")

		// 并发拉 detail + serviceInfos：N 台服务器 × 2 GET × 串行 ~200ms => 改 10 并发 ≈ N/5 * 200ms
		type srvResult struct {
			info         map[string]interface{}
			svcInfo      map[string]interface{}
			lottery      *hwLottery
			detailError  error
			svcInfoError error
		}
		results := make([]srvResult, len(names))
		sem := make(chan struct{}, 10)
		var wg sync.WaitGroup
		for i, name := range names {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, nm string) {
				defer wg.Done()
				defer func() { <-sem }()
				var info map[string]interface{}
				client, err := ovh.IsolateClient(client)
				if err != nil {
					results[idx].detailError = err
					return
				}
				infoPath, pErr := buildOVHPath("dedicated", "server", nm)
				if pErr != nil {
					results[idx].detailError = pErr
					return
				}
				if err := client.Get(infoPath, &info); err != nil {
					results[idx].detailError = err
					return
				}
				results[idx].info = info
				var svc map[string]interface{}
				// serviceInfos 的错误不能吞：吞掉之后"查不到"会被渲染成"没开自动续费"，
				// 用户以为不用管，机器到期就被回收了
				svcPath, sErr := buildOVHPath("dedicated", "server", nm, "serviceInfos")
				if sErr != nil {
					results[idx].svcInfoError = sErr
					return
				}
				if err := client.Get(svcPath, &svc); err != nil {
					results[idx].svcInfoError = err
					return
				}
				results[idx].svcInfo = svc
				if expiredOwnedServer(info, svc, nil, nil) {
					return
				}
				// 中奖摘要:详情页算过就纯缓存读(0 请求),没算过补 1 次 hardware GET,
				// 1h TTL。徽章是锦上添花,失败只当没有,不影响列表
				results[idx].lottery = hwLotteryFor(client, cacheScope, nm)
			}(i, name)
		}
		wg.Wait()

		servers := []gin.H{}
		for i, name := range names {
			r := results[i]
			if expiredOwnedServer(r.info, r.svcInfo, r.detailError, r.svcInfoError) {
				state.Logger.Debug("隐藏已过期服务器 "+name, "server_control")
				continue
			}
			if r.detailError != nil {
				state.Logger.Error("获取服务器 "+name+" 详情失败: "+r.detailError.Error(), "server_control")
				servers = append(servers, gin.H{"serviceName": name, "name": name, "error": r.detailError.Error()})
				continue
			}
			info, svcInfo := r.info, r.svcInfo
			// 用 *bool：schema 里 renew 可空，加上查询本身也可能失败，
			// 这两种"不知道"必须和"确实关闭了自动续费"区分开
			var renewalType *bool
			if r.svcInfoError != nil {
				state.Logger.Error("获取服务器 "+name+" 续费信息失败: "+r.svcInfoError.Error(), "server_control")
			} else if svcInfo != nil {
				if rn, ok := svcInfo["renew"].(map[string]interface{}); ok {
					if a, ok := rn["automatic"].(bool); ok {
						renewalType = &a
					}
				}
				if renewalType == nil {
					// renew 为 null 时退回顶层 renewalType(service.RenewalTypeEnum)，
					// 枚举里 automatic* 系列都表示自动续费
					if rt, ok := svcInfo["renewalType"].(string); ok && rt != "" {
						auto := strings.HasPrefix(rt, "automatic")
						renewalType = &auto
					}
				}
			}
			// 缺失字段补默认值
			monitoring := info["monitoring"]
			if monitoring == nil {
				monitoring = false
			}
			professionalUse := info["professionalUse"]
			if professionalUse == nil {
				professionalUse = false
			}
			entry := gin.H{
				"serviceName":     name,
				"name":            valueOr(info, "name", name),
				"commercialRange": valueOr(info, "commercialRange", "N/A"),
				"datacenter":      valueOr(info, "datacenter", "N/A"),
				"state":           valueOr(info, "state", "unknown"),
				"monitoring":      monitoring,
				"reverse":         valueOr(info, "reverse", ""),
				"ip":              valueOr(info, "ip", "N/A"),
				"os":              valueOr(info, "os", "N/A"),
				"bootId":          info["bootId"],
				"professionalUse": professionalUse,
				"status":          valueOr(svcInfo, "status", "unknown"),
				"renewalType":     renewalType,
			}
			if r.svcInfoError != nil {
				entry["svcInfoError"] = r.svcInfoError.Error()
			}
			// 中奖徽章摘要:只带 won/tier,详情在机器详情页
			if r.lottery != nil && r.lottery.Checked && r.lottery.Won {
				entry["lottery"] = gin.H{"won": true, "tier": r.lottery.Tier}
			}
			servers = append(servers, entry)
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "servers": servers, "total": len(servers)})
	}
}

// Only explicit expiry evidence hides a server. A past billing date, a timeout,
// or an authentication failure must not make an otherwise usable server vanish.
func expiredOwnedServer(info, serviceInfo map[string]interface{}, detailErr, serviceErr error) bool {
	if ovh.IsServiceExpired(detailErr) || ovh.IsServiceExpired(serviceErr) {
		return true
	}
	for _, value := range []interface{}{info["state"], serviceInfo["status"]} {
		if status, ok := value.(string); ok && strings.EqualFold(strings.TrimSpace(status), "expired") {
			return true
		}
	}
	return false
}

func valueOr(m map[string]interface{}, key, fallback string) interface{} {
	if v, ok := m[key]; ok && v != nil {
		return v
	}
	return fallback
}

// Reboot POST /api/server-control/:service_name/reboot
func Reboot(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		rebootPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "reboot")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(rebootPath, map[string]interface{}{}, &result); err != nil {
			// 重启防抖(OVH 回 403 + "A reboot has already been requested")不是失败:
			// 用户要的重启已经在进行,当成功返回,别让人误以为权限出了问题
			if ovh.IsRebootAlreadyRequested(err) {
				state.Logger.Info("服务器 "+svc+" 已有重启在进行中,本次请求按幂等成功处理", "server_control")
				c.JSON(http.StatusOK, gin.H{
					"success": true,
					"message": "这台服务器已经有一个重启在进行中,无需重复发送 —— 稍等它完成即可", "code": "E704347D1",
				})
				return
			}
			state.Logger.Error("重启服务器 "+svc+" 失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("服务器 "+svc+" 重启请求已发送", "server_control")
		state.Logger.Audit("reboot_server", c.ClientIP(), svc, fmt.Sprintf("taskId=%v", result["taskId"]))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "服务器 " + svc + " 重启请求已发送",
			"taskId":  result["taskId"],
		})
	}
}
