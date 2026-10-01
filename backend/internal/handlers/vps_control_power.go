package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// VpsStart POST /api/vps-control/:service_name/start
func VpsStart(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var task map[string]interface{}
		if err := client.Post("/vps/"+svc+"/start", map[string]interface{}{}, &task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("VPS "+svc+" 启动任务已创建", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "启动任务已创建", "task": task})
	}
}

// VpsStop POST /api/vps-control/:service_name/stop
func VpsStop(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var task map[string]interface{}
		if err := client.Post("/vps/"+svc+"/stop", map[string]interface{}{}, &task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("VPS "+svc+" 关机任务已创建", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "关机任务已创建", "task": task})
	}
}

// VpsReboot POST /api/vps-control/:service_name/reboot
func VpsReboot(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var task map[string]interface{}
		if err := client.Post("/vps/"+svc+"/reboot", map[string]interface{}{}, &task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("VPS "+svc+" 重启任务已创建", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "重启任务已创建", "task": task})
	}
}

// VpsGetConsoleUrl POST /api/vps-control/:service_name/console
func VpsGetConsoleUrl(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var url string
		if err := client.Post("/vps/"+svc+"/getConsoleUrl", map[string]interface{}{}, &url); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("VPS "+svc+" 控制台 URL 已生成", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "url": url})
	}
}

// VpsSetPassword POST /api/vps-control/:service_name/password
//
// /vps/{sn}/setPassword 被 OVH 标记废弃(EU/CA,删除日期 2026-10-15,无替代;
// US 从来没有)—— 按约定废弃端点不再调用。路由保留给旧前端,响应固定为
// "已下线 + 替代做法"。改密码任何时候都能走 noVNC 里的 passwd。
func VpsSetPassword(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusGone, gin.H{
			"success": false,
			"error":   "OVH 已下线 VPS 远程重置密码接口(2026-10-15 废弃,无替代)。请点「控制台」打开 noVNC,进系统后用 passwd 命令修改",
		})
	}
}
