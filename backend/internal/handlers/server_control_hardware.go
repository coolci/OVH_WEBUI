package handlers

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
)

// GetHardwareInfo GET /api/server-control/:service_name/hardware
func GetHardwareInfo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var hardware map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/specifications/hardware", &hardware); err != nil {
			state.Logger.Error("获取服务器 "+svc+" 硬件信息失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 1:1 对应 Python app.py:6150-6167：缺字段补 N/A / 0 / {} / []，
		// 否则 JSON 序列化 null 让前端 .toLowerCase / .length 崩溃
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"hardware": gin.H{
				"bootMode":                valueOr(hardware, "bootMode", "N/A"),
				"coresPerProcessor":       defaultZero(hardware["coresPerProcessor"]),
				"threadsPerProcessor":     defaultZero(hardware["threadsPerProcessor"]),
				"numberOfProcessors":      defaultZero(hardware["numberOfProcessors"]),
				"processorName":           valueOr(hardware, "processorName", "N/A"),
				"processorArchitecture":   valueOr(hardware, "processorArchitecture", "N/A"),
				"memorySize":              defaultObj(hardware["memorySize"]),
				"motherboard":             valueOr(hardware, "motherboard", "N/A"),
				"formFactor":              valueOr(hardware, "formFactor", "N/A"),
				"description":             valueOr(hardware, "description", ""),
				"diskGroups":              defaultArr(hardware["diskGroups"]),
				"expansionCards":          defaultArr(hardware["expansionCards"]),
				"usbKeys":                 defaultArr(hardware["usbKeys"]),
				"defaultHardwareRaidSize": defaultObj(hardware["defaultHardwareRaidSize"]),
				"defaultHardwareRaidType": valueOr(hardware, "defaultHardwareRaidType", "N/A"),
			},
		})
	}
}

// GetNetworkSpecs GET /api/server-control/:service_name/network-specs
func GetNetworkSpecs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var network map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/specifications/network", &network); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"network": gin.H{
				"bandwidth":  network["bandwidth"],
				"connection": network["connection"],
				"ola":        network["ola"],
				"routing":    network["routing"],
				"traffic":    network["traffic"],
				"switching":  network["switching"],
				"vmac":       network["vmac"],
				"vrack":      network["vrack"],
			},
		})
	}
}

// GetServerIPs GET /api/server-control/:service_name/ips
// encodeIPBlock 将 OVH IP 块路径参数编码为 /ip/{block} 可用形式。
// 只转义 "/" → %2F，保留 IPv6 冒号（OVH 惯例）；PathEscape 整段会破坏 IPv6。
func encodeIPBlock(ip string) string {
	return strings.ReplaceAll(strings.TrimSpace(ip), "/", "%2F")
}

// inferIPMeta 在 /ip/{block} 详情失败时，从 CIDR 推断 family/type，避免 UI 显示 unknown。
func inferIPMeta(block string) (family, typ string) {
	raw := block
	if i := strings.Index(block, "/"); i >= 0 {
		raw = block[:i]
	}
	if strings.Contains(raw, ":") {
		return "ipv6", "dedicated"
	}
	if net.ParseIP(raw) != nil {
		return "ipv4", "dedicated"
	}
	return "unknown", "dedicated"
}

func GetServerIPs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var list []string
		if err := client.Get("/dedicated/server/"+svc+"/ips", &list); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		// 并发拉 /ip/{block} 详情；失败时记日志并做本地推断（不再硬写 type=unknown）
		type ipDetail struct {
			block  string
			detail map[string]interface{}
			err    error
		}
		results := make([]ipDetail, len(list))
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		for i, block := range list {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, blk string) {
				defer wg.Done()
				defer func() { <-sem }()
				results[idx].block = blk
				path := "/ip/" + encodeIPBlock(blk)
				var d map[string]interface{}
				if err := client.Get(path, &d); err != nil {
					// 再试：去掉前导零式的 IPv6 压缩差异不处理；改试 PathEscape 整段
					alt := "/ip/" + url.PathEscape(blk)
					if err2 := client.Get(alt, &d); err2 != nil {
						results[idx].err = err
						return
					}
				}
				results[idx].detail = d
			}(i, block)
		}
		wg.Wait()

		ips := make([]gin.H, 0, len(list))
		for _, r := range results {
			if r.detail == nil {
				family, typ := inferIPMeta(r.block)
				if r.err != nil {
					state.Logger.Warn(
						fmt.Sprintf("[IPs] %s 详情失败 block=%s: %s → 推断 type=%s family=%s",
							svc, r.block, r.err.Error(), typ, family),
						"server_control",
					)
				}
				ips = append(ips, gin.H{
					"ip":          r.block,
					"type":        typ,
					"family":      family,
					"description": "",
					"routedTo":    svc,
					"inferred":    true,
				})
				continue
			}
			routedTo := ""
			if rt, ok := r.detail["routedTo"].(map[string]interface{}); ok {
				if s, ok := rt["serviceName"].(string); ok {
					routedTo = s
				}
			}
			typ := "dedicated"
			if t := valueOr(r.detail, "type", ""); t != nil {
				if s, ok := t.(string); ok && s != "" {
					typ = s
				}
			}
			family, _ := inferIPMeta(r.block)
			if v, ok := r.detail["version"].(float64); ok {
				if int(v) == 6 {
					family = "ipv6"
				} else if int(v) == 4 {
					family = "ipv4"
				}
			}
			ips = append(ips, gin.H{
				"ip":          r.block,
				"type":        typ,
				"family":      family,
				"description": valueOr(r.detail, "description", ""),
				"routedTo":    routedTo,
				"inferred":    false,
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ips": ips, "total": len(ips)})
	}
}

// GetReverseDNS GET /api/server-control/:service_name/reverse
//
// 反向 DNS 不在 /dedicated/server/ 域下,而在 /ip/{ipBlock}/reverse。
// 流程:服务器 IP 块 → 每块查 reverse 列表 → 每个 IP 查 reverse 详情。
func GetReverseDNS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var ipBlocks []string
		if err := client.Get("/dedicated/server/"+svc+"/ips", &ipBlocks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 1) 每个 IP 块并发拉 reverse 列表(块下哪些具体 IP 配了反向)
		type blockResult struct {
			block string
			ips   []string
		}
		blockResults := make([]blockResult, len(ipBlocks))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for i, blk := range ipBlocks {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, block string) {
				defer wg.Done()
				defer func() { <-sem }()
				encoded := strings.ReplaceAll(block, "/", "%2F")
				var ips []string
				_ = client.Get("/ip/"+encoded+"/reverse", &ips)
				blockResults[idx] = blockResult{block: block, ips: ips}
			}(i, blk)
		}
		wg.Wait()

		// 2) 把 (block, ip) 配对展开,并发拉每条 reverse 详情
		type entry struct {
			block string
			ip    string
		}
		entries := []entry{}
		for _, r := range blockResults {
			for _, ip := range r.ips {
				entries = append(entries, entry{block: r.block, ip: ip})
			}
		}
		details := make([]map[string]interface{}, len(entries))
		var wg2 sync.WaitGroup
		sem2 := make(chan struct{}, 10)
		for i, e := range entries {
			wg2.Add(1)
			sem2 <- struct{}{}
			go func(idx int, en entry) {
				defer wg2.Done()
				defer func() { <-sem2 }()
				encoded := strings.ReplaceAll(en.block, "/", "%2F")
				var d map[string]interface{}
				if err := client.Get("/ip/"+encoded+"/reverse/"+en.ip, &d); err == nil {
					details[idx] = d
				}
			}(i, e)
		}
		wg2.Wait()

		reverseList := []gin.H{}
		for i, e := range entries {
			if details[i] == nil {
				continue
			}
			reverseList = append(reverseList, gin.H{
				"ipReverse": e.ip,
				"reverse":   details[i]["reverse"],
				"ipBlock":   e.block,
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "reverses": reverseList})
	}
}

// SetReverseDNS POST /api/server-control/:service_name/reverse
//
// 设反向 DNS 也走 /ip/{ipBlock}/reverse。需要先找到 body.IP 所属的 IP 块。
func SetReverseDNS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			IP      string `json:"ip"`
			Reverse string `json:"reverse"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.IP == "" || body.Reverse == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "IP地址和反向DNS不能为空"})
			return
		}
		// 找该 IP 所在的服务器 IP 块
		block, err := findIPBlockForServer(client, svc, body.IP)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		encoded := strings.ReplaceAll(block, "/", "%2F")
		if err := client.Post("/ip/"+encoded+"/reverse", map[string]interface{}{
			"ipReverse": body.IP,
			"reverse":   body.Reverse,
		}, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("服务器 "+svc+" IP "+body.IP+" 反向DNS已设置为 "+body.Reverse, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "反向DNS已设置"})
	}
}

// DeleteReverseDNS DELETE /api/server-control/:service_name/reverse/:ip
// OVH 的 DELETE /ip/{ipBlock}/reverse/{ipReverse} 删除单条反向记录。
func DeleteReverseDNS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		ip := c.Param("ip")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		block, err := findIPBlockForServer(client, svc, ip)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		encoded := strings.ReplaceAll(block, "/", "%2F")
		if err := client.Delete("/ip/"+encoded+"/reverse/"+ip, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("服务器 "+svc+" IP "+ip+" 反向DNS已删除", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "反向DNS已删除"})
	}
}

// findIPBlockForServer 在服务器的 IP 块列表里找出包含给定 IPv4 的那个块。
// 用 net.ParseCIDR + Contains 判定;返回精确块字符串(如 "1.2.3.0/29")。
func findIPBlockForServer(client interface {
	Get(path string, result interface{}) error
}, svc, ipStr string) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("非法 IP: %s", ipStr)
	}
	var blocks []string
	if err := client.Get("/dedicated/server/"+svc+"/ips", &blocks); err != nil {
		return "", err
	}
	for _, blk := range blocks {
		_, ipnet, err := net.ParseCIDR(blk)
		if err != nil {
			// 老格式可能没带 mask,补 /32 / /128 再 parse
			if strings.Contains(blk, ":") {
				blk = blk + "/128"
			} else {
				blk = blk + "/32"
			}
			_, ipnet, err = net.ParseCIDR(blk)
			if err != nil {
				continue
			}
		}
		if ipnet.Contains(ip) {
			return blk, nil
		}
	}
	return "", fmt.Errorf("IP %s 不在服务器 %s 的任何 IP 块内", ipStr, svc)
}

// GetServiceInfo GET /api/server-control/:service_name/serviceinfo
func GetServiceInfo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var info map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/serviceInfos", &info); err != nil {
			state.Logger.Error("获取服务器 "+svc+" 服务信息失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
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
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"serviceInfo": gin.H{
				"status":                    valueOr(info, "status", "unknown"),
				"expiration":                valueOr(info, "expiration", ""),
				"creation":                  valueOr(info, "creation", ""),
				"renewalType":               automatic, // 自动续费 yes/no
				"renewalPeriod":             period,    // 续费周期(月)
				"renewalDeleteAtExpiration": deleteAtExpiration,
				"renewalForced":             forced, // OVH 强制自动续费(不能改)
				"renewalManualPayment":      manualPayment,
				"possibleRenewPeriod":       possiblePeriods,
			},
		})
	}
}

// UpdateServiceRenewal PUT /api/server-control/:service_name/serviceinfo
//
// 修改服务的续费策略。前端传 mode (auto / manual / delete-at-expiration) + 可选 period,
// 后端先 GET 当前 serviceInfos,合并 renew 字段,再 PUT 整体回去(OVH PUT 要求完整对象)。
// forced=true (engaged 合同期) 时 OVH 会拒,我们这里直接返 400 提示用户。
func UpdateServiceRenewal(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Mode   string `json:"mode"`   // "auto" / "manual" / "delete"
			Period int    `json:"period"` // 月数,0 表示不改
		}
		_ = c.ShouldBindJSON(&body)

		var info map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/serviceInfos", &info); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
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
				"error":   "该服务器处于 OVH 合同期(engaged),续费策略由 OVH 锁定,无法修改",
			})
			return
		}
		switch body.Mode {
		case "auto":
			renew["automatic"] = true
			renew["deleteAtExpiration"] = false
			renew["manualPayment"] = false
		case "manual":
			renew["automatic"] = false
			renew["deleteAtExpiration"] = false
			renew["manualPayment"] = true
		case "delete":
			renew["automatic"] = false
			renew["deleteAtExpiration"] = true
			renew["manualPayment"] = false
		default:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "mode 必须是 auto / manual / delete 之一"})
			return
		}
		if body.Period > 0 {
			renew["period"] = body.Period
		}
		info["renew"] = renew

		// PUT 整对象回去(OVH 这个端点要求完整 services.Service)
		if err := client.Put("/dedicated/server/"+svc+"/serviceInfos", info, nil); err != nil {
			state.Logger.Error("修改服务器 "+svc+" 续费策略失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("服务器 "+svc+" 续费策略已更新: mode="+body.Mode, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "续费策略已更新"})
	}
}

// ChangeContact POST /api/server-control/:service_name/change-contact
func ChangeContact(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
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
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "至少需要指定一个联系人（管理员、技术或计费）"})
			return
		}
		// OVH 这个接口返回 long[](任务 ID 数组),不是 map
		var taskIDs []int64
		if err := client.Post("/dedicated/server/"+svc+"/changeContact", params, &taskIDs); err != nil {
			state.Logger.Error("变更服务器 "+svc+" 联系人失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("服务器 %s 联系人变更请求已提交: %v, tasks=%v", svc, params, taskIDs), "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "联系人变更请求已提交",
			"taskIds": taskIDs,
		})
	}
}

// GetInterventions GET /api/server-control/:service_name/interventions
func GetInterventions(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/dedicated/server/"+svc+"/intervention", &ids); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 并发拉详情
		details := parallelGetDetails(client, ids, func(k interface{}) string {
			return "/dedicated/server/" + svc + "/intervention/" + idToString(k)
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "interventions": list})
	}
}

// GetInterventionDetail GET /api/server-control/:service_name/interventions/:intervention_id
func GetInterventionDetail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		id := c.Param("intervention_id")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/intervention/"+id, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "intervention": d})
	}
}

// GetPlannedInterventions GET /api/server-control/:service_name/planned-interventions
func GetPlannedInterventions(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/dedicated/server/"+svc+"/plannedIntervention", &ids); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 并发拉详情
		details := parallelGetDetails(client, ids, func(k interface{}) string {
			return "/dedicated/server/" + svc + "/plannedIntervention/" + idToString(k)
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "plannedInterventions": list})
	}
}

// GetPlannedInterventionDetail GET /api/server-control/:service_name/planned-interventions/:intervention_id
func GetPlannedInterventionDetail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		id := c.Param("intervention_id")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d map[string]interface{}
		if err := client.Get(fmt.Sprintf("/dedicated/server/%s/plannedIntervention/%s", svc, id), &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "plannedIntervention": d})
	}
}

// HardwareReplace POST /api/server-control/:service_name/hardware/replace
func HardwareReplace(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body map[string]interface{}
		_ = c.ShouldBindJSON(&body)
		componentType, _ := body["componentType"].(string)
		comment, _ := body["comment"].(string)
		if componentType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 componentType 参数"})
			return
		}
		var result map[string]interface{}
		var err2 error
		switch componentType {
		case "hardDiskDrive":
			if comment == "" {
				comment = "Request hard disk drive replacement - faulty disk detected"
			}
			err2 = client.Post("/dedicated/server/"+svc+"/support/replace/hardDiskDrive", map[string]interface{}{
				"comment": comment,
				"disks":   []interface{}{},
				"inverse": true,
			}, &result)
		case "memory":
			details := "Memory module failure"
			if v, ok := body["details"].(string); ok && v != "" {
				details = v
			}
			if comment == "" {
				comment = "Request memory module replacement - hardware failure detected"
			}
			err2 = client.Post("/dedicated/server/"+svc+"/support/replace/memory", map[string]interface{}{
				"comment":          comment,
				"details":          details,
				"slotsDescription": "",
			}, &result)
		case "cooling":
			details := "Cooling system failure"
			if v, ok := body["details"].(string); ok && v != "" {
				details = v
			}
			if comment == "" {
				comment = "Request cooling system replacement - fan failure or overheating"
			}
			err2 = client.Post("/dedicated/server/"+svc+"/support/replace/cooling", map[string]interface{}{
				"comment": comment,
				"details": details,
			}, &result)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "不支持的组件类型: " + componentType})
			return
		}
		if err2 != nil {
			errMsg := err2.Error()
			state.Logger.Error("硬件更换失败: "+svc+" - "+componentType+" - "+errMsg, "server_control")
			if strings.Contains(errMsg, "Action pending") {
				ticketID := "未知"
				if m := regexp.MustCompile(`ticketId[:\s]+(\d+)`).FindStringSubmatch(errMsg); m != nil {
					ticketID = m[1]
				}
				c.JSON(http.StatusBadRequest, gin.H{
					"success":   false,
					"error":     "已有待处理的硬件更换工单 (Ticket #" + ticketID + ")，请等待完成后再提交新请求",
					"ticketId":  ticketID,
					"isPending": true,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": errMsg})
			return
		}
		state.Logger.Info("硬件更换请求已发送: "+svc+" - "+componentType, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "硬件更换请求已发送", "task": result})
	}
}

// GetHardwareRaidProfiles GET /api/server-control/:service_name/hardware-raid-profiles
func GetHardwareRaidProfiles(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var profiles interface{}
		if err := client.Get("/dedicated/server/"+svc+"/install/hardwareRaidProfile", &profiles); err != nil {
			errMsg := strings.ToLower(err.Error())
			if strings.Contains(errMsg, "not supported") {
				c.JSON(http.StatusOK, gin.H{
					"success":   true,
					"profiles":  []interface{}{},
					"supported": false,
					"message":   "此服务器不支持硬件RAID",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "profiles": profiles, "supported": true})
	}
}

// GetHardwareDiskInfo GET /api/server-control/:service_name/hardware-disk-info
func GetHardwareDiskInfo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var hardware map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/specifications/hardware", &hardware); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		diskGroups := map[string]interface{}{}
		if dgs, ok := hardware["diskGroups"].([]interface{}); ok {
			for _, dgRaw := range dgs {
				dg, ok := dgRaw.(map[string]interface{})
				if !ok {
					continue
				}
				id := 0
				if v, ok := numconv.ToInt64(dg["diskGroupId"]); ok {
					id = int(v)
				}
				numberOfDisks := 0
				if v, ok := numconv.ToInt64(dg["numberOfDisks"]); ok {
					numberOfDisks = int(v)
				}
				diskSizeValue := 0
				diskSizeUnit := "GB"
				if ds, ok := dg["diskSize"].(map[string]interface{}); ok {
					if v, ok := numconv.ToInt64(ds["value"]); ok {
						diskSizeValue = int(v)
					}
					if u, ok := ds["unit"].(string); ok {
						diskSizeUnit = u
					}
				}
				disks := []map[string]interface{}{}
				for i := 0; i < numberOfDisks; i++ {
					disks = append(disks, map[string]interface{}{
						"capacity": diskSizeValue,
						"unit":     diskSizeUnit,
						"number":   i + 1,
						"diskType": dg["diskType"],
					})
				}
				diskGroups[fmt.Sprintf("%d", id)] = map[string]interface{}{
					"id":             id,
					"diskType":       dg["diskType"],
					"description":    dg["description"],
					"raidController": dg["raidController"],
					"disks":          disks,
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "diskGroups": diskGroups, "hardware": hardware})
	}
}

// GetPartitionSchemes GET /api/server-control/:service_name/partition-schemes
func GetPartitionSchemes(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		templateName := c.Query("templateName")
		if templateName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少templateName参数"})
			return
		}
		encodedTpl := url.PathEscape(templateName)
		var schemes []string
		if err := client.Get("/dedicated/installationTemplate/"+encodedTpl+"/partitionScheme", &schemes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 双层嵌套并发：先并发拉每个 scheme 的 info + partition list，
		// 再对每个 scheme 内的 partition 并发拉详情
		type schemeResult struct {
			name       string
			info       map[string]interface{}
			parts      []string
			missingInfo bool
		}
		schemeResults := make([]schemeResult, len(schemes))
		sem := make(chan struct{}, 10)
		var wg sync.WaitGroup
		for i, schemeName := range schemes {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, sname string) {
				defer wg.Done()
				defer func() { <-sem }()
				es := url.PathEscape(sname)
				var info map[string]interface{}
				if err := client.Get("/dedicated/installationTemplate/"+encodedTpl+"/partitionScheme/"+es, &info); err != nil {
					schemeResults[idx] = schemeResult{name: sname, missingInfo: true}
					return
				}
				var partitions []string
				_ = client.Get("/dedicated/installationTemplate/"+encodedTpl+"/partitionScheme/"+es+"/partition", &partitions)
				schemeResults[idx] = schemeResult{name: sname, info: info, parts: partitions}
			}(i, schemeName)
		}
		wg.Wait()

		details := []gin.H{}
		for _, sr := range schemeResults {
			if sr.missingInfo {
				details = append(details, gin.H{"name": sr.name, "priority": 0, "partitions": []interface{}{}})
				continue
			}
			priority := 0
			if v, ok := numconv.ToInt64(sr.info["priority"]); ok {
				priority = int(v)
			}
			// 并发拉该 scheme 的 partition 详情
			encodedScheme := url.PathEscape(sr.name)
			partDetails := make([]gin.H, len(sr.parts))
			pSem := make(chan struct{}, 10)
			var pWg sync.WaitGroup
			for pi, part := range sr.parts {
				pWg.Add(1)
				pSem <- struct{}{}
				go func(pidx int, p string) {
					defer pWg.Done()
					defer func() { <-pSem }()
					var partInfo map[string]interface{}
					if err := client.Get("/dedicated/installationTemplate/"+encodedTpl+"/partitionScheme/"+encodedScheme+"/partition/"+url.PathEscape(p), &partInfo); err != nil {
						return
					}
					order := 0
					if v, ok := numconv.ToInt64(partInfo["order"]); ok {
						order = int(v)
					}
					partDetails[pidx] = gin.H{
						"mountpoint": p,
						"filesystem": valueOr(partInfo, "filesystem", ""),
						"size":       defaultZero(partInfo["size"]),
						"order":      order,
						"raid":       partInfo["raid"],
						"type":       valueOr(partInfo, "type", "primary"),
					}
				}(pi, part)
			}
			pWg.Wait()
			// 去掉拉取失败的 nil 项
			cleaned := make([]gin.H, 0, len(partDetails))
			for _, pd := range partDetails {
				if pd != nil {
					cleaned = append(cleaned, pd)
				}
			}
			// 冒泡按 order 排序
			for i := 1; i < len(cleaned); i++ {
				for j := i; j > 0; j-- {
					oi, _ := cleaned[j]["order"].(int)
					oj, _ := cleaned[j-1]["order"].(int)
					if oj > oi {
						cleaned[j-1], cleaned[j] = cleaned[j], cleaned[j-1]
					}
				}
			}
			details = append(details, gin.H{
				"name":       sr.name,
				"priority":   priority,
				"partitions": cleaned,
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "schemes": details})
	}
}
