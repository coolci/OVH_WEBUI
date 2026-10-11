package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/handlers"
)

// registerVPSMonitorAndSupportRoutes 注册支持工单、VPS 监控与 OVH 账户端点
func registerVPSMonitorAndSupportRoutes(api *gin.RouterGroup, cfg Config) {
	state := cfg.State

	// 支持工单：列表、详情、消息、创建、回复、关闭、重新打开
	api.GET("/support/tickets", handlers.ListSupportTickets(state))
	api.POST("/support/tickets", handlers.CreateSupportTicket(state))
	api.GET("/support/tickets/:ticket_id", handlers.GetSupportTicket(state))
	api.GET("/support/tickets/:ticket_id/messages", handlers.ListSupportTicketMessages(state))
	api.POST("/support/tickets/:ticket_id/reply", handlers.ReplySupportTicket(state))
	api.POST("/support/tickets/:ticket_id/reopen", handlers.ReopenSupportTicket(state))
	api.POST("/support/tickets/:ticket_id/close", handlers.CloseSupportTicket(state))

	// VPS monitor
	api.GET("/vps-monitor/models", handlers.GetVPSModels(state))
	api.GET("/vps-monitor/subscriptions", handlers.GetVPSSubscriptions(state))
	api.POST("/vps-monitor/subscriptions", handlers.AddVPSSubscription(state))
	api.POST("/vps-monitor/subscriptions/batch-add-all", handlers.BatchAddAllVPS(state))
	api.PUT("/vps-monitor/subscriptions/:subscription_id", handlers.UpdateVPSSubscription(state))
	api.DELETE("/vps-monitor/subscriptions/clear", handlers.ClearVPSSubscriptions(state))
	api.DELETE("/vps-monitor/subscriptions/:subscription_id", handlers.RemoveVPSSubscription(state))
	api.GET("/vps-monitor/subscriptions/:subscription_id/history", handlers.GetVPSSubscriptionHistory(state))
	api.POST("/vps-monitor/start", handlers.StartVPSMonitor(state))
	api.POST("/vps-monitor/stop", handlers.StopVPSMonitor(state))
	api.GET("/vps-monitor/status", handlers.GetVPSMonitorStatus(state))
	api.PUT("/vps-monitor/interval", handlers.SetVPSMonitorInterval(state))
	api.POST("/vps-monitor/check/:plan_code", handlers.ManualCheckVPS(state))

	// Account
	api.GET("/ovh/account/info", handlers.GetAccountInfo(state))
	api.GET("/ovh/account/refunds", handlers.GetAccountRefunds(state))
	api.GET("/ovh/account/credit-balance", handlers.GetCreditBalance(state))
	api.GET("/ovh/account/email-history", handlers.GetEmailHistory(state))
	api.GET("/ovh/contact-change-requests", handlers.GetContactChangeRequests(state))
	api.GET("/ovh/contact-change-requests/:task_id", handlers.GetContactChangeRequestDetail(state))
	api.POST("/ovh/contact-change-requests/:task_id/accept", handlers.AcceptContactChangeRequest(state))
	api.POST("/ovh/contact-change-requests/:task_id/refuse", handlers.RefuseContactChangeRequest(state))
	api.POST("/ovh/contact-change-requests/:task_id/resend-email", handlers.ResendContactChangeEmail(state))
	api.GET("/ovh/account/sub-accounts", handlers.GetSubAccounts(state))
	api.GET("/ovh/account/bills", handlers.GetAccountBills(state))
	api.GET("/ovh/account/orders", handlers.GetAccountOrders(state))
}
