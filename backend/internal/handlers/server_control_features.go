package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

// Burst GET
func GetBurst(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var burst map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/burst", &burst); err != nil {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "does not exist") || strings.Contains(lower, "not exist") {
				state.Logger.Info("服务器 "+svc+" 不支持突发带宽功能", "server_control")
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "该服务器不支持突发带宽功能", "notAvailable": true})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "burst": burst})
	}
}

// Burst PUT
func UpdateBurst(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Status == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少status参数"})
			return
		}
		var result map[string]interface{}
		if err := client.Put("/dedicated/server/"+svc+"/burst", map[string]interface{}{
			"status": body.Status,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("更新服务器 "+svc+" 突发带宽状态为: "+body.Status, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "突发带宽状态已更新", "result": result})
	}
}

// Firewall GET
func GetFirewall(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var fw map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/features/firewall", &fw); err != nil {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "does not exist") || strings.Contains(lower, "not exist") {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "该服务器不支持防火墙功能", "notAvailable": true})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "firewall": fw})
	}
}

// Firewall PUT
func UpdateFirewall(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Enabled == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少enabled参数"})
			return
		}
		var result map[string]interface{}
		if err := client.Put("/dedicated/server/"+svc+"/features/firewall", map[string]interface{}{
			"enabled": *body.Enabled,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		text := "启用"
		if !*body.Enabled {
			text = "禁用"
		}
		state.Logger.Info(text+"服务器 "+svc+" 防火墙", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "防火墙已" + text, "result": result})
	}
}

// Backup FTP GET
func GetBackupFTP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/features/backupFTP", &d); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "备份FTP未激活", "notActivated": true})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "backupFtp": d})
	}
}

// Backup FTP POST (activate)
func ActivateBackupFTP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var result map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/features/backupFTP", map[string]interface{}{}, &result); err != nil {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "cannot benefit") || strings.Contains(lower, "not available") {
				c.JSON(http.StatusBadRequest, gin.H{
					"success":      false,
					"error":        "该服务器无法使用备份FTP服务",
					"notAvailable": true,
					"reason":       err.Error(),
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("激活服务器 "+svc+" 备份FTP成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "备份FTP已激活", "result": result})
	}
}

// Backup FTP DELETE
func DeleteBackupFTP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var result map[string]interface{}
		if err := client.Delete("/dedicated/server/"+svc+"/features/backupFTP", &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("删除服务器 "+svc+" 备份FTP成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "备份FTP已删除", "result": result})
	}
}

// Backup FTP access list
func GetBackupFTPAccess(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var blocks []string
		if err := client.Get("/dedicated/server/"+svc+"/features/backupFTP/access", &blocks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 并发拉每个 IP block 的详情
		details := parallelGetStringKeys(client, blocks, func(b string) string {
			return "/dedicated/server/" + svc + "/features/backupFTP/access/" + b
		}, 10)
		list := []interface{}{}
		for i, b := range blocks {
			if details[i] == nil {
				list = append(list, map[string]interface{}{"ipBlock": b, "error": "fetch failed"})
				continue
			}
			list = append(list, details[i])
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "accessList": list})
	}
}

// Backup FTP access add
func AddBackupFTPAccess(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			IPBlock string `json:"ipBlock"`
			FTP     *bool  `json:"ftp"`
			NFS     bool   `json:"nfs"`
			CIFS    bool   `json:"cifs"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.IPBlock == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少ipBlock参数"})
			return
		}
		ftp := true
		if body.FTP != nil {
			ftp = *body.FTP
		}
		var result map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/features/backupFTP/access", map[string]interface{}{
			"cifs":    body.CIFS,
			"ftp":     ftp,
			"ipBlock": body.IPBlock,
			"nfs":     body.NFS,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("添加备份FTP访问IP "+body.IPBlock+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "访问IP已添加", "result": result})
	}
}

// Backup FTP access delete
func DeleteBackupFTPAccess(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		ipBlock := c.Param("ip_block")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		if err := client.Delete("/dedicated/server/"+svc+"/features/backupFTP/access/"+ipBlock, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("删除备份FTP访问IP "+ipBlock+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "访问IP已删除"})
	}
}

// Backup FTP password
func ChangeBackupFTPPassword(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var result map[string]interface{}
		if err := client.Post("/dedicated/server/"+svc+"/features/backupFTP/password", map[string]interface{}{}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("修改服务器 "+svc+" 备份FTP密码成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "密码已重置，新密码已发送至邮箱", "result": result})
	}
}

// Backup FTP authorizable blocks
func GetBackupFTPAuthorizableBlocks(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var blocks []string
		if err := client.Get("/dedicated/server/"+svc+"/features/backupFTP/authorizableBlocks", &blocks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "blocks": blocks})
	}
}

// Backup Cloud GET
func GetBackupCloud(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/features/backupCloud", &d); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "云备份未激活", "notActivated": true})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "backupCloud": d})
	}
}

// Backup Cloud offer details
func GetBackupCloudOfferDetails(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc+"/backupCloudOfferDetails", &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "offerDetails": d})
	}
}
