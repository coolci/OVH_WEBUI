package handlers

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

// GetMitigation GET /api/server-control/:service_name/mitigation
//
// 列服务器所有 IP 的 DDoS 缓解状态。
// OVH 的 /ip/{ip}/mitigation 端点要求 {ip} 是 IP 块(/32 用 %2F 转义),
// 但是从 /dedicated/server/{svc}/ips 拿到的就是 IP 块格式,直接拼。
//
// 返回结构:
//   ips: [{ ipBlock, mitigations: [{ ipOnMitigation, state, auto, permanent }] }]
func GetMitigation(state *app.State) gin.HandlerFunc {
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
		type ipResult struct {
			block       string
			mitigations []map[string]interface{}
			err         error
		}
		results := make([]ipResult, len(ipBlocks))
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		for i, blk := range ipBlocks {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, ipBlock string) {
				defer wg.Done()
				defer func() { <-sem }()
				encoded := strings.ReplaceAll(ipBlock, "/", "%2F")
				// 1) 列出该 block 下处于 mitigation 的具体 IP
				var ips []string
				if err := client.Get("/ip/"+encoded+"/mitigation", &ips); err != nil {
					results[idx] = ipResult{block: ipBlock, err: err}
					return
				}
				// 2) 并发拉每个 IP 的详情
				details := make([]map[string]interface{}, 0, len(ips))
				for _, ip := range ips {
					var d map[string]interface{}
					if err := client.Get("/ip/"+encoded+"/mitigation/"+ip, &d); err == nil {
						details = append(details, d)
					}
				}
				results[idx] = ipResult{block: ipBlock, mitigations: details}
			}(i, blk)
		}
		wg.Wait()

		list := []gin.H{}
		for _, r := range results {
			row := gin.H{"ipBlock": r.block, "mitigations": r.mitigations}
			if r.err != nil {
				row["error"] = r.err.Error()
			}
			if r.mitigations == nil {
				row["mitigations"] = []interface{}{}
			}
			list = append(list, row)
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ips": list})
	}
}

// EnableMitigation POST /api/server-control/:service_name/mitigation/:ip
// 对指定 IP 开 permanent mitigation。注意 :ip 参数是单个 IPv4,所属 block 用 query ?block=xxx
func EnableMitigation(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.Param("ip")
		ipBlock := c.Query("block")
		if ipBlock == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 block 参数(IP 所属的 ipBlock)"})
			return
		}
		// OVH ipOnMitigation 字段是 ipv4 类型,IPv6 走过去会 400
		if !strings.Contains(ip, ".") || strings.Contains(ip, ":") {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "OVH anti-DDoS Mitigation 只支持 IPv4。IPv6 地址默认有网络层免疫",
			})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		encoded := strings.ReplaceAll(ipBlock, "/", "%2F")
		var result map[string]interface{}
		if err := client.Post("/ip/"+encoded+"/mitigation",
			map[string]interface{}{"ipOnMitigation": ip}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("启用 IP "+ip+" 的永久 DDoS 缓解", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "DDoS 缓解已启用", "mitigation": result})
	}
}

// DisableMitigation DELETE /api/server-control/:service_name/mitigation/:ip?block=...
// 关闭指定 IP 的 permanent mitigation。auto mitigation 在攻击时仍会自动启用。
func DisableMitigation(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.Param("ip")
		ipBlock := c.Query("block")
		if ipBlock == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 block 参数"})
			return
		}
		if !strings.Contains(ip, ".") || strings.Contains(ip, ":") {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "IPv6 不支持 anti-DDoS Mitigation"})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		encoded := strings.ReplaceAll(ipBlock, "/", "%2F")
		if err := client.Delete("/ip/"+encoded+"/mitigation/"+ip, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("关闭 IP "+ip+" 的永久 DDoS 缓解", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "DDoS 缓解已关闭"})
	}
}
