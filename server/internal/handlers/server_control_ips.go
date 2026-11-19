package handlers

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// GetServerIPs GET /api/server-control/:service_name/ips
func GetServerIPs(state *app.State) gin.HandlerFunc {
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
		ipsPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ips")
		if !ok {
			return
		}
		var list []string
		if err := client.Get(ipsPath, &list); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		// 并发拉每个 IP 的详情
		details, _ := parallelGetStringKeysWithErrs(client, list, func(ip string) string {
			return safePathFn("ip", ip)
		}, 10)
		ips := []gin.H{}
		for i, ip := range list {
			detail := details[i]
			if detail == nil {
				// OVH 的 /ip/{ip} 接口中，部分区域（如加拿大 BHS 机房）的自带默认 IP 未在全局 IP 库建档，
				// 会返回 404 (The requested object does not exist)。但该 IP 本身即属于该独立服务器，
				// 因此默认标识为 dedicated（主服务器专用 IP）而非 unknown。
				ips = append(ips, gin.H{
					"ip":       ip,
					"type":     "dedicated",
					"routedTo": svc,
				})
				continue
			}
			routedTo := ""
			if r, ok := detail["routedTo"].(map[string]interface{}); ok {
				if s, ok := r["serviceName"].(string); ok {
					routedTo = s
				}
			}
			if routedTo == "" {
				routedTo = svc
			}
			ips = append(ips, gin.H{
				"ip":          ip,
				"type":        valueOr(detail, "type", "dedicated"),
				"description": valueOr(detail, "description", ""),
				"routedTo":    routedTo,
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
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		ipsPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ips")
		if !ok {
			return
		}
		var ipBlocks []string
		if err := client.Get(ipsPath, &ipBlocks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		// 1) 每个 IP 块并发拉 reverse 列表(块下哪些具体 IP 配了反向)
		// /ip/** 与 /dedicated/server/** 是两个独立授权域,上一步成功不代表这一步有权限,
		// 所以每块的 error 都要留下来,不能像原来那样 `_ =` 丢掉。
		type blockResult struct {
			block string
			ips   []string
		}
		blockResults := make([]blockResult, len(ipBlocks))
		blockErrs := make([]error, len(ipBlocks))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for i, blk := range ipBlocks {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, block string) {
				defer wg.Done()
				defer func() { <-sem }()
				revPath, pErr := buildOVHPath("ip", block, "reverse")
				if pErr != nil {
					blockErrs[idx] = pErr
					return
				}
				var ips []string
				client, err := ovh.IsolateClient(client)
				if err != nil {
					blockErrs[idx] = err

					return
				}
				if err := client.Get(revPath, &ips); err != nil {
					blockErrs[idx] = err
				}
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
		detailErrs := make([]error, len(entries))
		var wg2 sync.WaitGroup
		sem2 := make(chan struct{}, 10)
		for i, e := range entries {
			wg2.Add(1)
			sem2 <- struct{}{}
			go func(idx int, en entry) {
				defer wg2.Done()
				defer func() { <-sem2 }()
				detailPath, pErr := buildOVHPath("ip", en.block, "reverse", en.ip)
				if pErr != nil {
					detailErrs[idx] = pErr
					return
				}
				var d map[string]interface{}
				client, err := ovh.IsolateClient(client)
				if err != nil {
					detailErrs[idx] = err
					return
				}
				if err := client.Get(detailPath, &d); err != nil {
					detailErrs[idx] = err
					return
				}
				// 同上:200 + null 也当失败记账,否则 failed 计数和被跳过的条目会对不上
				if d == nil {
					detailErrs[idx] = fmt.Errorf("OVH 返回了空的反向DNS记录: %s", en.ip)
					return
				}
				details[idx] = d
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

		// 统计两阶段的失败数:全军覆没(通常是 /ip/* 无授权或 OVH 限流)必须报错,
		// 否则前端会显示"没有配置任何反向 DNS",用户会以为记录被删了
		failed := 0
		for _, e := range blockErrs {
			if e != nil {
				failed++
			}
		}
		for _, e := range detailErrs {
			if e != nil {
				failed++
			}
		}
		total := len(blockErrs) + len(detailErrs)
		if total > 0 && failed == total {
			fe := hardwareFirstErr(blockErrs, detailErrs)
			state.Logger.Error("获取服务器 "+svc+" 反向DNS失败(全部 "+fmt.Sprintf("%d", total)+" 次 /ip 调用均失败): "+fe.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fe.Error()})
			return
		}
		resp := gin.H{"success": true, "reverses": reverseList}
		if failed > 0 {
			fe := hardwareFirstErr(blockErrs, detailErrs)
			state.Logger.Warn(fmt.Sprintf("服务器 %s 反向DNS 部分获取失败(%d/%d): %s", svc, failed, total, fe.Error()), "server_control")
			resp["partial"] = true
			resp["failed"] = failed
			resp["error"] = "部分反向DNS记录获取失败：" + fe.Error()
		}
		c.JSON(http.StatusOK, resp)
	}
}

// SetReverseDNS POST /api/server-control/:service_name/reverse
//
// 设反向 DNS 也走 /ip/{ipBlock}/reverse。需要先找到 body.IP 所属的 IP 块。
func SetReverseDNS(state *app.State) gin.HandlerFunc {
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
			IP      string `json:"ip"`
			Reverse string `json:"reverse"`
		}
		_ = c.ShouldBindJSON(&body)
		body.IP = strings.TrimSpace(body.IP)
		body.Reverse = strings.TrimSuffix(strings.TrimSpace(body.Reverse), ".")
		if body.IP == "" || body.Reverse == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "IP地址和反向DNS不能为空", "code": "E907CF2B8"})
			return
		}
		if _, ok := validateSingleIP(body.IP); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "非法 IP 地址格式", "code": "INVALID_IP"})
			return
		}
		// 找该 IP 所在的服务器 IP 块
		block, err := findIPBlockForServer(client, svc, body.IP)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		revPath, ok := ovhPathOrAbort(c, "ip", block, "reverse")
		if !ok {
			return
		}
		if err := client.Post(revPath, map[string]interface{}{
			"ipReverse": body.IP,
			"reverse":   body.Reverse,
		}, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("服务器 "+svc+" IP "+body.IP+" 反向DNS已设置为 "+body.Reverse, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "反向DNS已设置", "code": "E3BD7C8AF"})
	}
}

// DeleteReverseDNS DELETE /api/server-control/:service_name/reverse/:ip
// OVH 的 DELETE /ip/{ipBlock}/reverse/{ipReverse} 删除单条反向记录。
func DeleteReverseDNS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		ip, ok := validateSingleIP(c.Param("ip"))
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "非法 IP: " + c.Param("ip")})
			return
		}
		if rawBlock := strings.TrimSpace(c.Query("block")); rawBlock != "" {
			if _, ok := validateIPOrCIDR(rawBlock); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "非法 block 参数", "code": "INVALID_IP_BLOCK"})
				return
			}
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		block, err := findIPBlockForServer(client, svc, ip)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		delPath, ok := ovhPathOrAbort(c, "ip", block, "reverse", ip)
		if !ok {
			return
		}
		if err := client.Delete(delPath, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("服务器 "+svc+" IP "+ip+" 反向DNS已删除", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "反向DNS已删除", "code": "E480066AC"})
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
	ipsPath, err := buildOVHPath("dedicated", "server", svc, "ips")
	if err != nil {
		return "", err
	}
	var blocks []string
	if err := client.Get(ipsPath, &blocks); err != nil {
		return "", err
	}
	for _, blk := range blocks {
		// 补掩码只是为了能 ParseCIDR,补出来的字符串绝不能外传:
		// /ip/{ip} 的 path 参数是 ipBlock 资源 id,必须与 /dedicated/server/{svc}/ips
		// 返回的原样值一致,自作主张加 "/32" 会让后续 reverse 请求 404。
		parseable := blk
		_, ipnet, err := net.ParseCIDR(parseable)
		if err != nil {
			// 老格式可能没带 mask,补 /32 / /128 再 parse
			if strings.Contains(parseable, ":") {
				parseable += "/128"
			} else {
				parseable += "/32"
			}
			_, ipnet, err = net.ParseCIDR(parseable)
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
