package handlers

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
)

// ListVps GET /api/vps-control/list
//
// OVH /vps 返回 string[](serviceName 列表)。每个 VPS 并发拉 /vps/{name} 详情 +
// /vps/{name}/status 状态(running / stopped / migrating / ...)
func ListVps(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var names []string
		if err := client.Get("/vps", &names); err != nil {
			state.Logger.Error("获取 VPS 列表失败: "+err.Error(), "vps_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("获取 VPS 列表成功", "vps_control")

		type vpsResult struct {
			info  map[string]interface{}
			svc   map[string]interface{}
			err   error
		}
		results := make([]vpsResult, len(names))
		sem := make(chan struct{}, 10)
		var wg sync.WaitGroup
		for i, name := range names {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, nm string) {
				defer wg.Done()
				defer func() { <-sem }()
				var info map[string]interface{}
				if err := client.Get("/vps/"+nm, &info); err != nil {
					results[idx].err = err
					return
				}
				results[idx].info = info
				var svcInfo map[string]interface{}
				_ = client.Get("/vps/"+nm+"/serviceInfos", &svcInfo)
				results[idx].svc = svcInfo
			}(i, name)
		}
		wg.Wait()

		list := []gin.H{}
		for i, name := range names {
			r := results[i]
			if r.err != nil || r.info == nil {
				list = append(list, gin.H{"name": name, "serviceName": name, "error": "fetch failed"})
				continue
			}
			info := r.info
			renewalType := false
			if r.svc != nil {
				if rn, ok := r.svc["renew"].(map[string]interface{}); ok {
					if a, ok := rn["automatic"].(bool); ok {
						renewalType = a
					}
				}
			}
			// vps.Model 字段嵌套在 info["model"] 里
			vcore := 0
			memMB := 0
			diskGB := 0
			modelName := ""
			if model, ok := info["model"].(map[string]interface{}); ok {
				if v, ok := numconv.ToInt64(model["vcore"]); ok {
					vcore = int(v)
				}
				if v, ok := numconv.ToInt64(model["memory"]); ok {
					memMB = int(v)
				}
				if v, ok := numconv.ToInt64(model["disk"]); ok {
					diskGB = int(v)
				}
				modelName, _ = model["name"].(string)
			}
			// vps.LockStatus 是对象 { locked: bool, reason: enum } —— 不能直接给前端渲染,
			// 拍平成字符串。OVH 实测 reason 目前只有 "abuse",未锁定时 locked=false。
			lockStr := "unlocked"
			if ls, ok := info["lockStatus"].(map[string]interface{}); ok {
				if locked, _ := ls["locked"].(bool); locked {
					reason, _ := ls["reason"].(string)
					if reason != "" {
						lockStr = "locked (" + reason + ")"
					} else {
						lockStr = "locked"
					}
				}
			}
			list = append(list, gin.H{
				"serviceName":   name,
				"name":          name,
				"displayName":   valueOr(info, "displayName", name),
				"state":         valueOr(info, "state", "unknown"),
				"cluster":       valueOr(info, "cluster", ""),
				"zone":          valueOr(info, "zone", ""),
				"keymap":        valueOr(info, "keymap", "us"),
				"netbootMode":   valueOr(info, "netbootMode", "local"),
				"offerType":     valueOr(info, "offerType", ""),
				"slaMonitoring": info["slaMonitoring"], // 布尔字段不能走 valueOr,直接透传(nil 也 OK)
				"lockStatus":    lockStr,
				"model":         modelName,
				"vcore":         vcore,
				"memoryMB":      memMB,
				"diskGB":        diskGB,
				"status":        valueOr(r.svc, "status", "unknown"),
				"renewalType":   renewalType,
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "vps": list, "total": len(list)})
	}
}

// GetVpsInfo GET /api/vps-control/:service_name/info
// 返回 VPS 详细信息(model + state + zone 等)
func GetVpsInfo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var info map[string]interface{}
		if err := client.Get("/vps/"+svc, &info); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "info": info})
	}
}

// GetVpsServiceStatus GET /api/vps-control/:service_name/status
//
// OVH /vps/{name}/status 返回 vps.ip.ServiceStatus(ping/dns/http/https/smtp/ssh/tools 服务端口探测),
// 跟 /vps/{name}.state(running/stopped/...)是两码事 —— 前者是网络服务存活,后者是 VPS 自身状态。
func GetVpsServiceStatus(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var status map[string]interface{}
		if err := client.Get("/vps/"+svc+"/status", &status); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "status": status})
	}
}

// GetVpsServiceInfo GET /api/vps-control/:service_name/serviceinfo
// 跟 dedicated 一致:返回 renew + expiration + creation,字段名对齐已有的 RenewalDialog
func GetVpsServiceInfo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var info map[string]interface{}
		if err := client.Get("/vps/"+svc+"/serviceInfos", &info); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		renew, _ := info["renew"].(map[string]interface{})
		automatic, period, delAtExp, forced, manualPay := false, 0, false, false, false
		if renew != nil {
			if v, ok := renew["automatic"].(bool); ok {
				automatic = v
			}
			if v, ok := numconv.ToInt64(renew["period"]); ok {
				period = int(v)
			}
			if v, ok := renew["deleteAtExpiration"].(bool); ok {
				delAtExp = v
			}
			if v, ok := renew["forced"].(bool); ok {
				forced = v
			}
			if v, ok := renew["manualPayment"].(bool); ok {
				manualPay = v
			}
		}
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
				"renewalType":               automatic,
				"renewalPeriod":             period,
				"renewalDeleteAtExpiration": delAtExp,
				"renewalForced":             forced,
				"renewalManualPayment":      manualPay,
				"possibleRenewPeriod":       possiblePeriods,
			},
		})
	}
}

// UpdateVpsRenewal PUT /api/vps-control/:service_name/serviceinfo/renewal
// 同 dedicated 的 UpdateServiceRenewal 逻辑:GET → merge renew → PUT
func UpdateVpsRenewal(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Mode   string `json:"mode"`
			Period int    `json:"period"`
		}
		_ = c.ShouldBindJSON(&body)

		var info map[string]interface{}
		if err := client.Get("/vps/"+svc+"/serviceInfos", &info); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		renew, _ := info["renew"].(map[string]interface{})
		if renew == nil {
			renew = map[string]interface{}{}
		}
		if f, ok := renew["forced"].(bool); ok && f {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "该 VPS 处于 OVH 合同期内,续费策略由 OVH 锁定"})
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
		if err := client.Put("/vps/"+svc+"/serviceInfos", info, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("VPS "+svc+" 续费策略已更新: "+body.Mode, "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "续费策略已更新"})
	}
}

// GetVpsIps GET /api/vps-control/:service_name/ips
// /vps/{name}/ips 返回 ip[](IP 字符串数组),为每个 IP 并发拉详情
func GetVpsIps(state *app.State) gin.HandlerFunc {
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
		details := parallelGetStringKeys(client, ips, func(ip string) string {
			return "/vps/" + svc + "/ips/" + ip
		}, 8)
		list := []gin.H{}
		for i, ip := range ips {
			d := details[i]
			if d == nil {
				list = append(list, gin.H{"ipAddress": ip})
				continue
			}
			list = append(list, gin.H{
				"ipAddress":   valueOr(d, "ipAddress", ip),
				"reverse":     valueOr(d, "reverse", ""),
				"type":        valueOr(d, "type", ""),
				"version":     valueOr(d, "version", ""),
				"gateway":     valueOr(d, "gateway", ""),
				"geolocation": valueOr(d, "geolocation", ""),
				"macAddress":  valueOr(d, "macAddress", ""),
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ips": list, "total": len(list)})
	}
}

// SetVpsIpReverse PUT /api/vps-control/:service_name/ips/:ip/reverse
//
// OVH PUT /vps/{name}/ips/{ipAddress} 期望完整 vps.Ip 对象。read-modify-write:
// 先 GET 拿当前 ip 详情,改 reverse 字段,再整体 PUT 回去 —— 防止 OVH 把其他字段当 null 重置。
func SetVpsIpReverse(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		ip := c.Param("ip")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Reverse string `json:"reverse"`
		}
		_ = c.ShouldBindJSON(&body)
		var current map[string]interface{}
		if err := client.Get("/vps/"+svc+"/ips/"+ip, &current); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		current["reverse"] = body.Reverse
		if err := client.Put("/vps/"+svc+"/ips/"+ip, current, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("VPS "+svc+" IP "+ip+" 反向 DNS 设为 "+body.Reverse, "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "反向 DNS 已更新"})
	}
}

// GetVpsDatacenter GET /api/vps-control/:service_name/datacenter
func GetVpsDatacenter(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var dc map[string]interface{}
		if err := client.Get("/vps/"+svc+"/datacenter", &dc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "datacenter": dc})
	}
}

// VPS CPU/内存监控两个端点 OVH 已废弃,不再实现:
//   /vps/{name}/monitoring  - DEPRECATED 2024-07-15(deletionDate 2024-09-15)
//   /vps/{name}/statistics  - DEPRECATED 2023-11-07(deletionDate 2024-01-07)
// 实测 US OVH 返 500 Internal Server Error。OVH 没提供替代的 VPS 级监控端点
// (只剩 /disks/{id}/use 磁盘级),所以前端干脆移除监控视图。看负载请登 VPS top。
