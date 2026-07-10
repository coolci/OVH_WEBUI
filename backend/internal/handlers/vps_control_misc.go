package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

// ChangeVpsContact POST /api/vps-control/:service_name/change-contact
//
// EU only —— US OVHcloud 没有 NIC 联系人系统(独立公司,客户实体走 us.ovhcloud.com 自己的账户体系),
// 该端点 /vps/{name}/changeContact 在 US 不存在。这里提前拒,避免 OVH 报 404 让人摸不着头脑。
func ChangeVpsContact(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		acc, _ := ovhAccountFor(state, c)
		if acc.Endpoint == "ovh-us" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "美区 VPS 不支持「变更联系人」—— OVHcloud US 没有 NIC 联系人系统,过户需直接联系 OVH 客服",
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
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "至少需要指定一个联系人"})
			return
		}
		var taskIDs []int64
		if err := client.Post("/vps/"+svc+"/changeContact", params, &taskIDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("VPS %s 联系人变更已提交: %v, tasks=%v", svc, params, taskIDs), "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "联系人变更请求已提交", "taskIds": taskIDs})
	}
}

// TerminateVps POST /api/vps-control/:service_name/terminate
// 跟 dedicated 一致:OVH 返回 string(确认 token,通过邮件验证)
func TerminateVps(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var token string
		if err := client.Post("/vps/"+svc+"/terminate", map[string]interface{}{}, &token); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Warn("VPS "+svc+" 终止请求已提交,等邮件 token", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "终止请求已提交,请查邮件获取 token", "token": token})
	}
}

// ConfirmVpsTermination POST /api/vps-control/:service_name/confirm-termination
// /vps/{name}/confirmTermination 返回 string(确认消息)。body 至少需要 token + commentary 之一
func ConfirmVpsTermination(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Token      string `json:"token"`
			Reason     string `json:"reason"`
			Commentary string `json:"commentary"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 token"})
			return
		}
		params := map[string]interface{}{"token": body.Token}
		if body.Reason != "" {
			params["reason"] = body.Reason
		}
		if body.Commentary != "" {
			params["commentary"] = body.Commentary
		}
		var resp string
		if err := client.Post("/vps/"+svc+"/confirmTermination", params, &resp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Warn("VPS "+svc+" 终止已确认", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "终止已确认"})
	}
}

// GetVpsSecondaryDns GET /api/vps-control/:service_name/secondary-dns
// /vps/{name}/secondaryDnsDomains 返回 string[](域名数组)
func GetVpsSecondaryDns(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var domains []string
		if err := client.Get("/vps/"+svc+"/secondaryDnsDomains", &domains); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		details := parallelGetStringKeys(client, domains, func(d string) string {
			return "/vps/" + svc + "/secondaryDnsDomains/" + d
		}, 8)
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

// AddVpsSecondaryDns POST /api/vps-control/:service_name/secondary-dns
// body 需要 domain + ip(主 DNS IP)。OVH 返回 void
func AddVpsSecondaryDns(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Domain string `json:"domain"`
			IP     string `json:"ip"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Domain == "" || body.IP == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "domain 和 ip 必填"})
			return
		}
		if err := client.Post("/vps/"+svc+"/secondaryDnsDomains", map[string]interface{}{
			"domain": body.Domain,
			"ip":     body.IP,
		}, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("VPS "+svc+" 添加二级 DNS "+body.Domain, "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "二级 DNS 域名已添加"})
	}
}

// DeleteVpsSecondaryDns DELETE /api/vps-control/:service_name/secondary-dns/:domain
func DeleteVpsSecondaryDns(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		domain := c.Param("domain")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		if err := client.Delete("/vps/"+svc+"/secondaryDnsDomains/"+domain, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("VPS "+svc+" 删除二级 DNS "+domain, "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "二级 DNS 域名已删除"})
	}
}

// GetVpsOptions GET /api/vps-control/:service_name/options
// /vps/{name}/option 返回 vps.VpsOptionEnum[](string enum 数组),每个 /option/{name} 是 vps.Option 详情
func GetVpsOptions(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var opts []string
		if err := client.Get("/vps/"+svc+"/option", &opts); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		details := parallelGetStringKeys(client, opts, func(o string) string {
			return "/vps/" + svc + "/option/" + o
		}, 8)
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

// DeleteVpsOption DELETE /api/vps-control/:service_name/options/:option
func DeleteVpsOption(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		opt := c.Param("option")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		if err := client.Delete("/vps/"+svc+"/option/"+opt, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("VPS "+svc+" 取消附加选项 "+opt, "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "附加选项已取消"})
	}
}

// GetVpsAutomatedBackup GET /api/vps-control/:service_name/automated-backup
// 高端 VPS 才有,/vps/{name}/automatedBackup 返回 vps.AutomatedBackup 对象,无则 404
func GetVpsAutomatedBackup(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d map[string]interface{}
		if err := client.Get("/vps/"+svc+"/automatedBackup", &d); err != nil {
			// 没自动备份服务 → 200 + null
			c.JSON(http.StatusOK, gin.H{"success": true, "automatedBackup": nil})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "automatedBackup": d})
	}
}
