package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

// Secondary DNS
func GetSecondaryDNS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var domains []string
		if err := client.Get("/dedicated/server/"+svc+"/secondaryDnsDomains", &domains); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		details := parallelGetStringKeys(client, domains, func(d string) string {
			return "/dedicated/server/" + svc + "/secondaryDnsDomains/" + d
		}, 10)
		list := []interface{}{}
		for i, d := range domains {
			if details[i] == nil {
				list = append(list, map[string]interface{}{"domain": d})
				continue
			}
			details[i]["domain"] = d
			list = append(list, details[i])
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "domains": list})
	}
}

func AddSecondaryDNS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Domain string `json:"domain"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少domain参数"})
			return
		}
		if err := client.Post("/dedicated/server/"+svc+"/secondaryDnsDomains", map[string]interface{}{"domain": body.Domain}, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("添加从DNS域名 "+body.Domain+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "从DNS域名已添加"})
	}
}

func DeleteSecondaryDNS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		domain := c.Param("domain")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		if err := client.Delete("/dedicated/server/"+svc+"/secondaryDnsDomains/"+domain, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("删除从DNS域名 "+domain+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "从DNS域名已删除"})
	}
}

// Virtual MAC
func GetVirtualMACList(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var macs []string
		if err := client.Get("/dedicated/server/"+svc+"/virtualMac", &macs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		details := parallelGetStringKeys(client, macs, func(m string) string {
			return "/dedicated/server/" + svc + "/virtualMac/" + m
		}, 10)
		list := []interface{}{}
		for i, mac := range macs {
			if details[i] == nil {
				list = append(list, map[string]interface{}{"macAddress": mac})
				continue
			}
			details[i]["macAddress"] = mac
			list = append(list, details[i])
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "virtualMacs": list})
	}
}

func CreateVirtualMAC(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			IPAddress          string `json:"ipAddress"`
			Type               string `json:"type"`
			VirtualMachineName string `json:"virtualMachineName"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.IPAddress == "" || body.Type == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少必需参数"})
			return
		}
		var result map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/virtualMac", map[string]interface{}{
			"ipAddress":          body.IPAddress,
			"type":               body.Type,
			"virtualMachineName": body.VirtualMachineName,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("创建虚拟MAC成功: "+body.IPAddress, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "虚拟MAC已创建", "result": result})
	}
}

// Virtual Network Interface
func GetVirtualNetworkInterfaces(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var uuids []string
		if err := client.Get("/dedicated/server/"+svc+"/virtualNetworkInterface", &uuids); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		details := parallelGetStringKeys(client, uuids, func(u string) string {
			return "/dedicated/server/" + svc + "/virtualNetworkInterface/" + u
		}, 10)
		list := []interface{}{}
		for i, id := range uuids {
			if details[i] == nil {
				list = append(list, map[string]interface{}{"uuid": id})
				continue
			}
			details[i]["uuid"] = id
			list = append(list, details[i])
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "interfaces": list})
	}
}

func EnableVirtualNetworkInterface(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		id := c.Param("uuid")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		if err := client.Post("/dedicated/server/"+svc+"/virtualNetworkInterface/"+id+"/enable", map[string]interface{}{}, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("启用虚拟网络接口 "+id+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "虚拟网络接口已启用"})
	}
}

func DisableVirtualNetworkInterface(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		id := c.Param("uuid")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		if err := client.Post("/dedicated/server/"+svc+"/virtualNetworkInterface/"+id+"/disable", map[string]interface{}{}, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("禁用虚拟网络接口 "+id+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "虚拟网络接口已禁用"})
	}
}

// vRack
func GetVRackList(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var vracks []string
		if err := client.Get("/dedicated/server/"+svc+"/vrack", &vracks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		details := parallelGetStringKeys(client, vracks, func(v string) string {
			return "/dedicated/server/" + svc + "/vrack/" + v
		}, 10)
		list := []interface{}{}
		for i, v := range vracks {
			if details[i] == nil {
				list = append(list, map[string]interface{}{"vrackName": v})
				continue
			}
			details[i]["vrackName"] = v
			list = append(list, details[i])
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "vracks": list})
	}
}

func RemoveFromVRack(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		vrack := c.Param("vrack")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		if err := client.Delete("/dedicated/server/"+svc+"/vrack/"+vrack, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("从vRack "+vrack+" 移除服务器成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "服务器已从vRack移除"})
	}
}

// Orderable
func GetOrderableBandwidth(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d interface{}
		if err := client.Get("/dedicated/server/"+svc+"/orderable/bandwidth", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "orderable": d})
	}
}

func GetOrderableTraffic(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d interface{}
		if err := client.Get("/dedicated/server/"+svc+"/orderable/traffic", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "orderable": d})
	}
}

func GetOrderableIP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d interface{}
		if err := client.Get("/dedicated/server/"+svc+"/orderable/ip", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "orderable": d})
	}
}

// Options
func GetServerOptions(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var opts []string
		if err := client.Get("/dedicated/server/"+svc+"/option", &opts); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		details := parallelGetStringKeys(client, opts, func(o string) string {
			return "/dedicated/server/" + svc + "/option/" + o
		}, 10)
		list := []interface{}{}
		for i, opt := range opts {
			if details[i] == nil {
				list = append(list, map[string]interface{}{"option": opt})
				continue
			}
			details[i]["option"] = opt
			list = append(list, details[i])
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "options": list})
	}
}

// IP specs
func GetIPSpecs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d interface{}
		if err := client.Get("/dedicated/server/"+svc+"/specifications/ip", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ipSpecs": d})
	}
}

func GetIPCanBeMovedTo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d interface{}
		if err := client.Get("/dedicated/server/"+svc+"/ipCanBeMovedTo", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "targets": d})
	}
}

func GetIPCountryAvailable(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d interface{}
		if err := client.Get("/dedicated/server/"+svc+"/ipCountryAvailable", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "countries": d})
	}
}

func MoveIP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			IP string `json:"ip"`
			To string `json:"to"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.IP == "" || body.To == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少必需参数"})
			return
		}
		var result map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/ipMove", map[string]interface{}{
			"ip": body.IP,
			"to": body.To,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("IP迁移任务已创建: "+body.IP+" -> "+body.To, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "IP迁移任务已创建", "result": result})
	}
}

// Ongoing
func GetOngoingTasks(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d interface{}
		if err := client.Get("/dedicated/server/"+svc+"/ongoing", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ongoing": d})
	}
}

// License
func GetCompliantWindowsVersions(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d interface{}
		if err := client.Get("/dedicated/server/"+svc+"/license/compliantWindows", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "versions": d})
	}
}

func GetCompliantWindowsSqlVersions(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d interface{}
		if err := client.Get("/dedicated/server/"+svc+"/license/compliantWindowsSqlServer", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "versions": d})
	}
}

// Termination
func TerminateService(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		// OVH /terminate 返回 string(确认 token)而不是对象
		var token string
		if err := client.Post("/dedicated/server/"+svc+"/terminate", map[string]interface{}{}, &token); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Warn("服务器 "+svc+" 终止请求已提交, 邮件 token=...", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "终止请求已提交,请查邮件获取 token", "token": token})
	}
}

func ConfirmTermination(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少token参数"})
			return
		}
		// OVH /confirmTermination 返回 string(确认信息)
		var resp string
		if err := client.Post("/dedicated/server/"+svc+"/confirmTermination", map[string]interface{}{
			"token": body.Token,
		}, &resp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Warn("服务器 "+svc+" 终止已确认", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "终止已确认"})
	}
}

// SPLA
func GetSPLAList(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/dedicated/server/"+svc+"/spla", &ids); err != nil {
			state.Logger.Error("获取SPLA列表失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		details := parallelGetDetails(client, ids, func(k interface{}) string {
			return "/dedicated/server/" + svc + "/spla/" + idToString(k)
		}, 10)
		list := []interface{}{}
		for i, id := range ids {
			if details[i] == nil {
				list = append(list, map[string]interface{}{"id": id})
				continue
			}
			details[i]["id"] = id
			list = append(list, details[i])
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "splaList": list})
	}
}

func CreateSPLA(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Type         string `json:"type"`
			SerialNumber string `json:"serialNumber"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Type == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少type参数"})
			return
		}
		// 1:1 对应 Python app.py:8296-8300：data.get('serialNumber') 缺失返回 None，
		// 序列化为 JSON null；之前 Go 用 struct 默认 "" 会让 OVH 拒空字符串
		payload := map[string]interface{}{"type": body.Type}
		if body.SerialNumber != "" {
			payload["serialNumber"] = body.SerialNumber
		} else {
			payload["serialNumber"] = nil
		}
		// OVH /spla 返回 long(新建 SPLA 的 ID),不是对象
		var newID int64
		if err := client.Post("/dedicated/server/"+svc+"/spla", payload, &newID); err != nil {
			state.Logger.Error("创建SPLA许可证失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("创建SPLA许可证成功: %s, id=%d", body.Type, newID), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "SPLA许可证已创建", "id": newID})
	}
}

// BIOS
func GetBIOSSettings(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		state.Logger.Info("[BIOS] 获取服务器 "+svc+" BIOS 设置", "server_control")
		var d map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/biosSettings", &d); err != nil {
			msg := err.Error()
			if containsAny(msg, []string{"does not exist", "object", "404", "not found"}) {
				state.Logger.Warn("[BIOS] 服务器 "+svc+" 不支持 BIOS 设置: "+msg, "server_control")
				c.JSON(http.StatusOK, gin.H{
					"success":      false,
					"notAvailable": true,
					"error":        "此服务器不支持 BIOS 远程设置",
				})
				return
			}
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": msg})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "bios": d})
	}
}

func GetBIOSSettingsSGX(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		state.Logger.Info("[BIOS] 获取服务器 "+svc+" SGX BIOS 设置", "server_control")
		var d map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/biosSettings/sgx", &d); err != nil {
			msg := err.Error()
			if containsAny(msg, []string{"does not exist", "object"}) {
				state.Logger.Warn("[BIOS] 服务器 "+svc+" 不支持 SGX: "+msg, "server_control")
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "SGX 不可用"})
				return
			}
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": msg})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "sgx": d})
	}
}

func intToStr(v int64) string {
	return formatInt(v)
}

func formatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func containsAny(s string, subs []string) bool {
	lc := lower(s)
	for _, sub := range subs {
		if indexOf(lc, lower(sub)) >= 0 {
			return true
		}
	}
	return false
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
