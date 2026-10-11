package router

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/handlers"
)

// availabilityHandler 用 *planCode 通配符处理像 "/api/availability/24sk20-ram-64g" 这样的路径
func availabilityHandler(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		pc := c.Param("planCode")
		pc = strings.TrimPrefix(pc, "/")
		c.Params = append(c.Params[:0], gin.Param{Key: "planCode", Value: pc})
		h(c)
	}
}

// registerAPIRoutes 注册核心业务与控制端点
func registerAPIRoutes(api *gin.RouterGroup, cfg Config) {
	state := cfg.State
	mon := cfg.Monitor

	// Health
	api.GET("/health", handlers.Health())

	// Settings
	api.GET("/settings", handlers.GetSettings(state))
	api.POST("/settings", handlers.SaveSettings(state))
	api.POST("/verify-auth", handlers.VerifyAuth(state))
	api.GET("/endpoint-config", handlers.EndpointConfig(state))

	// Logs / stats
	api.GET("/logs", handlers.GetLogs(state))
	api.POST("/logs/flush", handlers.FlushLogs(state))
	api.DELETE("/logs", handlers.ClearLogs(state))
	api.GET("/stats", handlers.GetStats(state, mon))

	// Queue
	api.GET("/queue", handlers.GetQueue(state))
	api.GET("/queue/timings", handlers.GetPurchaseTimings(state))
	api.POST("/queue", handlers.AddQueueItem(state))
	api.DELETE("/queue/clear", handlers.ClearQueue(state))
	api.DELETE("/queue/:id", handlers.RemoveQueueItem(state))
	api.PUT("/queue/:id/status", handlers.UpdateQueueStatus(state))
	api.PUT("/queue/:id/interval", handlers.UpdateQueueInterval(state))

	// Purchase history
	api.GET("/purchase-history", handlers.GetPurchaseHistory(state))
	api.DELETE("/purchase-history", handlers.ClearPurchaseHistory(state))
	api.POST("/purchase-history/refresh-status", handlers.RefreshOrderStatuses(state))

	// Monitor
	api.GET("/monitor/subscriptions", handlers.GetSubscriptions(state, mon))
	api.POST("/monitor/subscriptions", handlers.AddSubscription(state, mon))
	api.PUT("/monitor/subscriptions/:planCode", handlers.UpdateSubscription(state, mon))
	api.POST("/monitor/subscriptions/batch-add-all", handlers.BatchAddAll(state, mon))
	api.DELETE("/monitor/subscriptions/clear", handlers.ClearSubscriptions(state, mon))
	api.DELETE("/monitor/subscriptions/:planCode", handlers.RemoveSubscription(state, mon))
	api.GET("/monitor/subscriptions/:planCode/history", handlers.GetSubscriptionHistory(state, mon))
	api.POST("/monitor/start", handlers.StartMonitor(state, mon))
	api.POST("/monitor/stop", handlers.StopMonitor(state, mon))
	api.GET("/monitor/status", handlers.GetMonitorStatus(state, mon))
	api.PUT("/monitor/interval", handlers.SetMonitorInterval(state, mon))
	api.POST("/monitor/test-notification", handlers.TestNotification(state))
	api.GET("/notify/channels", handlers.GetNotifyChannels(state))
	api.GET("/telegram/verify", handlers.VerifyTelegram(state))

	// Telegram
	api.GET("/telegram/status", handlers.TelegramStatus(state))
	api.GET("/telegram/poller", handlers.TelegramStatus(state))
	api.POST("/telegram/quick-order", handlers.TelegramQuickOrder(state, mon))
	api.POST("/telegram/register-commands", handlers.RegisterTelegramCommands(state))

	// Outbound proxy
	api.POST("/accounts/:id/proxy-test", handlers.TestAccountProxy(state))
	api.POST("/accounts/:id/proxy-check", handlers.CheckAccountProxy(state))
	api.GET("/accounts/proxy-status", handlers.ProxyStatus(state))

	// Servers / availability / cache
	api.GET("/servers", handlers.GetServers(state))
	api.GET("/availability/*planCode", availabilityHandler(handlers.GetAvailability(state)))
	api.POST("/availability/*planCode", availabilityHandler(handlers.GetAvailability(state)))
	api.POST("/internal/monitor/price", handlers.MonitorPrice(state))
	api.POST("/servers/:planCode/price", handlers.ServerPrice(state))
	api.GET("/cache/info", handlers.CacheInfo(state))
	api.POST("/cache/clear", handlers.ClearCache(state))
	api.GET("/catalog", handlers.GetCatalog(state))
	api.GET("/system/metrics", handlers.GetSystemMetrics(state))
	api.GET("/version", handlers.GetVersion(state))

	// App pairing
	api.POST("/app/pair", handlers.RedeemPairingCode(state))
	api.POST("/app/pairing-codes", handlers.CreatePairingCode(state))
	api.GET("/app/devices", handlers.ListAppDevices(state))
	api.DELETE("/app/devices/:id", handlers.RevokeAppDevice(state))
	api.GET("/version/check-update", handlers.CheckUpdate(state))
	api.POST("/version/update", handlers.SelfUpdate(state, cfg.OnRestart))
	api.GET("/version/update/status", handlers.GetUpdateStatus(state))

	// Accounts
	api.GET("/accounts", handlers.ListAccounts(state))
	api.GET("/accounts/:id", handlers.GetAccountByID(state))
	api.POST("/accounts", handlers.CreateAccount(state))
	api.PUT("/accounts/:id", handlers.UpdateAccount(state))
	api.DELETE("/accounts/:id", handlers.DeleteAccountByID(state))
	api.POST("/accounts/:id/set-default", handlers.SetDefaultAccountByID(state))
	api.POST("/accounts/:id/verify", handlers.VerifyAccount(state))

	// Quick order
	api.POST("/queue/quick-order", handlers.QuickOrder(state))
}
