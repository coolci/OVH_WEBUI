package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// netOVHStatusCode 取 OVH 返回的 HTTP 状态码；不是 OVH API 错误(DNS/超时等传输层错误)时返回 0。
// 之所以不用 strings.Contains 匹配错误文案：OVH 对「服务不属于本凭据」「服务名拼错」
// 也回 404 "This service does not exist"，靠子串匹配会把权限问题一并降级成「暂无数据」，
// 用户完全无从自查。判状态码至少能把 401/403/5xx 挡在降级之外。
func netOVHStatusCode(err error) int {
	var apiErr *ovhsdk.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return 0
}

// netOVHMessage 取 OVH 原始 message，用于在降级响应里保留可自查的线索。
func netOVHMessage(err error) string {
	var apiErr *ovhsdk.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Message
	}
	return err.Error()
}

// MRTG 的 period / type 在 schema 里是枚举(dedicated.server.MrtgPeriodEnum / MrtgTypeEnum)，
// 枚举外的值 OVH 直接 400。前端 query 是用户可控输入，先在本地兜住比赌 OVH 宽容度稳妥。
var mrtgPeriodEnum = map[string]bool{
	"daily": true, "hourly": true, "monthly": true, "weekly": true, "yearly": true,
}

var mrtgTypeEnum = map[string]bool{
	"errors:download": true, "errors:upload": true,
	"packets:download": true, "packets:upload": true,
	"traffic:download": true, "traffic:upload": true,
}

// normalizeMRTGQuery 解析并校验 period / type，非法值直接告诉用户合法取值，不透传给 OVH。
func normalizeMRTGQuery(c *gin.Context) (period string, trafficType string, err error) {
	period = c.DefaultQuery("period", "daily")
	trafficType = c.DefaultQuery("type", "traffic:download")
	if !mrtgPeriodEnum[period] {
		return "", "", fmt.Errorf("period 参数非法: %s (合法值: daily/hourly/weekly/monthly/yearly)", period)
	}
	if !mrtgTypeEnum[trafficType] {
		return "", "", fmt.Errorf("type 参数非法: %s (合法值: traffic:download/traffic:upload/packets:download/packets:upload/errors:download/errors:upload)", trafficType)
	}
	return period, trafficType, nil
}

// (旧版 legacyMRTGFallback 已删:它打的 /dedicated/server/{svc}/mrtg 三区 DEPRECATED,
//  按约定废弃端点不再调用 —— 新端点拿不到网卡时如实报错/报空,不静默兜底。)

// perNICMRTG 按 schema 推荐路径逐张网卡取流量图：/networkInterfaceController/{mac}/mrtg。
func perNICMRTG(client *ovhsdk.Client, svc string, macs []string, period, trafficType string) []gin.H {
	type mrtgResult struct {
		data []map[string]interface{}
		err  error
	}
	results := make([]mrtgResult, len(macs))
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	for i, mac := range macs {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, m string) {
			defer wg.Done()
			defer func() { <-sem }()
			var d []map[string]interface{}
			q := url.Values{}
			q.Set("period", period)
			q.Set("type", trafficType)
			path, pErr := buildOVHPathQuery(q, "dedicated", "server", svc, "networkInterfaceController", m, "mrtg")
			if pErr != nil {
				results[idx] = mrtgResult{err: pErr}
				return
			}
			client, err := ovh.IsolateClient(client)
			if err != nil {
				results[idx] = mrtgResult{err: err}
				return
			}
			if err := client.Get(path, &d); err != nil {
				results[idx] = mrtgResult{err: err}
				return
			}
			results[idx] = mrtgResult{data: d}
		}(i, mac)
	}
	wg.Wait()

	all := []gin.H{}
	for i, mac := range macs {
		r := results[i]
		if r.err != nil {
			all = append(all, gin.H{"mac": mac, "data": []interface{}{}, "error": r.err.Error()})
			continue
		}
		all = append(all, gin.H{"mac": mac, "data": r.data})
	}
	return all
}

// GetNetworkInterfaces GET /api/server-control/:service_name/network-interfaces
func GetNetworkInterfaces(state *app.State) gin.HandlerFunc {
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
		nicPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "networkInterfaceController")
		if !ok {
			return
		}
		state.Logger.Info("[网卡] 获取物理网卡列表: "+svc, "server_control")
		var macs []string
		if err := client.Get(nicPath, &macs); err != nil {
			// networkInterfaceController 是 BETA 端点，对没有网卡数据的机器会回 404，
			// 这类空资源降级成空列表是刻意的产品行为；401/403/5xx/传输层错误必须照实报出来，
			// 否则切错账户或服务名拼错时用户只会看到「暂无网卡信息」，无从自查。
			if netOVHStatusCode(err) == http.StatusNotFound {
				state.Logger.Warn("[网卡] "+svc+" 返回 404，按「暂无网卡信息」处理: "+err.Error(), "server_control")
				c.JSON(http.StatusOK, gin.H{
					"success":    true,
					"interfaces": []interface{}{},
					"count":      0,
					"message":    "该服务器暂无网卡信息", "code": "EF0DDE9F4",
					"detail": netOVHMessage(err),
				})
				return
			}
			state.Logger.Error("[网卡] 获取网卡列表失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		// 并发拉每张网卡详情。用带 error 的版本:失败原因要原样带给前端,
		// 编一句 "fetch failed" 会让用户分不清是权限问题还是这张卡真没数据。
		details, errs := parallelGetStringKeysWithErrs(client, macs, func(m string) string {
			return safePathFn("dedicated", "server", svc, "networkInterfaceController", m)
		}, 10)
		failed, firstErr := countErrs(errs)
		if failed > 0 {
			state.Logger.Warn(fmt.Sprintf("[网卡] %s 有 %d/%d 张网卡详情获取失败,首个错误: %v",
				svc, failed, len(macs), firstErr), "server_control")
		}
		interfaces := []gin.H{}
		for i, mac := range macs {
			d := details[i]
			if d == nil {
				// 字段名与前端 readPartialList / DetailErrorTag 的约定一致(_detailError)
				msg := "详情获取失败"
				if errs[i] != nil {
					msg = errs[i].Error()
				}
				interfaces = append(interfaces, gin.H{
					"mac":          mac,
					"linkType":     "unknown",
					"_detailError": msg,
				})
				continue
			}
			interfaces = append(interfaces, gin.H{
				"mac":                     mac,
				"linkType":                d["linkType"],
				"virtualNetworkInterface": d["virtualNetworkInterface"],
			})
		}
		state.Logger.Info(fmt.Sprintf("[网卡] 找到 %d 个物理网卡(%d 张详情缺失)", len(interfaces), failed), "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"interfaces":  interfaces,
			"count":       len(interfaces),
			"partial":     failed > 0,
			"failedCount": failed,
		})
	}
}

// GetMRTGData GET /api/server-control/:service_name/mrtg
func GetMRTGData(state *app.State) gin.HandlerFunc {
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
		period, trafficType, qErr := normalizeMRTGQuery(c)
		if qErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": ovh.Explain(qErr)})
			return
		}
		nicPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "networkInterfaceController")
		if !ok {
			return
		}
		state.Logger.Info(fmt.Sprintf("[MRTG] 获取流量数据: %s - %s - %s", svc, period, trafficType), "server_control")

		var macs []string
		listErr := client.Get(nicPath, &macs)
		// 旧端点 /dedicated/server/{svc}/mrtg 三区 DEPRECATED(deletion 2018 年就到期),
		// 按约定不再回落。网卡列表拿不到就如实报错 —— 空流量图配一条明确错误,
		// 比静默画一张空图或打一条废弃接口强。
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "网卡列表读取失败,无法取流量图: " + ovh.Explain(listErr),
			})
			return
		}
		if len(macs) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": true, "period": period, "type": trafficType,
				"interfaces": []gin.H{},
				"message":    "OVH 未返回任何网卡(可能是极老机型未接入 networkInterfaceController),因此没有流量图数据", "code": "E65DF8BAB",
			})
			return
		}

		all := perNICMRTG(client, svc, macs, period, trafficType)
		for _, item := range all {
			if e, ok := item["error"]; ok {
				state.Logger.Warn(fmt.Sprintf("[MRTG] 获取网卡 %v 数据失败: %v", item["mac"], e), "server_control")
				continue
			}
			if d, ok := item["data"].([]map[string]interface{}); ok {
				state.Logger.Info(fmt.Sprintf("[MRTG] 获取网卡 %v 数据成功: %d 个数据点", item["mac"], len(d)), "server_control")
			}
		}
		state.Logger.Info(fmt.Sprintf("[MRTG] 成功获取 %d 个网卡的流量数据", len(all)), "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"interfaces": all,
			"period":     period,
			"type":       trafficType,
			"server":     svc,
		})
	}
}

// ConfigureOLAAggregation POST /api/server-control/:service_name/ola/aggregation
func ConfigureOLAAggregation(state *app.State) gin.HandlerFunc {
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
			Name                     string   `json:"name"`
			VirtualNetworkInterfaces []string `json:"virtualNetworkInterfaces"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少聚合名称(name)参数", "code": "E5DFC8606"})
			return
		}
		if len(body.VirtualNetworkInterfaces) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "至少需要2个网络接口进行聚合", "code": "E0A44E5AC"})
			return
		}
		olaPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ola", "aggregation")
		if !ok {
			return
		}
		state.Logger.Info(fmt.Sprintf("[OLA] 配置网络聚合: %s - %s - %d个接口", svc, body.Name, len(body.VirtualNetworkInterfaces)), "server_control")
		var result map[string]interface{}
		if err := client.Post(olaPath, map[string]interface{}{
			"name":                     body.Name,
			"virtualNetworkInterfaces": body.VirtualNetworkInterfaces,
		}, &result); err != nil {
			respondOVHError(c, err)
			return
		}
		state.Logger.Info(fmt.Sprintf("[OLA] 网络聚合配置任务已创建: Task#%v", result["taskId"]), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "网络聚合配置任务已创建", "code": "E5FFCBB25", "task": result})
	}
}

// ResetOLAConfiguration POST /api/server-control/:service_name/ola/reset
func ResetOLAConfiguration(state *app.State) gin.HandlerFunc {
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
			VirtualNetworkInterface string `json:"virtualNetworkInterface"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.VirtualNetworkInterface == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少虚拟网络接口UUID(virtualNetworkInterface)参数", "code": "EAD1397D5"})
			return
		}
		resetPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ola", "reset")
		if !ok {
			return
		}
		state.Logger.Info("[OLA] 重置网络接口: "+svc+" - "+body.VirtualNetworkInterface, "server_control")
		var result map[string]interface{}
		if err := client.Post(resetPath, map[string]interface{}{
			"virtualNetworkInterface": body.VirtualNetworkInterface,
		}, &result); err != nil {
			respondOVHError(c, err)
			return
		}
		state.Logger.Info(fmt.Sprintf("[OLA] 网络接口重置任务已创建: Task#%v", result["taskId"]), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "网络接口重置任务已创建", "code": "E44BE3AA0", "task": result})
	}
}

// OLAGroup POST /api/server-control/:service_name/ola/group
//
// 路由名保留(前端/外部调用方可能按老名字调)，但实际打的是 OVH 官方 replacement:
// schema 里 /dedicated/server/{svc}/ola/group 已 DEPRECATED → /dedicated/server/{svc}/ola/aggregation，
// 两者 body 签名完全一致(name + virtualNetworkInterfaces)，所以直接走新端点不会有行为差异，
// 还能避免 OVH 下线旧端点后这条路由突然 404。
func OLAGroup(state *app.State) gin.HandlerFunc {
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
		// schema: name(string) 与 virtualNetworkInterfaces(uuid[]) 都是 body 必填。
		// 老实现发的是空 {}，必被 OVH 400。
		var body struct {
			Name                     string   `json:"name"`
			VirtualNetworkInterfaces []string `json:"virtualNetworkInterfaces"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少聚合名称(name)参数", "code": "E5DFC8606"})
			return
		}
		if len(body.VirtualNetworkInterfaces) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "至少需要2个网络接口进行聚合", "code": "E0A44E5AC"})
			return
		}
		olaPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ola", "aggregation")
		if !ok {
			return
		}
		state.Logger.Info(fmt.Sprintf("[OLA] 创建OLA组(走 ola/aggregation): %s - %s - %d个接口", svc, body.Name, len(body.VirtualNetworkInterfaces)), "server_control")
		var result map[string]interface{}
		if err := client.Post(olaPath, map[string]interface{}{
			"name":                     body.Name,
			"virtualNetworkInterfaces": body.VirtualNetworkInterfaces,
		}, &result); err != nil {
			respondOVHError(c, err)
			return
		}
		state.Logger.Info(fmt.Sprintf("创建OLA组成功: %s, Task#%v", svc, result["taskId"]), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "OLA组已创建", "code": "E72EBD409", "result": result, "task": result})
	}
}

// OLAUngroup POST /api/server-control/:service_name/ola/ungroup
//
// 同 OLAGroup：schema 里 /ola/ungroup 已 DEPRECATED → /ola/reset，body 签名一致
// (virtualNetworkInterface uuid)，故实际打新端点。
// 差异只有返回值：旧 /ola/ungroup 是 Task[]，新 /ola/reset 是单个 Task；
// 为了不破坏可能已按数组解析的调用方，tasks 仍以数组形式返回。
func OLAUngroup(state *app.State) gin.HandlerFunc {
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
		// schema: virtualNetworkInterface(uuid) 是 body 必填。老实现发空 {}，必被 OVH 400。
		var body struct {
			VirtualNetworkInterface string `json:"virtualNetworkInterface"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.VirtualNetworkInterface == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少虚拟网络接口UUID(virtualNetworkInterface)参数", "code": "EAD1397D5"})
			return
		}
		resetPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ola", "reset")
		if !ok {
			return
		}
		state.Logger.Info("[OLA] 解散OLA组(走 ola/reset): "+svc+" - "+body.VirtualNetworkInterface, "server_control")
		var task map[string]interface{}
		if err := client.Post(resetPath, map[string]interface{}{
			"virtualNetworkInterface": body.VirtualNetworkInterface,
		}, &task); err != nil {
			respondOVHError(c, err)
			return
		}
		tasks := []map[string]interface{}{task}
		state.Logger.Info(fmt.Sprintf("解散OLA组成功: %s, Task#%v", svc, task["taskId"]), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "OLA组已解散", "code": "EF6B3CAF6", "tasks": tasks, "task": task})
	}
}

// GetTrafficStatistics GET /api/server-control/:service_name/statistics
func GetTrafficStatistics(state *app.State) gin.HandlerFunc {
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
		// 老实现的默认 period 是 "lastday"，不在 dedicated.server.MrtgPeriodEnum 里；
		// 这里靠 normalizeMRTGQuery 统一收敛到合法枚举（默认 daily）。
		period, typeParam, qErr := normalizeMRTGQuery(c)
		if qErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": ovh.Explain(qErr)})
			return
		}
		nicPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "networkInterfaceController")
		if !ok {
			return
		}
		state.Logger.Info(fmt.Sprintf("[Stats] 获取服务器 %s 流量统计: %s - %s", svc, period, typeParam), "server_control")

		var macs []string
		listErr := client.Get(nicPath, &macs)
		// 同 GetMRTGData:不再回落废弃的 /mrtg,拿不到网卡就如实说
		if listErr != nil {
			state.Logger.Error("[Stats] 网卡列表读取失败: "+listErr.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "网卡列表读取失败,无法取流量统计: " + ovh.Explain(listErr),
			})
			return
		}
		if len(macs) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": true, "period": period, "type": typeParam,
				"statistics": []gin.H{},
				"message":    "OVH 未返回任何网卡(可能是极老机型未接入 networkInterfaceController),因此没有流量统计数据", "code": "EFB67AACB",
			})
			return
		}

		stats := perNICMRTG(client, svc, macs, period, typeParam)
		state.Logger.Info(fmt.Sprintf("[Stats] 流量统计获取成功: %d 个网卡", len(stats)), "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"statistics": stats,
			"period":     period,
			"type":       typeParam,
		})
	}
}

// GetNetworkInterfaceStats GET /api/server-control/:service_name/network-stats
func GetNetworkInterfaceStats(state *app.State) gin.HandlerFunc {
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
		nicPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "networkInterfaceController")
		if !ok {
			return
		}
		state.Logger.Info("[Network] 获取服务器 "+svc+" 网络接口信息", "server_control")
		var macs []string
		if err := client.Get(nicPath, &macs); err != nil {
			respondOVHError(c, err)
			return
		}
		// 并发拉每张网卡详情
		details := parallelGetStringKeys(client, macs, func(m string) string {
			return safePathFn("dedicated", "server", svc, "networkInterfaceController", m)
		}, 10)
		// 失败的网卡不能直接丢：静默少几张卡会让用户以为服务器就这么多网卡，
		// 进而影响 OLA 聚合时的接口选择。与 GetNetworkInterfaces 保持一致的做法：按索引补位。
		interfaces := []map[string]interface{}{}
		failed := 0
		for i, mac := range macs {
			d := details[i]
			if d == nil {
				failed++
				interfaces = append(interfaces, map[string]interface{}{
					"mac":   mac,
					"error": "fetch failed",
				})
				continue
			}
			interfaces = append(interfaces, d)
		}
		if failed > 0 {
			state.Logger.Warn(fmt.Sprintf("[Network] %d/%d 个网卡详情获取失败", failed, len(macs)), "server_control")
		}
		state.Logger.Info(fmt.Sprintf("[Network] 找到 %d 个网络接口", len(interfaces)), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "interfaces": interfaces})
	}
}
