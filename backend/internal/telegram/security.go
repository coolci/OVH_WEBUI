package telegram

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// 生产安全默认值
const (
	MaxQuantityPerOrder   = 3   // /buy /queue 单次数量上限
	MaxOrdersPerRequest   = 10  // 单次请求最多创建的队列项
	MaxConfigsWhenNoOpts  = 1   // 未指定 options 时最多取 N 套配置
	MaxDCsWhenNoDC        = 1   // 未指定机房时最多取 N 个机房（避免全机房扇出）
	MaxQueueLen           = 200 // 全局队列长度硬顶
	MaxTelegramBodyBytes  = 64 * 1024
	UpdateIDRetentionDays = 7
	RateLimitWindow       = 10 * time.Second
	RateLimitMaxPerWindow = 8 // 每 chat 每窗口最多处理次数
)

// SecretTokenHeader Telegram 在设置 secret_token 后会带上此请求头。
const SecretTokenHeader = "X-Telegram-Bot-Api-Secret-Token"

// EnsureWebhookSecret 确保 config 中有 webhook secret；优先环境变量 TG_WEBHOOK_SECRET。
// 若都无则生成 32 字节随机 secret 并落盘。返回 secret。
func EnsureWebhookSecret(state *app.State) (string, error) {
	if env := strings.TrimSpace(os.Getenv("TG_WEBHOOK_SECRET")); env != "" {
		cfg := state.Config.Get()
		if cfg.TgWebhookSecret != env {
			cfg.TgWebhookSecret = env
			if err := state.Config.Set(cfg); err != nil {
				// 环境变量仍可用于校验，落盘失败不阻断
				state.Logger.Warn("写入 TG_WEBHOOK_SECRET 到 config 失败: "+err.Error(), "telegram")
			}
		}
		return env, nil
	}
	cfg := state.Config.Get()
	if s := strings.TrimSpace(cfg.TgWebhookSecret); s != "" {
		return s, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate webhook secret: %w", err)
	}
	secret := hex.EncodeToString(b)
	cfg.TgWebhookSecret = secret
	if err := state.Config.Set(cfg); err != nil {
		return "", err
	}
	state.Logger.Info("已生成 Telegram webhook secret_token（已落盘）", "telegram")
	return secret, nil
}

// ValidateWebhookSecret 校验请求头中的 secret。
// 若未配置 secret：拒绝（生产强制），除非环境变量 TG_WEBHOOK_SECRET_OPTIONAL=true（仅开发）。
func ValidateWebhookSecret(state *app.State, headerValue string) bool {
	want, err := EnsureWebhookSecret(state)
	if err != nil || want == "" {
		if strings.EqualFold(os.Getenv("TG_WEBHOOK_SECRET_OPTIONAL"), "true") {
			return true
		}
		return false
	}
	got := strings.TrimSpace(headerValue)
	if got == "" {
		return false
	}
	// 常量时间比较，避免时序侧信道
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// IsAuthorizedActor 授权规则（生产收紧）：
//  1. 配置的 TgChatID 必须匹配 chat_id（私聊或指定群）；或
//  2. 兼容：TgChatID 填的是 user id，且当前 user_id 匹配（私聊）。
// 群聊（chat_id < 0）时：必须 chat 匹配，且发送者 user_id 也必须等于 |TgChatID|
// 若 TgChatID 本身是负数群 ID，则仅允许该群（任意成员——建议改用私聊 Chat ID）。
func IsAuthorizedActor(state *app.State, chatID, userID interface{}) bool {
	cfg := state.Config.Get()
	want := normalizeChatID(strings.TrimSpace(cfg.TgChatID))
	if want == "" {
		return false
	}
	gotChat := normalizeChatID(chatIDToString(chatID))
	gotUser := normalizeChatID(chatIDToString(userID))

	// 主路径：chat 完全匹配
	if gotChat != "" && gotChat == want {
		// 群/超级群（chat id 为负）：必须配置 TG_ALLOWED_USER_IDS 白名单，禁止群内任意成员下单
		if strings.HasPrefix(gotChat, "-") {
			allow := strings.TrimSpace(os.Getenv("TG_ALLOWED_USER_IDS"))
			if allow == "" {
				return false
			}
			return userInList(gotUser, allow)
		}
		// 私聊：chat 匹配即可
		return true
	}

	// 兼容：配置 user id，私聊中 chat_id 通常等于 user_id
	if gotUser != "" && gotUser == want {
		// 仅当 chat 也是同一 user（私聊）或 chat 为空时
		if gotChat == "" || gotChat == want || gotChat == gotUser {
			return true
		}
	}
	return false
}

func userInList(userID, csv string) bool {
	if userID == "" {
		return false
	}
	for _, p := range strings.Split(csv, ",") {
		if normalizeChatID(strings.TrimSpace(p)) == userID {
			return true
		}
	}
	return false
}

// --- 简易频率限制（进程内）---

type rateBucket struct {
	windowStart time.Time
	count       int
}

var (
	rateMu   sync.Mutex
	rateByID = map[string]*rateBucket{}
)

// AllowRate 返回是否允许继续处理（按 chat 或 user 维度）。
func AllowRate(id string) bool {
	if id == "" {
		id = "unknown"
	}
	now := time.Now()
	rateMu.Lock()
	defer rateMu.Unlock()
	b, ok := rateByID[id]
	if !ok || now.Sub(b.windowStart) > RateLimitWindow {
		rateByID[id] = &rateBucket{windowStart: now, count: 1}
		return true
	}
	if b.count >= RateLimitMaxPerWindow {
		return false
	}
	b.count++
	return true
}

// ClampQuantity 限制数量到 [1, MaxQuantityPerOrder]
func ClampQuantity(q int) int {
	if q < 1 {
		return 1
	}
	if q > MaxQuantityPerOrder {
		return MaxQuantityPerOrder
	}
	return q
}

// QueueLen 当前队列长度
func QueueLen(state *app.State) int {
	state.QueueMu.Lock()
	defer state.QueueMu.Unlock()
	return len(state.Queue)
}

// CanEnqueue 是否还能入队 n 条
func CanEnqueue(state *app.State, n int) bool {
	return QueueLen(state)+n <= MaxQueueLen
}

// OptionsFingerprint 与 quick_order 一致的 options 指纹
func OptionsFingerprint(opts []string) string {
	if len(opts) == 0 {
		return ""
	}
	uniq := map[string]struct{}{}
	for _, o := range opts {
		s := strings.TrimSpace(o)
		if s != "" {
			uniq[s] = struct{}{}
		}
	}
	list := make([]string, 0, len(uniq))
	for s := range uniq {
		list = append(list, s)
	}
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j-1] > list[j]; j-- {
			list[j-1], list[j] = list[j], list[j-1]
		}
	}
	return strings.Join(list, "|")
}

// HasActiveDuplicate 是否已有相同 plan@dc@options 的活跃任务
func HasActiveDuplicate(state *app.State, planCode, datacenter string, options []string) bool {
	fp := OptionsFingerprint(options)
	state.QueueMu.Lock()
	defer state.QueueMu.Unlock()
	for _, it := range state.Queue {
		if it.PlanCode == planCode && it.Datacenter == datacenter &&
			(it.Status == "running" || it.Status == "pending" || it.Status == "paused") &&
			OptionsFingerprint(it.Options) == fp {
			return true
		}
	}
	return false
}

// RecentSuccessDuplicate 近 120s 内是否刚成功同配置
func RecentSuccessDuplicate(state *app.State, planCode, datacenter string, options []string) bool {
	fp := OptionsFingerprint(options)
	nowTS := time.Now().Unix()
	state.HistoryMu.Lock()
	defer state.HistoryMu.Unlock()
	for i := len(state.History) - 1; i >= 0; i-- {
		h := state.History[i]
		if h.PlanCode == planCode && h.Datacenter == datacenter && h.Status == "success" &&
			OptionsFingerprint(h.Options) == fp {
			if t, err := time.Parse(time.RFC3339Nano, h.PurchaseTime); err == nil {
				if nowTS-t.Unix() < 120 {
					return true
				}
			}
		}
	}
	return false
}

// NewTelegramQueueItem 构造带 AccountID 的 Telegram 队列项。
// MaxRetries=0 表示抢购可持续重试直至手动删除（队列处理器仅在 MaxRetries>0 时终止）。
func NewTelegramQueueItem(accountID, planCode, datacenter string, options []string) types.QueueItem {
	now := types.NowISO()
	return types.QueueItem{
		ID:            uuid.NewString(),
		AccountID:     accountID,
		PlanCode:      planCode,
		Datacenter:    datacenter,
		Options:       append([]string{}, options...),
		Status:        "running",
		CreatedAt:     now,
		UpdatedAt:     now,
		RetryInterval: 30,
		RetryCount:    0,
		MaxRetries:    0,
		LastCheckTime: 0,
		FromTelegram:  true,
		Priority:      50,
	}
}
