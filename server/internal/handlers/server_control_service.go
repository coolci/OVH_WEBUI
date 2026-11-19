package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/ovh"
)

// GetServiceInfo GET /api/server-control/:service_name/serviceinfo
func GetServiceInfo(state *app.State) gin.HandlerFunc {
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
		svcPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "serviceInfos")
		if !ok {
			return
		}
		var info map[string]interface{}
		if err := client.Get(svcPath, &info); err != nil {
			state.Logger.Error("获取服务器 "+svc+" 服务信息失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		renew, _ := info["renew"].(map[string]interface{})
		automatic := false
		period := 0
		deleteAtExpiration := false
		forced := false
		manualPayment := false
		if renew != nil {
			if a, ok := renew["automatic"].(bool); ok {
				automatic = a
			}
			if p, ok := numconv.ToInt64(renew["period"]); ok {
				period = int(p)
			}
			if d, ok := renew["deleteAtExpiration"].(bool); ok {
				deleteAtExpiration = d
			}
			if f, ok := renew["forced"].(bool); ok {
				forced = f
			}
			if m, ok := renew["manualPayment"].(bool); ok {
				manualPayment = m
			}
		}
		// possibleRenewPeriod: OVH 给的合法续费周期(月数数组,前端 select 用)
		possiblePeriods := []int{}
		if arr, ok := info["possibleRenewPeriod"].([]interface{}); ok {
			for _, v := range arr {
				if p, ok := numconv.ToInt64(v); ok && p > 0 {
					possiblePeriods = append(possiblePeriods, int(p))
				}
			}
		}
		si := map[string]interface{}{
			"status":                    valueOr(info, "status", "unknown"),
			"expiration":                valueOr(info, "expiration", ""),
			"creation":                  valueOr(info, "creation", ""),
			"renewalType":               automatic, // 自动续费 yes/no
			"renewalPeriod":             period,    // 续费周期(月)
			"renewalDeleteAtExpiration": deleteAtExpiration,
			"renewalForced":             forced, // OVH 强制自动续费(不能改)
			"renewalManualPayment":      manualPayment,
			"possibleRenewPeriod":       possiblePeriods,
		}
		// 终止状态从 lifecycle.pendingActions 读(文档指定的读回路径);
		// renew.deleteAtExpiration 会不会随 terminationPolicy 同步,文档没说,不能赌它
		attachTerminationState(state, client, serviceIDForDedicated, svc, "server_control", si)
		c.JSON(http.StatusOK, gin.H{"success": true, "serviceInfo": si})
	}
}

// UpdateServiceRenewal PUT /api/server-control/:service_name/serviceinfo
//
// 修改服务的续费策略。前端传 mode (auto / manual / delete-at-expiration) + 可选 period,
// 后端先 GET 当前 serviceInfos 读出 renew 现状,改完只 PUT 回 renew 这一个字段 ——
// services.Service 里其余字段全是只读,一并发回去会被 OVH 400 掉。
// forced=true (engaged 合同期) 时 OVH 会拒,我们这里直接返 400 提示用户。
func UpdateServiceRenewal(state *app.State) gin.HandlerFunc {
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
		var body struct {
			Mode   string `json:"mode"`   // "auto" / "manual" / "delete"
			Period int    `json:"period"` // 月数,0 表示不改
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Period < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请填写有效的续费方式与周期", "code": "RENEWAL_INVALID_REQUEST"})
			return
		}

		svcPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "serviceInfos")
		if !ok {
			return
		}
		var info map[string]interface{}
		if err := client.Get(svcPath, &info); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		renew, _ := info["renew"].(map[string]interface{})
		if renew == nil {
			renew = map[string]interface{}{}
		}
		// forced 锁:OVH engaged 合同期内,renew 改不动 —— 提前拒绝,不让 PUT 浪费一次往返
		if f, ok := renew["forced"].(bool); ok && f {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "该服务器处于 OVH 合同期(engaged),续费策略由 OVH 锁定,无法修改", "code": "E8F1028D1",
			})
			return
		}
		// forced 是 OVH 自己算的锁,原样回传 —— 它是 canBeNull=false,少了会被拒
		forced := false
		if f, ok := renew["forced"].(bool); ok {
			forced = f
		}

		// 显式构造完整的 RenewType,不在 GET 回来的 map 上就地改。
		//
		// service.RenewType 里 automatic / deleteAtExpiration / forced 三个是
		// canBeNull=false —— 少给任何一个,OVH 回
		// 400 "[renew] Missing properties: (xxx) for type RenewType"。
		// 光看 readOnly 挑字段是不够的,还得看 canBeNull:前者决定"能不能发",
		// 后者决定"必不必须发"。(schema 里那个 required 字段全是 false,不可信。)
		next := map[string]interface{}{"forced": forced}
		switch body.Mode {
		case "auto":
			next["automatic"] = true
			next["deleteAtExpiration"] = false
			next["manualPayment"] = false
		case "manual":
			next["automatic"] = false
			next["deleteAtExpiration"] = false
			next["manualPayment"] = true
		case "delete":
			// 到期终止不能靠 renew 标志位(OVH 回 400 "Arguments conflicting",行为不可预测)。
			// 也**不能**指向 POST /terminate——那是立即终止,提交即暂停服务器,真实踩过。
			// 唯一正确的路是 PUT /services/{serviceId} 的 terminationPolicy。
			c.JSON(http.StatusBadRequest, gin.H{"success": false,
				"error": "到期终止请用 PUT termination-policy 接口(terminationPolicy=terminateAtExpirationDate);不要调 /terminate——那是立即终止,提交即暂停服务", "code": "ECBAA0559"})
			return
		default:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "mode 必须是 auto / manual / delete 之一", "code": "EA464F1AF"})
			return
		}
		// period 可空:用户指定了就用,否则沿用 OVH 当前值
		if body.Period > 0 {
			next["period"] = body.Period
		} else if pv, ok := renew["period"]; ok && pv != nil {
			next["period"] = pv
		}

		// GET 响应自带 serviceId:老路由被 US live 校验卡死时,
		// 使用 mode + ISO 8601 period 的新路由 PUT /services/{serviceId} 再尝试
		var sid *int64
		if v, ok := numconv.ToInt64(info["serviceId"]); ok {
			sid = &v
		}
		// services.Service 里只有 renew 可写,其余 12 个字段全是只读 ——
		// 把 GET 回来的整个对象发回去会被 OVH 400
		// "Try to alter read-only properties: status, creation, expiration"
		if err := putServiceRenew(client, svcPath, sid, next, body.Period > 0); err != nil {
			state.Logger.Error("修改服务器 "+svc+" 续费策略失败: "+err.Error(), "server_control")
			respondRenewError(c, state, svc, err)
			return
		}
		state.Logger.Info("服务器 "+svc+" 续费策略已更新: mode="+body.Mode, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "续费策略已更新", "code": "ED101FDC4"})
	}
}

// ChangeContact POST /api/server-control/:service_name/change-contact
//
// EU/CA only —— /dedicated/server/{serviceName}/changeContact 在 EU 与 CA 两站都存在(POST),
// 只有 US(api.us.ovhcloud.com)整个 dedicated/server 命名空间里没有这个端点
// (本地三区 dump + live schema 双重核实:EU/CA 各 107 条路径含 changeContact,US 98 条不含)。
// OVHcloud US 是独立公司,没有 NIC 联系人体系,配套的 /me/task/contactChange 系列在 US 同样缺失。
// 这里提前拒,免得用户看到 OVH 原始的 404。
//
// 判大区走 srvRegionFor(见 server_control_basic.go),不比 acc.Endpoint 字符串:
// endpoint 还有 kimsufi-* / soyoustart-* 别名,散装比较漏一种写法门控就失效了。
func ChangeContact(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		if region := srvRegionFor(state, c); region == srvRegionUS {
			state.Logger.Warn("账户所在大区 "+region+" 不提供服务器联系人变更(changeContact),已拦截请求", "server_control")
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "美区账户不支持通过 API 变更服务器联系人 —— OVHcloud US 没有 NIC 联系人系统,请在 OVHcloud US 控制台或联系客服办理", "code": "ED999C012",
				"unsupported": true,
				"region":      region,
			})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body map[string]interface{}
		_ = c.ShouldBindJSON(&body)
		params := map[string]interface{}{}
		if v, ok := body["contactAdmin"].(string); ok && v != "" {
			params["contactAdmin"] = v
		}
		if v, ok := body["contactTech"].(string); ok && v != "" {
			params["contactTech"] = v
		}
		if v, ok := body["contactBilling"].(string); ok && v != "" {
			params["contactBilling"] = v
		}
		if len(params) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "至少需要指定一个联系人（管理员、技术或计费）", "code": "E06AF4A77"})
			return
		}
		// OVH 这个接口返回 long[](任务 ID 数组),不是 map
		contactPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "changeContact")
		if !ok {
			return
		}
		var taskIDs []int64
		if err := client.Post(contactPath, params, &taskIDs); err != nil {
			state.Logger.Error("变更服务器 "+svc+" 联系人失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info(fmt.Sprintf("服务器 %s 联系人变更请求已提交: %v, tasks=%v", svc, params, taskIDs), "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "联系人变更请求已提交", "code": "EA0AA106E",
			"taskIds": taskIDs,
		})
	}
}
