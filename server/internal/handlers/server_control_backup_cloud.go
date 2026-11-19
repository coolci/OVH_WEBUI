package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// Backup Cloud GET
func GetBackupCloud(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupCloud")
		if !ok {
			return
		}
		var d map[string]interface{}
		if err := client.Get(p, &d); err != nil {
			code, msg := featureOVHErr(err)
			if code == http.StatusNotFound || strings.Contains(strings.ToLower(msg), "does not exist") {
				if featureIsFeatureMissing(client, svc) {
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "云备份未激活", "code": "E8120431F", "notActivated": true})
					return
				}
				state.Logger.Warn("查询服务器 "+svc+" 云备份：服务器不存在或不属于当前账户 - "+msg, "server_control")
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error":   "服务器不存在或不属于当前账户", "code": "E834268F5",
					"unknownService": true,
					"reason":         msg,
				})
				return
			}
			state.Logger.Error("查询服务器 "+svc+" 云备份失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "backupCloud": d})
	}
}

// Backup Cloud POST (activate)
func ActivateBackupCloud(state *app.State) gin.HandlerFunc {
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
			CloudProjectID     string `json:"cloudProjectId"`
			ProjectDescription string `json:"projectDescription"`
		}
		_ = c.ShouldBindJSON(&body)
		payload := map[string]interface{}{}
		if v := strings.TrimSpace(body.CloudProjectID); v != "" {
			payload["cloudProjectId"] = v
		}
		if v := strings.TrimSpace(body.ProjectDescription); v != "" {
			payload["projectDescription"] = v
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupCloud")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(p, payload, &result); err != nil {
			status := http.StatusInternalServerError
			if code, _ := featureOVHErr(err); code == http.StatusBadRequest || code == http.StatusConflict {
				status = code
			}
			state.Logger.Error("激活服务器 "+svc+" 云备份失败: "+err.Error(), "server_control")
			c.JSON(status, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("激活服务器 "+svc+" 云备份成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "云备份已激活", "code": "E56282980", "result": result})
	}
}

// Backup Cloud DELETE
func DeleteBackupCloud(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupCloud")
		if !ok {
			return
		}
		if err := client.Delete(p, nil); err != nil {
			code, msg := featureOVHErr(err)
			if code == http.StatusNotFound || strings.Contains(strings.ToLower(msg), "does not exist") {
				if featureIsFeatureMissing(client, svc) {
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "云备份未激活", "code": "E8120431F", "notActivated": true})
					return
				}
				state.Logger.Warn("停用服务器 "+svc+" 云备份：服务器不存在或不属于当前账户 - "+msg, "server_control")
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error":   "服务器不存在或不属于当前账户", "code": "E834268F5",
					"unknownService": true,
					"reason":         msg,
				})
				return
			}
			state.Logger.Error("停用服务器 "+svc+" 云备份失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		state.Logger.Info("停用服务器 "+svc+" 云备份成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "云备份已停用（容器内数据不会被删除）", "code": "E32472475"})
	}
}

// Backup Cloud password
func ChangeBackupCloudPassword(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupCloud", "password")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(p, map[string]interface{}{}, &result); err != nil {
			code, msg := featureOVHErr(err)
			if code == http.StatusNotFound || strings.Contains(strings.ToLower(msg), "does not exist") {
				if featureIsFeatureMissing(client, svc) {
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "云备份未激活", "code": "E8120431F", "notActivated": true})
					return
				}
				state.Logger.Warn("重置服务器 "+svc+" 云备份密码：服务器不存在或不属于当前账户 - "+msg, "server_control")
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error":   "服务器不存在或不属于当前账户", "code": "E834268F5",
					"unknownService": true,
					"reason":         msg,
				})
				return
			}
			state.Logger.Error("重置服务器 "+svc+" 云备份密码失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		state.Logger.Info("重置服务器 "+svc+" 云备份密码成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "云备份密码已重置", "code": "E82979D39", "result": result})
	}
}

// Backup Cloud offer details
func GetBackupCloudOfferDetails(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "backupCloudOfferDetails")
		if !ok {
			return
		}
		var d map[string]interface{}
		if err := client.Get(p, &d); err != nil {
			// schema 的 desc 写的是 "if available for the current server"，即没有报价时本来就会报错，
			// 这属于正常业务状态而不是故障，降级成 notAvailable，别让前端弹"服务器内部错误"。
			code, msg := featureOVHErr(err)
			if code == http.StatusNotFound || strings.Contains(strings.ToLower(msg), "does not exist") {
				if featureIsFeatureMissing(client, svc) {
					state.Logger.Info("服务器 "+svc+" 没有可用的云备份报价", "server_control")
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "该服务器不支持云备份", "code": "E94D0ECE3", "notAvailable": true})
					return
				}
				state.Logger.Warn("查询服务器 "+svc+" 云备份报价：服务器不存在或不属于当前账户 - "+msg, "server_control")
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error":   "服务器不存在或不属于当前账户", "code": "E834268F5",
					"unknownService": true,
					"reason":         msg,
				})
				return
			}
			state.Logger.Error("查询服务器 "+svc+" 云备份报价失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "offerDetails": d})
	}
}
