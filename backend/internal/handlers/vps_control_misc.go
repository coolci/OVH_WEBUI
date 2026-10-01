package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// ChangeVpsContact POST /api/vps-control/:service_name/change-contact
func ChangeVpsContact(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			ContactAdmin   string `json:"contactAdmin"`
			ContactBilling string `json:"contactBilling"`
			ContactTech    string `json:"contactTech"`
		}
		_ = c.ShouldBindJSON(&body)
		params := map[string]interface{}{}
		if body.ContactAdmin != "" {
			params["contactAdmin"] = body.ContactAdmin
		}
		if body.ContactBilling != "" {
			params["contactBilling"] = body.ContactBilling
		}
		if body.ContactTech != "" {
			params["contactTech"] = body.ContactTech
		}
		if len(params) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "至少提供一个联系人字段"})
			return
		}
		var taskIDs []int64
		if err := client.Post("/vps/"+svc+"/changeContact", params, &taskIDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info(fmt.Sprintf("VPS %s 联系人变更已提交: %v, tasks=%v", svc, params, taskIDs), "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "联系人变更已提交", "taskIds": taskIDs})
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
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Warn("VPS "+svc+" 终止请求已提交,等邮件 token", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "终止请求已提交,请查邮件获取 token", "token": token})
	}
}

// ConfirmVpsTermination POST /api/vps-control/:service_name/confirm-termination
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
			Commentary string `json:"commentary"`
			Reason     string `json:"reason"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少确认 token"})
			return
		}
		params := map[string]interface{}{"token": body.Token}
		if body.Commentary != "" {
			params["commentary"] = body.Commentary
		}
		if body.Reason != "" {
			params["reason"] = body.Reason
		}
		var resp string
		if err := client.Post("/vps/"+svc+"/confirmTermination", params, &resp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Warn("VPS "+svc+" 终止已确认", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "VPS 终止已确认", "response": resp})
	}
}

// GetVpsSecondaryDns GET /api/vps-control/:service_name/secondary-dns
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
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		details := parallelGetStringKeys(client, domains, func(d string) string {
			return "/vps/" + svc + "/secondaryDnsDomains/" + d
		}, 8)
		list := []map[string]interface{}{}
		for i, d := range domains {
			if details[i] != nil {
				list = append(list, details[i])
			} else {
				list = append(list, map[string]interface{}{"domain": d})
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "secondaryDns": list})
	}
}

// AddVpsSecondaryDns POST /api/vps-control/:service_name/secondary-dns
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
		if body.Domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少域名"})
			return
		}
		params := map[string]interface{}{"domain": body.Domain}
		if body.IP != "" {
			params["ip"] = body.IP
		}
		if err := client.Post("/vps/"+svc+"/secondaryDnsDomains", params, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("VPS "+svc+" 添加二级 DNS "+body.Domain, "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "二级 DNS 添加成功"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("VPS "+svc+" 删除二级 DNS "+domain, "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "二级 DNS 删除成功"})
	}
}

// GetVpsOptions GET /api/vps-control/:service_name/options
func GetVpsOptions(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		// 美区 OVHcloud 不提供附加选项接口(GET/DELETE /vps/{sn}/option)。
		// 原注释只提到 DELETE 没有,实际 GET 也不在美区 schema 里。
		isUS := vpsRegionFor(state, c) == vpsRegionUS
		var opts []string
		if err := client.Get("/vps/"+svc+"/option", &opts); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		details := parallelGetStringKeys(client, opts, func(o string) string {
			return "/vps/" + svc + "/option/" + o
		}, 8)
		list := []gin.H{}
		for i, o := range opts {
			m := gin.H{"option": o}
			if details[i] != nil {
				for k, v := range details[i] {
					m[k] = v
				}
			}
			if isUS {
				m["manageEndpointsAvailable"] = false
				m["unsupportedReason"] = "美区 OVHcloud 未提供 VPS 附加选项接口"
			}
			list = append(list, m)
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "options": list})
	}
}

// DeleteVpsOption DELETE /api/vps-control/:service_name/options/:option[?deleteNow=true]
//
// ⚠️ DELETE /vps/{sn}/option/{option} 已从 EU/US/CA 三区 schema 里**整个消失**
// (2026-10 复查;此前标 DEPRECATED、deletionDate 2024-06-01,如今成真)——
// 按约定不再调用。路由保留给旧前端,固定回"已移除 + 指路 OVH 控制台"。
// 列表/详情(GET /option、GET /option/{option})还活着,不受影响。
func DeleteVpsOption(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusGone, gin.H{
			"success": false,
			"error":   "OVH 已移除「取消 VPS 附加选项」接口(2026-10 从 API 下线,无替代)。请到 OVH 控制台(My services → 该 VPS → 选项)取消",
		})
	}
}

// GetVpsAutomatedBackup GET /api/vps-control/:service_name/automated-backup
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
			// 未开通自动备份时 OVH 返回 404
			if ovhIsNotFound(err) {
				c.JSON(http.StatusOK, gin.H{"success": true, "automatedBackup": nil, "notSubscribed": true})
				return
			}
			state.Logger.Error("VPS "+svc+" 查询自动备份失败: "+err.Error(), "vps_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "automatedBackup": d})
	}
}
