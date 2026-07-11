package telegram

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ovh-webui/server/internal/app"
)

// BotCommand 解析后的 Telegram 斜杠命令。
// 例: "/buy@MyBot 24ska01 gra 2" → Name=buy, Args=[24ska01 gra 2]
type BotCommand struct {
	Name string
	Args []string
	Raw  string
}

// KnownCommands 已注册/支持的命令名（不含斜杠）。
var KnownCommands = map[string]string{
	"start":   "显示帮助与可用命令",
	"help":    "显示帮助与可用命令",
	"stock":   "查询库存: /stock <planCode>",
	"queue":   "加入队列: /queue <planCode> [dc] [qty] [options]",
	"buy":     "快速下单: /buy <planCode> [dc] [qty] [options]",
	"monitor": "添加监控: /monitor <planCode> [dc...]",
	"price":   "查询价格: /price <planCode> <dc>",
}

// ParseBotCommand 解析以 / 开头的 Bot 命令。
// 支持 /cmd@BotName 形式；非斜杠消息返回 nil。
func ParseBotCommand(text string) *BotCommand {
	text = strings.TrimSpace(text)
	if text == "" || !strings.HasPrefix(text, "/") {
		return nil
	}
	// 去掉首个换行后的正文（有些客户端会把 caption 混入）
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		text = text[:i]
	}
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return nil
	}
	cmd := strings.TrimPrefix(parts[0], "/")
	if cmd == "" {
		return nil
	}
	// /buy@SomeBot → buy
	if at := strings.Index(cmd, "@"); at >= 0 {
		cmd = cmd[:at]
	}
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	if cmd == "" {
		return nil
	}
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}
	return &BotCommand{Name: cmd, Args: args, Raw: text}
}

// IsKnownCommand 是否为本 Bot 支持的命令。
func IsKnownCommand(name string) bool {
	_, ok := KnownCommands[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// HelpMessage 返回中文帮助文案（/start、/help、未知命令时共用）。
func HelpMessage() string {
	return strings.TrimSpace(`
🤖 OVH Sniper Bot 命令帮助

📦 库存查询
  /stock <planCode>
  例: /stock 24ska01

🛒 加入抢购队列
  /queue <planCode> [datacenter] [quantity] [options逗号分隔]
  例: /queue 24ska01 gra
  例: /queue 24ska01 gra 1 ram-32g,softraid-2x512nvme

⚡ 快速下单（同入队，有货即抢）
  /buy <planCode> [datacenter] [quantity] [options]
  例: /buy 24ska01 gra

👁 添加监控（有货推送）
  /monitor <planCode> [datacenter...]
  例: /monitor 24ska01
  例: /monitor 24ska01 gra rbx

💰 价格查询
  /price <planCode> <datacenter>
  例: /price 24ska01 gra

也可直接发送（无斜杠）:
  planCode [datacenter] [quantity] [options]
  例: 24ska01 gra 1

说明: 仅配置的 Chat ID 可下单；机房代码小写 3–4 位（gra/rbx/bhs…）。
`) + "\n"
}

// IsAuthorizedChat 校验消息来源是否为配置的 TgChatID。
// chatID 可能是 float64/int64/json.Number/string（JSON 反序列化差异）。
func IsAuthorizedChat(state *app.State, chatID interface{}) bool {
	cfg := state.Config.Get()
	want := normalizeChatID(strings.TrimSpace(cfg.TgChatID))
	if want == "" {
		return false
	}
	got := normalizeChatID(chatIDToString(chatID))
	return got != "" && got == want
}

// ChatIDString 将 chat/user id 规范为字符串（供 handlers 频率限制等使用）。
func ChatIDString(chatID interface{}) string {
	return chatIDToString(chatID)
}

func chatIDToString(chatID interface{}) string {
	switch v := chatID.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		// JSON 数字默认 float64；用整型打印避免科学计数法
		return fmt.Sprintf("%.0f", v)
	case float32:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", chatID))
	}
}

func normalizeChatID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".0")
	return s
}

// DefaultAccountID 返回默认 OVH 账户 ID；无账户返回空串。
func DefaultAccountID(state *app.State) string {
	acc, ok := state.FindAccount("")
	if !ok {
		return ""
	}
	return acc.ID
}

// ParseOrderArgs 从命令参数解析 planCode / dc / qty / options。
// 约定与 free-form ParseOrderMessage 一致：
//   planCode [datacenter] [quantity] [options(逗号分隔)]
func ParseOrderArgs(args []string) *OrderInfo {
	if len(args) == 0 {
		return nil
	}
	// 复用 free-form 解析器
	return ParseOrderMessage(strings.Join(args, " "))
}
