package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// GetSettings GET /api/settings
func GetSettings(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, state.Config.Get())
	}
}

// SaveSettings POST /api/settings
// 合并更新：前端可不传敏感/内部字段；空值保留服务端已有配置，避免抹掉 TgWebhookSecret 等。
func SaveSettings(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var patch types.Config
		if err := c.ShouldBindJSON(&patch); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		prev := state.Config.Get()
		newCfg := prev

		// 凭据去空白（前端粘贴时常带空格/换行，会导致 OVH 签名失败 "Invalid signature"）
		patch.AppKey = strings.TrimSpace(patch.AppKey)
		patch.AppSecret = strings.TrimSpace(patch.AppSecret)
		patch.ConsumerKey = strings.TrimSpace(patch.ConsumerKey)
		patch.TgToken = strings.TrimSpace(patch.TgToken)
		patch.TgChatID = strings.TrimSpace(patch.TgChatID)
		patch.TgWebhookSecret = strings.TrimSpace(patch.TgWebhookSecret)
		patch.Endpoint = strings.TrimSpace(patch.Endpoint)
		patch.Zone = strings.TrimSpace(patch.Zone)
		patch.IAM = strings.TrimSpace(patch.IAM)

		// 非空才覆盖（合并语义）；Webhook secret 绝不用空串覆盖
		if patch.AppKey != "" {
			newCfg.AppKey = patch.AppKey
		}
		if patch.AppSecret != "" {
			newCfg.AppSecret = patch.AppSecret
		}
		if patch.ConsumerKey != "" {
			newCfg.ConsumerKey = patch.ConsumerKey
		}
		if patch.TgToken != "" {
			newCfg.TgToken = patch.TgToken
		}
		// ChatID 允许显式清空？一般不允许空覆盖已有，避免误清通知
		if patch.TgChatID != "" {
			newCfg.TgChatID = patch.TgChatID
		}
		if patch.TgWebhookSecret != "" {
			newCfg.TgWebhookSecret = patch.TgWebhookSecret
		}
		if patch.Endpoint != "" {
			newCfg.Endpoint = patch.Endpoint
		}
		if patch.Zone != "" {
			newCfg.Zone = patch.Zone
		}
		if patch.IAM != "" {
			newCfg.IAM = patch.IAM
		}

		// 默认值兜底
		if newCfg.Endpoint == "" {
			newCfg.Endpoint = "ovh-eu"
		}
		if newCfg.Zone == "" {
			newCfg.Zone = "IE"
		}

		if err := state.Config.Set(newCfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		state.Logger.Info("API settings updated in config.json", "system")

		// TG 配置变更 → 同步发测试消息（1:1 对应 Python save_settings 2450-2463）
		if newCfg.TgToken != "" && newCfg.TgChatID != "" {
			changed := newCfg.TgToken != prev.TgToken || newCfg.TgChatID != prev.TgChatID
			if changed || prev.TgToken == "" || prev.TgChatID == "" {
				state.Logger.Info("Telegram Token或Chat ID已更新/设置。尝试发送Telegram测试消息到 Chat ID: "+newCfg.TgChatID, "")
				if telegram.SendMessage(state, "OVH 控制台: Telegram 通知已成功配置 (来自 Go 后端测试)", nil) {
					state.Logger.Info("Telegram 测试消息发送成功。", "")
				} else {
					state.Logger.Warn("Telegram 测试消息发送失败。请检查 Token 和 Chat ID 以及后端日志。", "")
				}
			} else {
				state.Logger.Info("Telegram 配置未更改，跳过测试消息。", "")
			}
		} else {
			state.Logger.Info("未配置 Telegram Token 或 Chat ID，跳过测试消息。", "")
		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// VerifyAuth POST /api/verify-auth
func VerifyAuth(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"valid": false})
			return
		}
		var me map[string]interface{}
		if err := client.Get("/me", &me); err != nil {
			state.Logger.Error("Authentication verification failed: "+err.Error(), "system")
			c.JSON(http.StatusOK, gin.H{"valid": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"valid": true})
	}
}

// EndpointConfig GET /api/endpoint-config
func EndpointConfig(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := state.Config.Get()
		c.JSON(http.StatusOK, gin.H{
			"endpoint": cfg.Endpoint,
			"zone":     cfg.Zone,
		})
	}
}
