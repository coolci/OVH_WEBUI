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
	"start":    "显示帮助与可用命令",
	"help":     "显示帮助与可用命令",
	"watch":    "盯补货抢购: /watch <planCode> [dc...] [x数量]",
	"w":        "watch 的别名",
	"unwatch":  "取消盯补货: /unwatch <planCode>",
	"uw":       "unwatch 的别名",
	"stock":    "查询库存: /stock <planCode>",
	"queue":    "加入队列: /queue <planCode> [dc] [qty] [options]",
	"buy":      "快速下单: /buy <planCode> [dc] [qty] [options]",
	"tasks":    "查看抢购任务队列: /tasks",
	"accounts": "查看与切换 OVH 账户: /accounts",
	"monitor":  "添加监控: /monitor <planCode> [dc...]",
	"price":    "查询价格: /price <planCode> <dc>",
	"interval": "查看或修改默认重试间隔: /interval [秒]",
	"iv":       "interval 的别名",
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
⚡ OVH CLOUD TERMINAL · 智能控制台 ⚡
━━━━━━━━━━━━━━━━━━━━━━━━━
📊 运行状态: 🟢 极速引擎在线 ｜ 毫秒级锁单
🌐 节点支持: GRA · RBX · SBG · BHS · WAW · FRA · LON...

🎯 常用核心指令:
• /buy <型号> [机房] [数量]
  └ ⚡ 快速下单 (有货毫秒直抢，缺货自动排队)
• /stock <型号>
  └ 📦 全球机房库存与多硬件规格穿透查询
• /price <型号> <机房>
  └ 💰 官方实时结算价与税费核算
• /monitor <型号> [机房...]
  └ 📡 毫秒级补货监控，上架即时 Telegram 警报
• /tasks
  └ 📋 实时并发抢购任务队列与一键中止
• /accounts
  └ 👤 多区 OVH 账户矩阵状态与无缝切换
• /interval [秒]
  └ ⏱️ 查看或设置新建任务全局重试间隔

💡 极速免命令模式:
直接发送「型号 机房 数量」，例如：24ska01 gra 1

📌 操作指引:
点击下方快捷中枢按钮，免打字直达各项功能 👇
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
