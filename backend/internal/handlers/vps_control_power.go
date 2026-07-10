package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

// VpsStart POST /api/vps-control/:service_name/start
// /vps/{name}/start 返回 vps.Task 对象 { id, state, type, progress, date }
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
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("VPS "+svc+" 启动任务已创建", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "启动任务已创建", "task": task})
	}
}

// VpsStop POST /api/vps-control/:service_name/stop
// 注意:OVH 不会因为 stop 停止计费;省电是物理服务器视角,VPS 仍占用 hypervisor 配额
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
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("VPS "+svc+" 重启任务已创建", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "重启任务已创建", "task": task})
	}
}

// VpsGetConsoleUrl POST /api/vps-control/:service_name/console
// OVH POST /vps/{name}/getConsoleUrl 返回 string(noVNC 一次性 URL,典型 5 分钟有效)
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
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("VPS "+svc+" 控制台 URL 已生成", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "url": url})
	}
}

// VpsSetPassword POST /api/vps-control/:service_name/password
//
// EU only —— US OVHcloud 没有 setPassword 端点,需用户在 noVNC 控制台里用 passwd 自助改。
// 这里提前拒,避免 404。
func VpsSetPassword(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		acc, _ := ovhAccountFor(state, c)
		if acc.Endpoint == "ovh-us" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "美区 VPS 不支持远程重置密码 —— 请通过 Web 控制台进入系统后用 passwd 命令自助改",
			})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var task map[string]interface{}
		if err := client.Post("/vps/"+svc+"/setPassword", map[string]interface{}{}, &task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("VPS "+svc+" 密码重置任务已创建,新密码将邮件发送", "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "密码重置任务已创建,新密码已发送至邮箱", "task": task})
	}
}
