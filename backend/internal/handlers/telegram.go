package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// SetTelegramWebhook POST /api/telegram/set-webhook
func SetTelegramWebhook(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			WebhookURL string `json:"webhook_url"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.WebhookURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 webhook_url 参数"})
			return
		}
		ok, msg, info := telegram.SetWebhook(state, body.WebhookURL)
		if ok {
			c.JSON(http.StatusOK, gin.H{
				"success":      true,
				"message":      "Webhook 设置成功",
				"webhook_url":  msg,
				"webhook_info": info,
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "设置失败: " + msg})
	}
}

// GetTelegramWebhookInfo GET /api/telegram/get-webhook-info
func GetTelegramWebhookInfo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, info, errMsg := telegram.GetWebhookInfo(state)
		if !ok {
			status := http.StatusBadRequest
			if strings.Contains(errMsg, "未配置") {
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{"success": false, "error": errMsg})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "webhook_info": info})
	}
}

// TelegramWebhook POST /api/telegram/webhook
func TelegramWebhook(state *app.State, mon *monitor.Monitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}

		// 处理 callback_query
		if cb, ok := data["callback_query"].(map[string]interface{}); ok {
			cbData, _ := cb["data"].(string)
			message, _ := cb["message"].(map[string]interface{})
			chatID := getNested(message, "chat", "id")
			messageID, _ := getNumOrFloat(message["message_id"])
			fromUser, _ := cb["from"].(map[string]interface{})
			userID, _ := getNumOrFloat(fromUser["id"])
			state.Logger.Info(fmt.Sprintf("收到Telegram回调: user_id=%v, callback_data=%s...", userID, truncate(cbData, 50)), "telegram")

			cbID := fmt.Sprintf("%v", cb["id"])
			var callbackObj map[string]interface{}
			if strings.HasPrefix(cbData, "b64:") {
				base64Part := cbData[4:]
				// padding
				if missing := len(base64Part) % 4; missing != 0 {
					base64Part += strings.Repeat("=", 4-missing)
				}
				decoded, err := base64.StdEncoding.DecodeString(base64Part)
				if err != nil {
					state.Logger.Warn("base64解码失败（可能是数据被截断）: "+err.Error()+", base64_len="+fmt.Sprintf("%d", len(cbData[4:])), "telegram")
					state.Logger.Warn("callback_data（前100字符）: "+truncate(cbData, 100), "telegram")
					telegram.AnswerCallback(state, cbID, "按钮数据无效", true)
					c.JSON(http.StatusOK, gin.H{"ok": true, "error": "callback_decode_failed"})
					return
				}
				if err := json.Unmarshal(decoded, &callbackObj); err != nil {
					state.Logger.Error("解析callback_data JSON失败: "+err.Error()+", data="+truncate(cbData, 100), "telegram")
					telegram.AnswerCallback(state, cbID, "按钮数据无效", true)
					c.JSON(http.StatusOK, gin.H{"ok": true, "error": "invalid_callback_json"})
					return
				}
			} else {
				if err := json.Unmarshal([]byte(cbData), &callbackObj); err != nil {
					state.Logger.Error("解析callback_data JSON失败: "+err.Error()+", data="+truncate(cbData, 100), "telegram")
					telegram.AnswerCallback(state, cbID, "按钮数据无效", true)
					c.JSON(http.StatusOK, gin.H{"ok": true, "error": "invalid_callback_json"})
					return
				}
			}

			action := ""
			if v, ok := callbackObj["a"].(string); ok {
				action = v
			} else if v, ok := callbackObj["action"].(string); ok {
				action = v
			}

			if action == "add_to_queue" {
				messageUUID := ""
				if v, ok := callbackObj["u"].(string); ok {
					messageUUID = v
				} else if v, ok := callbackObj["uuid"].(string); ok {
					messageUUID = v
				}
				if messageUUID != "" {
					if cached := mon.MessageUUIDCacheLookup(messageUUID); cached != nil {
						planCode := cached.PlanCode
						dc := cached.Datacenter
						options := cached.Options
						state.Logger.Info(fmt.Sprintf("✅ 从UUID缓存恢复配置: UUID=%s, %s@%s, options=%v",
							messageUUID, planCode, dc, options), "telegram")
						item := types.QueueItem{
							ID:            uuid.NewString(),
							PlanCode:      planCode,
							Datacenter:    dc,
							Options:       options,
							Status:        "running",
							CreatedAt:     types.NowISO(),
							UpdatedAt:     types.NowISO(),
							RetryInterval: 30,
							RetryCount:    0,
							LastCheckTime: 0,
							FromTelegram:  true,
						}
						state.QueueMu.Lock()
						state.Queue = append(state.Queue, item)
						state.QueueMu.Unlock()
						_ = state.SaveQueue()
						optsStr := strings.Join(options, ", ")
						if optsStr == "" {
							optsStr = "无（默认配置）"
						}
						state.Logger.Info(fmt.Sprintf("Telegram用户 %v 通过UUID按钮添加到队列: %s@%s, 配置选项: %s",
							userID, planCode, dc, optsStr), "telegram")
						confirmMsg := fmt.Sprintf("✅ 已添加到抢购队列！\n\n型号: %s\n机房: %s\n配置: %s\n\n系统将自动尝试下单。",
							planCode, strings.ToUpper(dc), optsStr)
						telegram.AnswerCallback(state, cbID, "已添加到队列！", false)
						telegram.SendReply(state, chatID, confirmMsg, int64(messageID))
						// 必须 200：Telegram 收到非 2xx 会重试并标记 webhook 故障
						c.JSON(http.StatusOK, gin.H{"ok": true})
						return
					}
					state.Logger.Warn("UUID未找到 in cache: "+messageUUID, "telegram")
				}

				// 降级到旧机制（callback 内直接带 p/d/o）
				planCode := strOr(callbackObj, "p", "planCode")
				dc := strOr(callbackObj, "d", "datacenter")
				var options []string
				if optsRaw, ok := callbackObj["o"]; ok {
					options = toStringSlice(optsRaw)
				} else if optsRaw, ok := callbackObj["options"]; ok {
					options = toStringSlice(optsRaw)
				}
				if planCode == "" || dc == "" {
					// 典型原因：部署重启后旧按钮 UUID 未持久化；或按钮已超过 24h TTL
					state.Logger.Warn(fmt.Sprintf("一键下单失败: UUID=%s 缓存未命中且无 planCode/datacenter 降级数据", messageUUID), "telegram")
					telegram.AnswerCallback(state, cbID, "按钮已失效，请等待新的上架通知后再点", true)
					if chatID != nil {
						telegram.SendReply(state, chatID,
							"❌ 一键下单失败：该通知按钮已过期或服务刚重启导致缓存丢失。\n\n请等待新的上架通知后重试，或在网页端手动加入抢购队列。",
							int64(messageID))
					}
					// 对 Telegram 永远回 200，避免 pending_update 堆积与 webhook 报错
					c.JSON(http.StatusOK, gin.H{"ok": true, "error": "button_expired"})
					return
				}
				if len(options) == 0 {
					cacheKey := planCode + "|" + dc
					options = mon.OptionsCacheLookup(cacheKey)
					if len(options) > 0 {
						state.Logger.Info("✅ 从缓存恢复 options: "+cacheKey+" = "+strings.Join(options, ","), "telegram")
					}
				}
				item := types.QueueItem{
					ID:            uuid.NewString(),
					PlanCode:      planCode,
					Datacenter:    dc,
					Options:       options,
					Status:        "running",
					CreatedAt:     types.NowISO(),
					UpdatedAt:     types.NowISO(),
					RetryInterval: 30,
					RetryCount:    0,
					LastCheckTime: 0,
					FromTelegram:  true,
				}
				state.QueueMu.Lock()
				state.Queue = append(state.Queue, item)
				state.QueueMu.Unlock()
				_ = state.SaveQueue()
				optsStr := strings.Join(options, ", ")
				if optsStr == "" {
					optsStr = "无（默认配置）"
				}
				state.Logger.Info(fmt.Sprintf("Telegram用户 %v 通过按钮添加到队列（旧机制）: %s@%s, 配置选项: %s",
					userID, planCode, dc, optsStr), "telegram")
				confirmText := strings.Join(options, ", ")
				if confirmText == "" {
					confirmText = "默认配置"
				}
				confirmMsg := fmt.Sprintf("✅ 已添加到抢购队列！\n\n型号: %s\n机房: %s\n配置: %s\n\n系统将自动尝试下单。",
					planCode, strings.ToUpper(dc), confirmText)
				telegram.AnswerCallback(state, cbID, "已添加到队列！", false)
				telegram.SendReply(state, chatID, confirmMsg, int64(messageID))
				c.JSON(http.StatusOK, gin.H{"ok": true})
				return
			}
			state.Logger.Warn("未知的action: "+action, "telegram")
			// 未知 action 也回 200，避免 Telegram 反复重推
			telegram.AnswerCallback(state, fmt.Sprintf("%v", cb["id"]), "未知操作", true)
			c.JSON(http.StatusOK, gin.H{"ok": true, "error": "unknown_action"})
			return
		}

		// 处理普通消息
		if msg, ok := data["message"].(map[string]interface{}); ok {
			text, _ := msg["text"].(string)
			text = strings.TrimSpace(text)
			chatID := getNested(msg, "chat", "id")
			messageID, _ := getNumOrFloat(msg["message_id"])
			fromUser, _ := msg["from"].(map[string]interface{})
			userID, _ := getNumOrFloat(fromUser["id"])
			username, _ := fromUser["username"].(string)
			if username == "" {
				username = "未知用户"
			}
			state.Logger.Info(fmt.Sprintf("收到Telegram普通消息: user_id=%v, username=%s, text=%s",
				userID, username, truncate(text, 100)), "telegram")

			orderInfo := telegram.ParseOrderMessage(text)
			if orderInfo != nil {
				state.Logger.Info(fmt.Sprintf("解析下单消息: planCode=%s, datacenter=%s, quantity=%d, options=%v",
					orderInfo.PlanCode, orderInfo.Datacenter, orderInfo.Quantity, orderInfo.Options), "telegram")
				result := telegram.ProcessOrder(state, orderInfo.PlanCode, orderInfo.Datacenter, orderInfo.Quantity, orderInfo.Options)
				var reply string
				if result.Success {
					dcText := "所有可用机房"
					if orderInfo.Datacenter != "" {
						dcText = strings.ToUpper(orderInfo.Datacenter)
					}
					optsText := "所有可用配置"
					if len(orderInfo.Options) > 0 {
						optsText = strings.Join(orderInfo.Options, ", ")
					}
					reply = fmt.Sprintf("✅ 下单成功！\n\n型号: %s\n机房: %s\n数量: %d\n配置: %s\n\n已创建: %d/%d 个订单\n系统将自动尝试下单。",
						orderInfo.PlanCode, dcText, orderInfo.Quantity, optsText, result.CreatedOrders, result.TotalOrders)
				} else {
					reply = "❌ 下单失败\n\n" + result.Message
				}
				telegram.SendReply(state, chatID, reply, int64(messageID))
				c.JSON(http.StatusOK, gin.H{"ok": true})
				return
			}
			state.Logger.Debug("消息不是下单格式，忽略", "telegram")
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
		_ = time.Now() // keep import
	}
}

func getNested(m map[string]interface{}, keys ...string) interface{} {
	var cur interface{} = m
	for _, k := range keys {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func getNumOrFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func strOr(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func toStringSlice(v interface{}) []string {
	out := []string{}
	switch x := v.(type) {
	case []interface{}:
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, x...)
	}
	return out
}
