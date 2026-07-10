package handlers

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

// isIPv4 简单判 IPv4 地址(含点号、不含冒号)。OVH 的 anti-DDoS mitigation 只支持 IPv4,
// IPv6 传过去会 400 "[ipOnMitigation] Given data is not valid for type ipv4"
func isIPv4(s string) bool {
	return strings.Contains(s, ".") && !strings.Contains(s, ":")
}

// GetVpsMitigation GET /api/vps-control/:service_name/mitigation
//
// 跟 dedicated 的 GetMitigation 逻辑一样,只是 IP 列表来源不同:
//   - dedicated: /dedicated/server/{svc}/ips     (返回 ipBlock[],带 mask)
//   - vps:       /vps/{svc}/ips                  (返回 ip[],单 IP)
//
// 单 IP 拿来查 mitigation 时 OVH 接 /ip/{ip}/mitigation 也认(自动当 /32 处理)
func GetVpsMitigation(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var ips []string
		if err := client.Get("/vps/"+svc+"/ips", &ips); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		type ipResult struct {
			ip          string
			mitigations []map[string]interface{}
			err         error
		}
		results := make([]ipResult, len(ips))
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		for i, ip := range ips {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, ipAddr string) {
				defer wg.Done()
				defer func() { <-sem }()
				encoded := strings.ReplaceAll(ipAddr, "/", "%2F")
				var miti []string
				if err := client.Get("/ip/"+encoded+"/mitigation", &miti); err != nil {
					results[idx] = ipResult{ip: ipAddr, err: err}
					return
				}
				details := make([]map[string]interface{}, 0, len(miti))
				for _, m := range miti {
					var d map[string]interface{}
					if err := client.Get("/ip/"+encoded+"/mitigation/"+m, &d); err == nil {
						details = append(details, d)
					}
				}
				results[idx] = ipResult{ip: ipAddr, mitigations: details}
			}(i, ip)
		}
		wg.Wait()

		list := []gin.H{}
		for _, r := range results {
			row := gin.H{"ipBlock": r.ip, "mitigations": r.mitigations}
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

// EnableVpsMitigation POST /api/vps-control/:service_name/mitigation/:ip?block=xxx
func EnableVpsMitigation(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.Param("ip")
		ipBlock := c.Query("block")
		if ipBlock == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 block 参数"})
			return
		}
		if !isIPv4(ip) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "OVH anti-DDoS Mitigation 只支持 IPv4。IPv6 地址在 OVH 网络层默认免疫常见 volumetric 攻击,无需手动配置",
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
		state.Logger.Info("VPS IP "+ip+" 启用永久 DDoS 缓解", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "DDoS 缓解已启用", "mitigation": result})
	}
}

// DisableVpsMitigation DELETE /api/vps-control/:service_name/mitigation/:ip?block=xxx
func DisableVpsMitigation(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.Param("ip")
		ipBlock := c.Query("block")
		if ipBlock == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 block 参数"})
			return
		}
		if !isIPv4(ip) {
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
		state.Logger.Info("VPS IP "+ip+" 关闭永久 DDoS 缓解", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "DDoS 缓解已关闭"})
	}
}
