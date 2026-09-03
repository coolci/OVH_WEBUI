package telegram

import (
	"fmt"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// BindQueueTelegram 把进度消息绑到刚入队的任务上，后续 edit 同一条。
func BindQueueTelegram(state *app.State, ids []string, chatID string, messageID int64) {
	if len(ids) == 0 || messageID == 0 || strings.TrimSpace(chatID) == "" {
		return
	}
	want := map[string]struct{}{}
	for _, id := range ids {
		want[id] = struct{}{}
	}
	state.QueueMu.Lock()
	for i := range state.Queue {
		if _, ok := want[state.Queue[i].ID]; ok {
			state.Queue[i].TelegramChatID = chatID
			state.Queue[i].TelegramMessageID = messageID
		}
	}
	state.QueueMu.Unlock()
	_ = state.SaveQueue()
}

// NotifyTaskProgress 用 EditMessage 回写排队/提交/成功/失败。绑了进度消息才改。
func NotifyTaskProgress(state *app.State, item *types.QueueItem, phase string, extra map[string]string) {
	if item == nil || item.TelegramMessageID == 0 || strings.TrimSpace(item.TelegramChatID) == "" {
		return
	}
	text := FormatTaskProgress(item, phase, extra)
	var markup map[string]interface{}
	if phase == "success" && extra["orderUrl"] != "" {
		btnText := "💳 前往 OVH 支付订单"
		if extra["orderId"] != "" {
			btnText = "💳 前往 OVH 支付订单 (" + extra["orderId"] + ")"
		}
		markup = map[string]interface{}{
			"inline_keyboard": [][]map[string]string{
				{
					{"text": btnText, "url": extra["orderUrl"]},
				},
			},
		}
	} else if phase == "queued" && item.ID != "" {
		short := shortID(item.ID)
		markup = map[string]interface{}{
			"inline_keyboard": [][]map[string]string{
				{
					{"text": "⏹ 取消此抢购任务", "callback_data": "i:T:" + short},
				},
			},
		}
	} else if phase == "accounts" || phase == "pick" {
		markup = nil
	} else {
		markup = EmptyInlineKeyboard()
	}
	if !EditMessage(state, item.TelegramChatID, item.TelegramMessageID, text, markup) {
		// 编辑失败时再发一条，避免用户完全看不到结果
		if phase == "success" || phase == "failed" || phase == "cancelled" {
			_, _ = SendToChat(state, item.TelegramChatID, text, markup)
		}
	}
}

func FormatTaskProgress(item *types.QueueItem, phase string, extra map[string]string) string {
	if extra == nil {
		extra = map[string]string{}
	}
	plan := item.PlanCode
	dc := DisplayDCFull(item.Datacenter)
	switch phase {
	case "submitting":
		return fmt.Sprintf("🚀 正在提交 OVH…\n\n📦 型号: %s\n📍 机房: %s\n🆔 任务: %s", plan, dc, shortID(item.ID))
	case "success":
		oid := extra["orderId"]
		var b strings.Builder
		b.WriteString("✅ 锁单成功！\n\n")
		b.WriteString("📦 型号: " + plan + "\n")
		b.WriteString("📍 机房: " + dc + "\n")
		if oid != "" {
			b.WriteString("🧾 订单号: " + oid + "\n")
		}
		if len(item.Options) > 0 {
			b.WriteString("⚙️ 选配: " + strings.Join(item.Options, ", ") + "\n")
		}
		b.WriteString("\n")
		if item.AutoPay {
			b.WriteString("💳 已请求默认支付方式自动扣款，请点击下方按钮核对支付状态。\n")
		} else {
			b.WriteString("⚠️ 订单尚未付款，请点击下方按钮完成支付，逾期将自动作废。\n")
		}
		b.WriteString("💡 点击下方按钮直达 OVH 支付账单：")
		return b.String()
	case "failed":
		reason := extra["reason"]
		if reason == "" {
			reason = "下单失败"
		}
		return fmt.Sprintf("❌ 抢购结束\n\n📦 型号: %s\n📍 机房: %s\n⚠️ 原因: %s", plan, dc, reason)
	case "cancelled":
		reason := extra["reason"]
		if reason == "" {
			reason = "已取消"
		}
		return fmt.Sprintf("🛑 抢购任务已终止\n\n📦 型号: %s\n📍 机房: %s\nℹ️ 说明: %s", plan, dc, reason)
	case "paused":
		return fmt.Sprintf("⏸ 抢购任务已暂停\n\n📦 型号: %s\n📍 机房: %s\nℹ️ 说明: 任务已在网页控制台暂停，恢复后将继续排队。", plan, dc)
	default:
		return fmt.Sprintf("⏳ 排队中…\n\n📦 型号: %s\n📍 机房: %s\n有货后将自动提交 OVH。", plan, dc)
	}
}

func shortID(id string) string {
	id = strings.ReplaceAll(id, "-", "")
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
