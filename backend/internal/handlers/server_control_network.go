package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

// GetNetworkInterfaces GET /api/server-control/:service_name/network-interfaces
func GetNetworkInterfaces(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		state.Logger.Info("[网卡] 获取物理网卡列表: "+svc, "server_control")
		var macs []string
		if err := client.Get("/dedicated/server/"+svc+"/networkInterfaceController", &macs); err != nil {
			errMsg := strings.ToLower(err.Error())
			if strings.Contains(errMsg, "does not exist") || strings.Contains(errMsg, "not found") {
				c.JSON(http.StatusOK, gin.H{
					"success":    true,
					"interfaces": []interface{}{},
					"count":      0,
					"message":    "该服务器暂无网卡信息",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 并发拉每张网卡详情
		details := parallelGetStringKeys(client, macs, func(m string) string {
			return "/dedicated/server/" + svc + "/networkInterfaceController/" + m
		}, 10)
		interfaces := []gin.H{}
		for i, mac := range macs {
			d := details[i]
			if d == nil {
				interfaces = append(interfaces, gin.H{
					"mac":      mac,
					"linkType": "unknown",
					"error":    "fetch failed",
				})
				continue
			}
			interfaces = append(interfaces, gin.H{
				"mac":                     mac,
				"linkType":                d["linkType"],
				"virtualNetworkInterface": d["virtualNetworkInterface"],
			})
		}
		state.Logger.Info(fmt.Sprintf("[网卡] 找到 %d 个物理网卡", len(interfaces)), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "interfaces": interfaces, "count": len(interfaces)})
	}
}

// GetMRTGData GET /api/server-control/:service_name/mrtg
func GetMRTGData(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		period := c.DefaultQuery("period", "daily")
		trafficType := c.DefaultQuery("type", "traffic:download")
		state.Logger.Info(fmt.Sprintf("[MRTG] 获取流量数据: %s - %s - %s", svc, period, trafficType), "server_control")

		var macs []string
		if err := client.Get("/dedicated/server/"+svc+"/networkInterfaceController", &macs); err != nil {
			state.Logger.Warn("[MRTG] 无法获取网卡列表，使用旧版API: "+err.Error(), "server_control")
			var data []map[string]interface{}
			q := url.Values{}
			q.Set("period", period)
			q.Set("type", trafficType)
			if err := client.Get("/dedicated/server/"+svc+"/mrtg?"+q.Encode(), &data); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "新旧API均失败: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"success":    true,
				"data":       data,
				"period":     period,
				"type":       trafficType,
				"interfaces": []interface{}{},
			})
			return
		}
		// 并发拉每张网卡 MRTG 数据
		type mrtgResult struct {
			data []map[string]interface{}
			err  error
		}
		mrtgResults := make([]mrtgResult, len(macs))
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
				path := "/dedicated/server/" + svc + "/networkInterfaceController/" + m + "/mrtg?" + q.Encode()
				if err := client.Get(path, &d); err != nil {
					mrtgResults[idx] = mrtgResult{err: err}
					return
				}
				mrtgResults[idx] = mrtgResult{data: d}
			}(i, mac)
		}
		wg.Wait()

		all := []gin.H{}
		for i, mac := range macs {
			r := mrtgResults[i]
			if r.err != nil {
				all = append(all, gin.H{"mac": mac, "data": []interface{}{}, "error": r.err.Error()})
				continue
			}
			all = append(all, gin.H{"mac": mac, "data": r.data})
			state.Logger.Info(fmt.Sprintf("[MRTG] 获取网卡 %s 数据成功: %d 个数据点", mac, len(r.data)), "server_control")
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
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Name                    string   `json:"name"`
			VirtualNetworkInterfaces []string `json:"virtualNetworkInterfaces"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少聚合名称(name)参数"})
			return
		}
		if len(body.VirtualNetworkInterfaces) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "至少需要2个网络接口进行聚合"})
			return
		}
		state.Logger.Info(fmt.Sprintf("[OLA] 配置网络聚合: %s - %s - %d个接口", svc, body.Name, len(body.VirtualNetworkInterfaces)), "server_control")
		var result map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/ola/aggregation", map[string]interface{}{
			"name":                     body.Name,
			"virtualNetworkInterfaces": body.VirtualNetworkInterfaces,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("[OLA] 网络聚合配置任务已创建: Task#%v", result["taskId"]), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "网络聚合配置任务已创建", "task": result})
	}
}

// ResetOLAConfiguration POST /api/server-control/:service_name/ola/reset
func ResetOLAConfiguration(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
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
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少虚拟网络接口UUID(virtualNetworkInterface)参数"})
			return
		}
		state.Logger.Info("[OLA] 重置网络接口: "+svc+" - "+body.VirtualNetworkInterface, "server_control")
		var result map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/ola/reset", map[string]interface{}{
			"virtualNetworkInterface": body.VirtualNetworkInterface,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("[OLA] 网络接口重置任务已创建: Task#%v", result["taskId"]), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "网络接口重置任务已创建", "task": result})
	}
}

// OLAGroup POST /api/server-control/:service_name/ola/group
func OLAGroup(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var result map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/ola/group", map[string]interface{}{}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("创建OLA组成功: "+svc, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "OLA组已创建", "result": result})
	}
}

// OLAUngroup POST /api/server-control/:service_name/ola/ungroup
func OLAUngroup(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		// OVH /ola/ungroup 返回 Task[](数组),不是单个 Task 对象 —— 跟 group / aggregation 不同!
		var tasks []map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/ola/ungroup", map[string]interface{}{}, &tasks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("解散OLA组成功: %s, %d 个 task", svc, len(tasks)), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "OLA组已解散", "tasks": tasks})
	}
}

// featureEnabled 判断 OVH supportedFeatures 中某项是否为 true（不能只看 key 存在）。
func featureEnabled(sf map[string]interface{}, key string) bool {
	v, ok := sf[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	default:
		return false
	}
}

// pickIPMIAccessType 按可用性优先级选择：HTML5 > JNLP > Serial。
func pickIPMIAccessType(ipmi map[string]interface{}) string {
	sf, _ := ipmi["supportedFeatures"].(map[string]interface{})
	if sf == nil {
		return ""
	}
	for _, k := range []string{"kvmipHtml5URL", "kvmipJnlp", "serialOverLanURL"} {
		if featureEnabled(sf, k) {
			return k
		}
	}
	return ""
}

// consoleAccessValue 从 OVH access 响应提取可打开内容（url 或 value）。
func consoleAccessValue(console map[string]interface{}) string {
	if console == nil {
		return ""
	}
	for _, k := range []string{"value", "url", "consoleUrl", "link"} {
		if s, ok := console[k].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// GetIPMIConsole IPMI / KVM 控制台
func GetIPMIConsole(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		state.Logger.Info("[IPMI] 获取服务器 "+svc+" IPMI 信息", "server_control")
		var ipmi map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/features/ipmi", &ipmi); err != nil {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "404") || strings.Contains(lower, "does not exist") || strings.Contains(lower, "not found") {
				c.JSON(http.StatusOK, gin.H{
					"success":      false,
					"notAvailable": true,
					"error":        "此服务器未开通 IPMI / KVM 功能",
				})
				return
			}
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": err.Error()})
			return
		}
		if activated, ok := ipmi["activated"].(bool); ok && !activated {
			c.JSON(http.StatusOK, gin.H{
				"success":      false,
				"notAvailable": true,
				"error":        "IPMI 未激活，请先在 OVH Manager 启用 IPMI",
				"ipmi":         ipmi,
			})
			return
		}

		accessType := pickIPMIAccessType(ipmi)
		if accessType == "" {
			c.JSON(http.StatusOK, gin.H{
				"success":      false,
				"notAvailable": true,
				"error":        "服务器未声明可用的 KVM/Serial 访问类型",
				"ipmi":         ipmi,
			})
			return
		}
		// 允许前端强制类型（须在 supportedFeatures 中为 true）
		if q := c.Query("type"); q != "" {
			if sf, _ := ipmi["supportedFeatures"].(map[string]interface{}); featureEnabled(sf, q) {
				accessType = q
			}
		}
		state.Logger.Info("[IPMI] 请求控制台访问类型: "+accessType, "server_control")

		clientIP := c.GetHeader("X-Forwarded-For")
		if clientIP == "" {
			clientIP = c.ClientIP()
		}
		if idx := strings.Index(clientIP, ","); idx != -1 {
			clientIP = strings.TrimSpace(clientIP[:idx])
		}
		params := map[string]interface{}{
			"type": accessType,
			"ttl":  15,
		}
		// 公网客户端 IP 写入白名单；本地开发跳过（OVH 不接受私网）
		if clientIP != "" &&
			!strings.HasPrefix(clientIP, "127.") &&
			!strings.HasPrefix(clientIP, "192.168.") &&
			!strings.HasPrefix(clientIP, "10.") &&
			!strings.HasPrefix(clientIP, "172.16.") &&
			clientIP != "::1" {
			params["ipToAllow"] = clientIP
			state.Logger.Info("[IPMI] IP 白名单: "+clientIP, "server_control")
		} else {
			state.Logger.Warn("[IPMI] 跳过 IP 白名单（本地/内网）: "+clientIP, "server_control")
		}

		var task map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/features/ipmi/access", params, &task); err != nil {
			// 已有 session 时 OVH 常返回冲突，直接尝试读取现有 access
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "already") || strings.Contains(lower, "exist") || strings.Contains(lower, "409") {
				state.Logger.Warn("[IPMI] 创建 access 冲突，尝试读取现有会话: "+err.Error(), "server_control")
			} else {
				c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "创建 IPMI 访问任务失败: " + err.Error()})
				return
			}
		}

		taskID := task["taskId"]
		if taskID != nil {
			state.Logger.Info(fmt.Sprintf("[IPMI] 创建访问任务 taskId=%v status=%v", taskID, task["status"]), "server_control")
			// 最长约 60s（30×2s）；BHS 等机房常超过 20s
			maxRetries := 30
			taskCompleted := false
			for i := 0; i < maxRetries; i++ {
				time.Sleep(2 * time.Second)
				var ts map[string]interface{}
				if err := client.Get(fmt.Sprintf("/dedicated/server/%s/task/%v", svc, taskID), &ts); err != nil {
					state.Logger.Error(fmt.Sprintf("[IPMI] 查询任务 %v 失败: %s", taskID, err.Error()), "server_control")
					c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "查询 IPMI 任务失败: " + err.Error()})
					return
				}
				status, _ := ts["status"].(string)
				state.Logger.Info(fmt.Sprintf("[IPMI] 任务状态 (%d/%d): %s", i+1, maxRetries, status), "server_control")
				if status == "done" {
					taskCompleted = true
					break
				}
				if status == "cancelled" || status == "customerError" || status == "ovhError" {
					c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "IPMI 访问任务失败: " + status})
					return
				}
			}
			if !taskCompleted {
				c.JSON(http.StatusGatewayTimeout, gin.H{
					"success": false,
					"error":   "IPMI 访问任务超时（约 60s），请稍后重试",
				})
				return
			}
		}

		// 任务完成后 access 内容可能延迟出现，短轮询取值
		var consoleAccess map[string]interface{}
		var lastErr error
		for i := 0; i < 8; i++ {
			consoleAccess = nil
			if err := client.Get("/dedicated/server/"+svc+"/features/ipmi/access?type="+url.QueryEscape(accessType), &consoleAccess); err != nil {
				lastErr = err
				state.Logger.Warn(fmt.Sprintf("[IPMI] 读取 access 失败 (%d/8): %s", i+1, err.Error()), "server_control")
				time.Sleep(1500 * time.Millisecond)
				continue
			}
			if consoleAccessValue(consoleAccess) != "" {
				lastErr = nil
				break
			}
			state.Logger.Info(fmt.Sprintf("[IPMI] access 尚无 value/url (%d/8)，继续等待", i+1), "server_control")
			time.Sleep(1500 * time.Millisecond)
		}
		val := consoleAccessValue(consoleAccess)
		if val == "" {
			msg := "IPMI 会话已创建，但未返回控制台 URL/内容"
			if lastErr != nil {
				msg = msg + ": " + lastErr.Error()
			}
			c.JSON(http.StatusBadGateway, gin.H{
				"success":    false,
				"error":      msg,
				"accessType": accessType,
				"console":    consoleAccess,
				"ipmi":       ipmi,
			})
			return
		}
		// 归一化：前端统一读 console.value
		if consoleAccess == nil {
			consoleAccess = map[string]interface{}{}
		}
		if _, has := consoleAccess["value"]; !has || consoleAccess["value"] == "" {
			consoleAccess["value"] = val
		}

		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"ipmi":       ipmi,
			"console":    consoleAccess,
			"accessType": accessType,
			"value":      val,
		})
	}
}

// mapStatsPeriod 把前端 period 别名映射到 OVH /statistics 枚举。
// 前端：hourly|daily|weekly|monthly|yearly
// OVH 常见：lasthour|lastday|lastweek|lastmonth|lastyear（亦兼容 lastday 等旧写法）
func mapStatsPeriod(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "hourly", "hour", "lasthour", "last_hour":
		return "lasthour"
	case "weekly", "week", "lastweek", "last_week":
		return "lastweek"
	case "monthly", "month", "lastmonth", "last_month":
		return "lastmonth"
	case "yearly", "year", "lastyear", "last_year":
		return "lastyear"
	case "daily", "day", "lastday", "last_day", "":
		return "lastday"
	default:
		return raw
	}
}

func isOVHNotAvailable(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	keys := []string{
		"404", "not found", "does not exist", "not exist",
		"not supported", "not available", "unknown api",
		"this service is not compatible", "not compatible",
	}
	for _, k := range keys {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

// GetTrafficStatistics GET /api/server-control/:service_name/statistics
// 使用签名 OVH client；机型不支持时返回 200 + notAvailable（不报 500）。
// 说明：多数独服无主机级 CPU/内存 agent，OVH 的 /statistics 常不可用；
// 网络带宽请用 MRTG（/mrtg）——几乎所有独服都支持。
func GetTrafficStatistics(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		period := mapStatsPeriod(c.DefaultQuery("period", "daily"))
		typeParam := c.DefaultQuery("type", "traffic:download")
		path := fmt.Sprintf(
			"/dedicated/server/%s/statistics?period=%s&type=%s",
			url.PathEscape(svc),
			url.QueryEscape(period),
			url.QueryEscape(typeParam),
		)
		state.Logger.Info("[Stats] "+svc+" period="+period+" type="+typeParam, "server_control")

		var raw interface{}
		if err := client.Get(path, &raw); err != nil {
			if isOVHNotAvailable(err) {
				state.Logger.Info("[Stats] 机型不支持 /statistics: "+err.Error(), "server_control")
				c.JSON(http.StatusOK, gin.H{
					"success":      false,
					"notAvailable": true,
					"error":        "此服务器未开通主机级 /statistics 指标",
					"hint":         "网络流量请使用 MRTG（本页默认数据源），主机 CPU/内存需 OVH 监控 agent 支持",
					"period":       period,
					"type":         typeParam,
				})
				return
			}
			state.Logger.Error("[Stats] 调用失败: "+err.Error(), "server_control")
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"error":   "流量统计上游调用失败",
				"details": err.Error(),
			})
			return
		}

		// 归一化为前端可画图结构：优先识别时序点；否则原样放入 raw
		statistics := normalizeHostStatistics(raw, typeParam)
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"statistics": statistics,
			"period":     period,
			"type":       typeParam,
		})
	}
}

// normalizeHostStatistics 把 OVH 原始 statistics 尽量转成 { net_rx|net_tx|cpu|mem: [{timestamp,value}] }
func normalizeHostStatistics(raw interface{}, typeParam string) map[string]interface{} {
	out := map[string]interface{}{}
	points := extractStatPoints(raw)
	if len(points) == 0 {
		out["raw"] = raw
		return out
	}
	key := "series"
	lt := strings.ToLower(typeParam)
	switch {
	case strings.Contains(lt, "download") || strings.Contains(lt, "rx"):
		key = "net_rx"
	case strings.Contains(lt, "upload") || strings.Contains(lt, "tx"):
		key = "net_tx"
	case strings.Contains(lt, "cpu"):
		key = "cpu"
	case strings.Contains(lt, "mem"):
		key = "mem"
	}
	out[key] = points
	return out
}

func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case jsonNumber:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// jsonNumber 兼容 encoding/json.Number 而不强制 import（go-ovh 解码多为 float64）
type jsonNumber interface {
	Float64() (float64, error)
}

func extractStatPoints(raw interface{}) []map[string]interface{} {
	arr, ok := raw.([]interface{})
	if !ok {
		if m, ok2 := raw.(map[string]interface{}); ok2 {
			for _, k := range []string{"data", "values", "points", "statistics"} {
				if v, hit := m[k]; hit {
					return extractStatPoints(v)
				}
			}
		}
		return nil
	}
	points := make([]map[string]interface{}, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		ts, okTS := toFloat64(m["timestamp"])
		if !okTS {
			continue
		}
		var val float64
		switch v := m["value"].(type) {
		case map[string]interface{}:
			val, _ = toFloat64(v["value"])
		default:
			val, _ = toFloat64(v)
		}
		points = append(points, map[string]interface{}{"timestamp": ts, "value": val})
	}
	return points
}

// GetNetworkInterfaceStats GET /api/server-control/:service_name/network-stats
func GetNetworkInterfaceStats(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		state.Logger.Info("[Network] 获取服务器 "+svc+" 网络接口信息", "server_control")
		var macs []string
		if err := client.Get("/dedicated/server/"+svc+"/networkInterfaceController", &macs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 并发拉每张网卡详情
		details := parallelGetStringKeys(client, macs, func(m string) string {
			return "/dedicated/server/" + svc + "/networkInterfaceController/" + m
		}, 10)
		interfaces := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				interfaces = append(interfaces, d)
			}
		}
		state.Logger.Info(fmt.Sprintf("[Network] 找到 %d 个网络接口", len(interfaces)), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "interfaces": interfaces})
	}
}
