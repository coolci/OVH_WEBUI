package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/types"
)

// cleanIPParam 清理 URL 通配段参数里的 IP，处理带掩码的 CIDR 与 URL 编码
func cleanIPParam(raw string) string {
	val, err := url.PathUnescape(raw)
	if err != nil {
		val = raw
	}
	val = strings.TrimSpace(val)
	val = strings.TrimPrefix(val, "/")
	return val
}

// fetchIPsForAccount 拉取指定账户下的 IP 列表并附带账户元数据与独服主 IP 回补
type serverBinding struct {
	serviceName string
	serverType  string // "dedicated" | "vps"
}

// fetchIPsForAccount 拉取指定账户下的 IP 列表并附带账户元数据，自动从独服与 VPS 资源库纠偏未知类型与未挂载状态
func fetchIPsForAccount(state *app.State, acc types.OVHAccount) []map[string]interface{} {
	client, err := state.OVH.ClientFor(acc.ID)
	if err != nil {
		return nil
	}

	var ipList []string
	if err := client.Get("/ip", &ipList); err != nil {
		state.Logger.Warn(fmt.Sprintf("获取账户 %s (%s) IP 列表失败: %s", acc.Name, acc.ID, err.Error()), "ip_management")
	}

	var list []map[string]interface{}

	if len(ipList) > 0 {
		details, _ := parallelGetStringKeysWithErrs(client, ipList, func(ip string) string {
			return "/ip/" + strings.ReplaceAll(ip, "/", "%2F")
		}, 10)

		for i, d := range details {
			item := d
			if item == nil {
				item = map[string]interface{}{
					"ip":   ipList[i],
					"type": "unknown",
				}
			}
			item["accountId"] = acc.ID
			item["accountName"] = acc.Name
			item["accountZone"] = acc.Zone
			list = append(list, item)
		}
	}

	// 建立该账户下所有独服与 VPS 的 IP 映射池（用于回填 OVH /ip 详情 404 或未关联的条目）
	bindings := make(map[string]serverBinding)

	// 1. 扫描独服列表与绑定的具体 IP
	var dedicatedServers []string
	if err := client.Get("/dedicated/server", &dedicatedServers); err == nil && len(dedicatedServers) > 0 {
		for _, sName := range dedicatedServers {
			var ips []string
			if err := client.Get("/dedicated/server/"+url.PathEscape(sName)+"/ips", &ips); err == nil {
				for _, ipStr := range ips {
					clean := strings.Split(ipStr, "/")[0]
					b := serverBinding{serviceName: sName, serverType: "dedicated"}
					bindings[clean] = b
					bindings[ipStr] = b
				}
			}
		}
	}

	// 2. 扫描 VPS 列表与绑定的具体 IP
	var vpsList []string
	if err := client.Get("/vps", &vpsList); err == nil && len(vpsList) > 0 {
		for _, vName := range vpsList {
			var ips []string
			if err := client.Get("/vps/"+url.PathEscape(vName)+"/ips", &ips); err == nil {
				for _, ipStr := range ips {
					clean := strings.Split(ipStr, "/")[0]
					b := serverBinding{serviceName: vName, serverType: "vps"}
					bindings[clean] = b
					bindings[ipStr] = b
				}
			}
		}
	}

	// 3. 关联修复：对已有列表中未挂载或 unknown 的 IP，用 bindings 自动补全
	seenIPs := make(map[string]bool)
	for _, item := range list {
		ipStr, _ := item["ip"].(string)
		clean := strings.Split(ipStr, "/")[0]
		seenIPs[clean] = true
		seenIPs[ipStr] = true

		// 检查当前 item 是否缺少挂载目标或 serviceName 为空
		routedTo, _ := item["routedTo"].(map[string]interface{})
		currentService, _ := routedTo["serviceName"].(string)

		if currentService == "" {
			if b, ok := bindings[clean]; ok {
				item["routedTo"] = map[string]interface{}{"serviceName": b.serviceName}
				item["type"] = b.serverType
			} else if b, ok := bindings[ipStr]; ok {
				item["routedTo"] = map[string]interface{}{"serviceName": b.serviceName}
				item["type"] = b.serverType
			}
		}

		// 修复 unknown 类型
		if t, _ := item["type"].(string); t == "" || t == "unknown" {
			if r, ok := item["routedTo"].(map[string]interface{}); ok {
				if s, ok := r["serviceName"].(string); ok && s != "" {
					sLower := strings.ToLower(s)
					if strings.HasPrefix(sLower, "vps-") || strings.Contains(sLower, ".vps.") {
						item["type"] = "vps"
					} else {
						item["type"] = "dedicated"
					}
				}
			}
		}
	}

	// 4. 补充未出现在 /ip 里的独服与 VPS 机器 IP
	for ipKey, b := range bindings {
		if strings.Contains(ipKey, "/") { // 只处理带掩码的主条目，避免 clean 和 ipStr 重复入队
			clean := strings.Split(ipKey, "/")[0]
			if !seenIPs[clean] && !seenIPs[ipKey] {
				seenIPs[clean] = true
				seenIPs[ipKey] = true
				list = append(list, map[string]interface{}{
					"ip":          ipKey,
					"type":        b.serverType,
					"accountId":   acc.ID,
					"accountName": acc.Name,
					"accountZone": acc.Zone,
					"routedTo": map[string]interface{}{
						"serviceName": b.serviceName,
					},
				})
			}
		}
	}

	return list
}

// ListIPs GET /api/ip
// 获取账户下所有 IP 资产（主 IP、附加 Failover IP、Cloud IP 等，支持 account=all 全局聚合）
func ListIPs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqAccount := strings.TrimSpace(c.Query("account"))

		if strings.EqualFold(reqAccount, "all") {
			state.AccountsMu.RLock()
			accs := append([]types.OVHAccount(nil), state.Accounts...)
			state.AccountsMu.RUnlock()

			var allList []map[string]interface{}
			var mu sync.Mutex
			var wg sync.WaitGroup

			for _, acc := range accs {
				wg.Add(1)
				go func(a types.OVHAccount) {
					defer wg.Done()
					subList := fetchIPsForAccount(state, a)
					if len(subList) > 0 {
						mu.Lock()
						allList = append(allList, subList...)
						mu.Unlock()
					}
				}(acc)
			}
			wg.Wait()

			sort.Slice(allList, func(i, j int) bool {
				ipI, _ := allList[i]["ip"].(string)
				ipJ, _ := allList[j]["ip"].(string)
				return ipI < ipJ
			})

			c.JSON(http.StatusOK, gin.H{"success": true, "ips": allList})
			return
		}

		acc, ok := ovhAccountFor(state, c)
		if !ok {
			noOVHResp(c)
			return
		}

		list := fetchIPsForAccount(state, acc)
		if list == nil {
			list = []map[string]interface{}{}
		}

		// 按 IP 排序
		sort.Slice(list, func(i, j int) bool {
			ipI, _ := list[i]["ip"].(string)
			ipJ, _ := list[j]["ip"].(string)
			return ipI < ipJ
		})

		c.JSON(http.StatusOK, gin.H{"success": true, "ips": list})
	}
}

// GetIPDetail GET /api/ip/details/*ip
// 获取单个 IP 块的详细信息
func GetIPDetail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var detail map[string]interface{}
		if err := client.Get("/ip/"+url.PathEscape(ip), &detail); err != nil {
			state.Logger.Error(fmt.Sprintf("获取 IP %s 详情失败: %s", ip, err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "detail": detail})
	}
}

// MoveIPToService POST /api/ip/move/*ip
// 将 Failover IP 一键移动（漂移）到指定目标服务器/VPS
func MoveIPToService(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		var body struct {
			To string `json:"to"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.To) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "必须指定目标服务器 (to)"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var result map[string]interface{}
		payload := map[string]interface{}{
			"to": strings.TrimSpace(body.To),
		}
		if err := client.Post("/ip/"+url.PathEscape(ip)+"/move", payload, &result); err != nil {
			state.Logger.Error(fmt.Sprintf("移动 IP %s 至 %s 失败: %s", ip, body.To, err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "移动 IP 失败: " + err.Error()})
			return
		}

		state.Logger.Info(fmt.Sprintf("IP %s 漂移至 %s 任务已提交", ip, body.To), "ip_management")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("IP %s 正在移动至 %s", ip, body.To),
			"result":  result,
		})
	}
}

// GetIPReverse GET /api/ip/reverse/*ip
// 获取指定 IP 块的所有反向解析 (rDNS / PTR)
func GetIPReverse(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var reverseIPs []string
		if err := client.Get("/ip/"+url.PathEscape(ip)+"/reverse", &reverseIPs); err != nil {
			state.Logger.Error(fmt.Sprintf("获取 IP %s 反向解析失败: %s", ip, err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		if len(reverseIPs) == 0 {
			c.JSON(http.StatusOK, gin.H{"success": true, "reverses": []map[string]interface{}{}})
			return
		}

		details, _ := parallelGetStringKeysWithErrs(client, reverseIPs, func(revIP string) string {
			return "/ip/" + url.PathEscape(ip) + "/reverse/" + url.PathEscape(revIP)
		}, 10)

		var list []map[string]interface{}
		for i, d := range details {
			if d != nil {
				list = append(list, d)
			} else {
				list = append(list, map[string]interface{}{
					"ipReverse": reverseIPs[i],
					"reverse":   "",
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "reverses": list})
	}
}

// SetIPReverse POST /api/ip/reverse/*ip
// 设置/添加 PTR 反向解析
func SetIPReverse(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		var body struct {
			IPReverse string `json:"ipReverse"`
			Reverse   string `json:"reverse"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求参数错误: " + err.Error()})
			return
		}

		ipRev := strings.TrimSpace(body.IPReverse)
		targetRev := strings.TrimSpace(body.Reverse)
		if ipRev == "" || targetRev == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ipReverse 与 reverse 均不能为空"})
			return
		}

		// 确保域名以点结尾（OVH 规范）
		if !strings.HasSuffix(targetRev, ".") {
			targetRev = targetRev + "."
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		payload := map[string]interface{}{
			"ipReverse": ipRev,
			"reverse":   targetRev,
		}

		var result map[string]interface{}
		if err := client.Post("/ip/"+url.PathEscape(ip)+"/reverse", payload, &result); err != nil {
			state.Logger.Error(fmt.Sprintf("设置 IP %s PTR 为 %s 失败: %s", ipRev, targetRev, err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "设置反向解析失败: " + err.Error()})
			return
		}

		state.Logger.Info(fmt.Sprintf("设置 IP %s PTR 为 %s 成功", ipRev, targetRev), "ip_management")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "反向解析已设置", "result": result})
	}
}

// DeleteIPReverse DELETE /api/ip/reverse/*ip
// 删除 PTR 反向解析
func DeleteIPReverse(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		ipRev := strings.TrimSpace(c.Query("ipReverse"))
		if ipRev == "" {
			var body struct {
				IPReverse string `json:"ipReverse"`
			}
			_ = c.ShouldBindJSON(&body)
			ipRev = strings.TrimSpace(body.IPReverse)
		}
		if ipRev == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 ipReverse 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		if err := client.Delete("/ip/"+url.PathEscape(ip)+"/reverse/"+url.PathEscape(ipRev), nil); err != nil {
			state.Logger.Error(fmt.Sprintf("删除 IP %s 反向解析失败: %s", ipRev, err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		state.Logger.Info(fmt.Sprintf("删除 IP %s 反向解析成功", ipRev), "ip_management")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "反向解析已删除"})
	}
}

// GetIPFirewall GET /api/ip/firewall/*ip
// 获取 IP 的 Edge 硬件防火墙状态与启用列表
func GetIPFirewall(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var firewallIPs []string
		if err := client.Get("/ip/"+url.PathEscape(ip)+"/firewall", &firewallIPs); err != nil {
			state.Logger.Error(fmt.Sprintf("获取 IP %s 防火墙列表失败: %s", ip, err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		if len(firewallIPs) == 0 {
			c.JSON(http.StatusOK, gin.H{"success": true, "firewalls": []map[string]interface{}{}})
			return
		}

		details, _ := parallelGetStringKeysWithErrs(client, firewallIPs, func(fwIP string) string {
			return "/ip/" + url.PathEscape(ip) + "/firewall/" + url.PathEscape(fwIP)
		}, 10)

		var list []map[string]interface{}
		for i, d := range details {
			if d != nil {
				list = append(list, d)
			} else {
				list = append(list, map[string]interface{}{
					"ipOnFirewall": firewallIPs[i],
					"enabled":      false,
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "firewalls": list})
	}
}

// CreateIPFirewall POST /api/ip/firewall/*ip
// 在指定具体 IP 上创建/启用硬件防火墙
func CreateIPFirewall(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		var body struct {
			IPOnFirewall string `json:"ipOnFirewall"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.IPOnFirewall) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 ipOnFirewall 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		payload := map[string]interface{}{
			"ipOnFirewall": strings.TrimSpace(body.IPOnFirewall),
		}
		var result map[string]interface{}
		if err := client.Post("/ip/"+url.PathEscape(ip)+"/firewall", payload, &result); err != nil {
			state.Logger.Error(fmt.Sprintf("创建 IP %s 硬件防火墙失败: %s", body.IPOnFirewall, err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "硬件防火墙已创建", "result": result})
	}
}

// ToggleIPFirewall PUT /api/ip/firewall/*ip
// 切换防火墙启用/停用状态
func ToggleIPFirewall(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		var body struct {
			IPOnFirewall string `json:"ipOnFirewall"`
			Enabled      bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.IPOnFirewall) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 ipOnFirewall 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		payload := map[string]interface{}{
			"enabled": body.Enabled,
		}
		if err := client.Put("/ip/"+url.PathEscape(ip)+"/firewall/"+url.PathEscape(body.IPOnFirewall), payload, nil); err != nil {
			state.Logger.Error(fmt.Sprintf("更新 IP %s 防火墙状态失败: %s", body.IPOnFirewall, err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "防火墙状态已更新"})
	}
}

// ListIPFirewallRules GET /api/ip/firewall-rules/*ip
// 列出具体 IP 的硬件防火墙规则
func ListIPFirewallRules(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		ipOnFirewall := strings.TrimSpace(c.Query("ipOnFirewall"))
		if ipOnFirewall == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 ipOnFirewall 查询参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var sequences []interface{}
		path := fmt.Sprintf("/ip/%s/firewall/%s/rule", url.PathEscape(ip), url.PathEscape(ipOnFirewall))
		if err := client.Get(path, &sequences); err != nil {
			state.Logger.Error(fmt.Sprintf("获取 IP %s 规则列表失败: %s", ipOnFirewall, err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		details := parallelGetDetails(client, sequences, func(seq interface{}) string {
			return fmt.Sprintf("/ip/%s/firewall/%s/rule/%v", url.PathEscape(ip), url.PathEscape(ipOnFirewall), seq)
		}, 10)

		var rules []map[string]interface{}
		for _, d := range details {
			if d != nil {
				rules = append(rules, d)
			}
		}

		sort.Slice(rules, func(i, j int) bool {
			sI, _ := numconv.ToInt64(rules[i]["sequence"])
			sJ, _ := numconv.ToInt64(rules[j]["sequence"])
			return sI < sJ
		})

		c.JSON(http.StatusOK, gin.H{"success": true, "rules": rules})
	}
}

// CreateIPFirewallRule POST /api/ip/firewall-rules/*ip
// 添加一条硬件防火墙规则
func CreateIPFirewallRule(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		var body struct {
			IPOnFirewall    string  `json:"ipOnFirewall"`
			Action          string  `json:"action"` // permit / deny
			Protocol        string  `json:"protocol"` // tcp / udp / icmp / ipv4
			Sequence        int64   `json:"sequence"`
			DestinationPort *string `json:"destinationPort,omitempty"`
			SourcePort      *string `json:"sourcePort,omitempty"`
			SourceIP        *string `json:"source,omitempty"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.IPOnFirewall) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少关键参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		payload := map[string]interface{}{
			"action":   body.Action,
			"protocol": body.Protocol,
			"sequence": body.Sequence,
		}
		if body.DestinationPort != nil && *body.DestinationPort != "" {
			if n, err := strconv.Atoi(*body.DestinationPort); err == nil {
				payload["destinationPort"] = n
			}
		}
		if body.SourcePort != nil && *body.SourcePort != "" {
			if n, err := strconv.Atoi(*body.SourcePort); err == nil {
				payload["sourcePort"] = n
			}
		}
		if body.SourceIP != nil && strings.TrimSpace(*body.SourceIP) != "" {
			payload["source"] = strings.TrimSpace(*body.SourceIP)
		}

		var result map[string]interface{}
		path := fmt.Sprintf("/ip/%s/firewall/%s/rule", url.PathEscape(ip), url.PathEscape(body.IPOnFirewall))
		if err := client.Post(path, payload, &result); err != nil {
			state.Logger.Error(fmt.Sprintf("创建规则失败: %s", err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "规则已添加", "result": result})
	}
}

// DeleteIPFirewallRule DELETE /api/ip/firewall-rules/*ip
// 删除硬件防火墙规则
func DeleteIPFirewallRule(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := cleanIPParam(c.Param("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 IP 参数"})
			return
		}

		ipOnFirewall := strings.TrimSpace(c.Query("ipOnFirewall"))
		seq := strings.TrimSpace(c.Query("sequence"))
		if ipOnFirewall == "" || seq == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 ipOnFirewall 或 sequence 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		path := fmt.Sprintf("/ip/%s/firewall/%s/rule/%s", url.PathEscape(ip), url.PathEscape(ipOnFirewall), seq)
		if err := client.Delete(path, nil); err != nil {
			state.Logger.Error(fmt.Sprintf("删除规则失败: %s", err.Error()), "ip_management")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "规则已删除"})
	}
}
