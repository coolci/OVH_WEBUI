package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/handlers"
)

// registerVPSControlRoutes 注册 VPS 管理端点路由组
func registerVPSControlRoutes(api *gin.RouterGroup, cfg Config) {
	state := cfg.State

	vc := api.Group("/vps-control")
	// :service_name 会被直接拼进 OVH 的请求路径,先在组上统一挡一道 ——
	// 名字里带 # 或 ? 会让请求被静默发到另一个端点,见 ValidateServiceName
	vc.Use(handlers.ValidateServiceName())
	{
		vc.GET("/list", handlers.ListVps(state))
		vc.GET("/:service_name/info", handlers.GetVpsInfo(state))
		vc.GET("/:service_name/status", handlers.GetVpsServiceStatus(state))
		vc.GET("/:service_name/serviceinfo", handlers.GetVpsServiceInfo(state))
		vc.PUT("/:service_name/serviceinfo/renewal", handlers.UpdateVpsRenewal(state))
		vc.GET("/:service_name/ips", handlers.GetVpsIps(state))
		vc.PUT("/:service_name/ips/:ip/reverse", handlers.SetVpsIpReverse(state))
		vc.GET("/:service_name/datacenter", handlers.GetVpsDatacenter(state))

		// 电源
		vc.POST("/:service_name/start", handlers.VpsStart(state))
		vc.POST("/:service_name/stop", handlers.VpsStop(state))
		vc.POST("/:service_name/reboot", handlers.VpsReboot(state))
		vc.POST("/:service_name/console", handlers.VpsGetConsoleUrl(state))
		vc.POST("/:service_name/password", handlers.VpsSetPassword(state))

		// 重装系统（支持标准 /rebuild 与历史兼容 /reinstall）
		vc.GET("/:service_name/current-os", handlers.GetVpsCurrentOS(state))
		vc.GET("/:service_name/templates", handlers.GetVpsTemplates(state))
		vc.GET("/:service_name/ssh-keys", handlers.GetAccountSSHKeys(state))
		vc.POST("/:service_name/rebuild", handlers.ReinstallVps(state))
		vc.POST("/:service_name/reinstall", handlers.ReinstallVps(state))

		// 任务
		vc.GET("/:service_name/tasks", handlers.GetVpsTasks(state))
		vc.GET("/:service_name/tasks/:task_id", handlers.GetVpsTaskDetail(state))

		// 快照
		vc.GET("/:service_name/snapshot", handlers.GetVpsSnapshot(state))
		vc.POST("/:service_name/snapshot", handlers.CreateVpsSnapshot(state))
		vc.PUT("/:service_name/snapshot", handlers.UpdateVpsSnapshotDescription(state))
		vc.POST("/:service_name/snapshot/revert", handlers.RevertVpsSnapshot(state))
		vc.DELETE("/:service_name/snapshot", handlers.DeleteVpsSnapshot(state))

		// 杂项
		vc.POST("/:service_name/change-contact", handlers.ChangeVpsContact(state))
		vc.PUT("/:service_name/termination-policy", handlers.UpdateVpsTerminationPolicy(state))
		vc.POST("/:service_name/terminate", handlers.TerminateVps(state))
		vc.POST("/:service_name/confirm-termination", handlers.ConfirmVpsTermination(state))
		vc.GET("/:service_name/secondary-dns", handlers.GetVpsSecondaryDns(state))
		vc.POST("/:service_name/secondary-dns", handlers.AddVpsSecondaryDns(state))
		vc.DELETE("/:service_name/secondary-dns/:domain", handlers.DeleteVpsSecondaryDns(state))
		vc.GET("/:service_name/options", handlers.GetVpsOptions(state))
		vc.DELETE("/:service_name/options/:option", handlers.DeleteVpsOption(state))
		vc.GET("/:service_name/automated-backup", handlers.GetVpsAutomatedBackup(state))

		// 14 天无理由撤单:GET 判断这台 VPS 还能不能退,POST 提交申请
		vc.GET("/:service_name/retraction", handlers.GetRetraction(state))
		vc.POST("/:service_name/retraction", handlers.PostRetraction(state))

		// 合同期(engagement)
		vc.GET("/:service_name/engagement", handlers.GetVpsEngagement(state))
		vc.GET("/:service_name/engagement/available", handlers.GetVpsEngagementAvailable(state))
		vc.GET("/:service_name/engagement/request", handlers.GetVpsEngagementRequest(state))
		vc.POST("/:service_name/engagement/request", handlers.CreateVpsEngagementRequest(state))
		vc.DELETE("/:service_name/engagement/request", handlers.DeleteVpsEngagementRequest(state))
		vc.PUT("/:service_name/engagement/end-rule", handlers.UpdateVpsEngagementEndRule(state))

		// DDoS mitigation(IP 级别,但 IP 列表从 /vps/{svc}/ips 取)
		vc.GET("/:service_name/mitigation", handlers.GetVpsMitigation(state))
		vc.POST("/:service_name/mitigation/:ip", handlers.EnableVpsMitigation(state))
		vc.DELETE("/:service_name/mitigation/:ip", handlers.DisableVpsMitigation(state))
	}
}
