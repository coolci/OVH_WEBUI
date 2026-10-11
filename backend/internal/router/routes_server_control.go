package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/handlers"
)

// registerServerControlRoutes 注册独立服务器管理端点路由组
func registerServerControlRoutes(api *gin.RouterGroup, cfg Config) {
	state := cfg.State

	sc := api.Group("/server-control")
	// :service_name 会被直接拼进 OVH 的请求路径,先在组上统一挡一道 ——
	// 名字里带 # 或 ? 会让请求被静默发到另一个端点,见 ValidateServiceName
	sc.Use(handlers.ValidateServiceName())
	{
		sc.GET("/list", handlers.ListMyServers(state))
		// 服务器本地别名:纯本地显示用,不下发 OVH
		sc.GET("/aliases", handlers.ListServerAliases(state))
		sc.PUT("/:service_name/alias", handlers.SetServerAlias(state))
		sc.DELETE("/:service_name/alias", handlers.DeleteServerAlias(state))
		sc.GET("/order-mapping", handlers.GetOrderMapping(state))
		// 14 天无理由撤单:GET 判断这台机器还能不能退(依据 OVH 的 retractionDate,
		// 不是自己算 14 天),POST 真正提交申请(不可逆,要求 confirm:true)
		sc.GET("/:service_name/retraction", handlers.GetRetraction(state))
		sc.POST("/:service_name/retraction", handlers.PostRetraction(state))
		sc.POST("/:service_name/reboot", handlers.Reboot(state))
		sc.GET("/:service_name/templates", handlers.GetOSTemplates(state))
		sc.POST("/:service_name/install", handlers.InstallOS(state))
		sc.GET("/:service_name/install/status", handlers.GetInstallStatus(state))
		sc.GET("/:service_name/tasks", handlers.GetServerTasks(state))
		sc.GET("/:service_name/tasks/:task_id/available-timeslots", handlers.GetTaskAvailableTimeslots(state))
		sc.POST("/:service_name/tasks/:task_id/schedule", handlers.ScheduleTaskTimeslot(state))

		// boot/monitoring
		// 一键救援:改 netboot → 设收信邮箱 → 重启,三步合一。
		// 手动做要在 OVH 后台点四步,而漏掉最后的重启是最常见的错误。
		sc.GET("/:service_name/rescue", handlers.GetRescueStatus(state))
		sc.POST("/:service_name/rescue", handlers.EnterRescue(state))
		sc.POST("/:service_name/rescue/exit", handlers.ExitRescue(state))
		sc.GET("/:service_name/boot", handlers.GetBootConfig(state))
		sc.PUT("/:service_name/boot/:boot_id", handlers.SetBootConfig(state))
		sc.GET("/:service_name/monitoring", handlers.GetMonitoringStatus(state))
		sc.PUT("/:service_name/monitoring", handlers.SetMonitoringStatus(state))
		sc.GET("/:service_name/boot-mode", handlers.GetBootModes(state))
		sc.PUT("/:service_name/boot-mode", handlers.ChangeBootMode(state))

		// hardware/network/dns
		sc.GET("/:service_name/hardware", handlers.GetHardwareInfo(state))
		sc.GET("/:service_name/network-specs", handlers.GetNetworkSpecs(state))
		sc.GET("/:service_name/ips", handlers.GetServerIPs(state))
		sc.GET("/:service_name/reverse", handlers.GetReverseDNS(state))
		sc.POST("/:service_name/reverse", handlers.SetReverseDNS(state))
		sc.DELETE("/:service_name/reverse/:ip", handlers.DeleteReverseDNS(state))
		sc.GET("/:service_name/serviceinfo", handlers.GetServiceInfo(state))
		sc.PUT("/:service_name/serviceinfo/renewal", handlers.UpdateServiceRenewal(state))

		// engagement(合同期切换)
		sc.GET("/:service_name/engagement", handlers.GetEngagement(state))
		sc.GET("/:service_name/engagement/available", handlers.GetEngagementAvailable(state))
		sc.GET("/:service_name/engagement/request", handlers.GetEngagementRequest(state))
		sc.POST("/:service_name/engagement/request", handlers.CreateEngagementRequest(state))
		sc.DELETE("/:service_name/engagement/request", handlers.DeleteEngagementRequest(state))
		sc.PUT("/:service_name/engagement/end-rule", handlers.UpdateEngagementEndRule(state))

		// DDoS mitigation
		sc.GET("/:service_name/mitigation", handlers.GetMitigation(state))
		sc.POST("/:service_name/mitigation/:ip", handlers.EnableMitigation(state))
		sc.DELETE("/:service_name/mitigation/:ip", handlers.DisableMitigation(state))
		sc.POST("/:service_name/change-contact", handlers.ChangeContact(state))
		sc.GET("/:service_name/interventions", handlers.GetInterventions(state))
		sc.GET("/:service_name/interventions/:intervention_id", handlers.GetInterventionDetail(state))
		sc.GET("/:service_name/planned-interventions", handlers.GetPlannedInterventions(state))
		sc.GET("/:service_name/planned-interventions/:intervention_id", handlers.GetPlannedInterventionDetail(state))
		sc.POST("/:service_name/hardware/replace", handlers.HardwareReplace(state))
		sc.GET("/:service_name/hardware-raid-profiles", handlers.GetHardwareRaidProfiles(state))
		sc.GET("/:service_name/hardware-disk-info", handlers.GetHardwareDiskInfo(state))
		sc.GET("/:service_name/partition-schemes", handlers.GetPartitionSchemes(state))

		// network
		sc.GET("/:service_name/network-interfaces", handlers.GetNetworkInterfaces(state))
		sc.GET("/:service_name/mrtg", handlers.GetMRTGData(state))
		sc.POST("/:service_name/ola/aggregation", handlers.ConfigureOLAAggregation(state))
		sc.POST("/:service_name/ola/reset", handlers.ResetOLAConfiguration(state))
		sc.POST("/:service_name/ola/group", handlers.OLAGroup(state))
		sc.POST("/:service_name/ola/ungroup", handlers.OLAUngroup(state))
		sc.GET("/:service_name/console", handlers.GetIPMIConsole(state))
		// 只查支持哪几种控制台类型(HTML5 / Java KVM / SOL),不申请会话
		sc.GET("/:service_name/ipmi-types", handlers.GetIPMIAccessTypes(state))
		sc.GET("/:service_name/statistics", handlers.GetTrafficStatistics(state))
		sc.GET("/:service_name/network-stats", handlers.GetNetworkInterfaceStats(state))

		// features
		sc.GET("/:service_name/burst", handlers.GetBurst(state))
		sc.PUT("/:service_name/burst", handlers.UpdateBurst(state))
		sc.GET("/:service_name/firewall", handlers.GetFirewall(state))
		sc.PUT("/:service_name/firewall", handlers.UpdateFirewall(state))
		sc.GET("/:service_name/backup-ftp", handlers.GetBackupFTP(state))
		sc.POST("/:service_name/backup-ftp", handlers.ActivateBackupFTP(state))
		sc.DELETE("/:service_name/backup-ftp", handlers.DeleteBackupFTP(state))
		sc.GET("/:service_name/backup-ftp/access", handlers.GetBackupFTPAccess(state))
		sc.POST("/:service_name/backup-ftp/access", handlers.AddBackupFTPAccess(state))
		// ipBlock 是带掩码的 CIDR(如 37.59.1.0/28)。gin 默认 UseRawPath=false,%2F 会被还原成 "/",
		// 把 URL 撑成多一段,:ip_block 永远匹配不上(实测编码与否都 404)。
		// 所以主用 query 形式 ?ipBlock=...,旧的路径形式保留做兼容(handler 三级兜底取值)。
		sc.DELETE("/:service_name/backup-ftp/access", handlers.DeleteBackupFTPAccess(state))
		sc.DELETE("/:service_name/backup-ftp/access/:ip_block", handlers.DeleteBackupFTPAccess(state))
		sc.POST("/:service_name/backup-ftp/password", handlers.ChangeBackupFTPPassword(state))
		sc.GET("/:service_name/backup-ftp/authorizable-blocks", handlers.GetBackupFTPAuthorizableBlocks(state))
		sc.GET("/:service_name/backup-cloud", handlers.GetBackupCloud(state))
		sc.GET("/:service_name/backup-cloud/offer-details", handlers.GetBackupCloudOfferDetails(state))
		// 云备份的写操作(官方 /features/backupCloud POST/DELETE 与 /password POST),
		// 原来只实现了只读,用户无法在控制台激活/停用/重置密码
		sc.POST("/:service_name/backup-cloud", handlers.ActivateBackupCloud(state))
		sc.DELETE("/:service_name/backup-cloud", handlers.DeleteBackupCloud(state))
		sc.POST("/:service_name/backup-cloud/password", handlers.ChangeBackupCloudPassword(state))

		// misc
		sc.GET("/:service_name/secondary-dns", handlers.GetSecondaryDNS(state))
		sc.POST("/:service_name/secondary-dns", handlers.AddSecondaryDNS(state))
		sc.DELETE("/:service_name/secondary-dns/:domain", handlers.DeleteSecondaryDNS(state))
		sc.GET("/:service_name/virtual-mac", handlers.GetVirtualMACList(state))
		sc.POST("/:service_name/virtual-mac", handlers.CreateVirtualMAC(state))
		sc.GET("/:service_name/virtual-network-interface", handlers.GetVirtualNetworkInterfaces(state))
		sc.GET("/:service_name/vrack", handlers.GetVRackList(state))
		sc.DELETE("/:service_name/vrack/:vrack", handlers.RemoveFromVRack(state))
		sc.GET("/:service_name/orderable/bandwidth", handlers.GetOrderableBandwidth(state))
		sc.GET("/:service_name/orderable/traffic", handlers.GetOrderableTraffic(state))
		sc.GET("/:service_name/orderable/ip", handlers.GetOrderableIP(state))
		sc.GET("/:service_name/options", handlers.GetServerOptions(state))
		sc.GET("/:service_name/ip-specs", handlers.GetIPSpecs(state))
		sc.GET("/:service_name/ip/can-be-moved-to", handlers.GetIPCanBeMovedTo(state))
		sc.GET("/:service_name/ip/country-available", handlers.GetIPCountryAvailable(state))
		sc.POST("/:service_name/ip/move", handlers.MoveIP(state))
		sc.GET("/:service_name/ongoing", handlers.GetOngoingTasks(state))
		sc.GET("/:service_name/license/windows/compliant", handlers.GetCompliantWindowsVersions(state))
		sc.GET("/:service_name/license/windows-sql/compliant", handlers.GetCompliantWindowsSqlVersions(state))
		// 到期终止走 terminationPolicy,不是 /terminate ——
		// 后者是立即终止,提交即暂停服务器
		sc.PUT("/:service_name/termination-policy", handlers.UpdateTerminationPolicy(state))
		sc.POST("/:service_name/terminate", handlers.TerminateService(state))
		sc.POST("/:service_name/confirm-termination", handlers.ConfirmTermination(state))
		sc.GET("/:service_name/spla", handlers.GetSPLAList(state))
		sc.POST("/:service_name/spla", handlers.CreateSPLA(state))
		sc.GET("/:service_name/bios-settings", handlers.GetBIOSSettings(state))
		sc.GET("/:service_name/bios-settings/sgx", handlers.GetBIOSSettingsSGX(state))
	}
}
