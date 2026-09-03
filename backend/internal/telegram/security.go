package telegram

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

const (
	// UpdateIDRetentionDays update_id 幂等表保留天数
	UpdateIDRetentionDays = 7
	// ButtonTTL 一键下单按钮有效期，与旧的 messageUUIDCacheTTL 保持一致
	ButtonTTL = 24 * time.Hour
	// RateLimitWindow / RateLimitMaxPerWindow 单 chat 的处理频率上限
	RateLimitWindow       = 10 * time.Second
	RateLimitMaxPerWindow = 8
	// 文本 /buy 扇出上限（本地增量）
	MaxQuantityPerOrder  = 3
	MaxOrdersPerRequest  = 10
	MaxConfigsWhenNoOpts = 1
	MaxDCsWhenNoDC       = 1
	MaxQueueLen          = 200
)

// IsAuthorizedActor 判断这条 update 的发送者是否是配置里那个 chat。
// 只认 config.TgChatID：
//   - 私聊：chat_id 等于配置值即可（Telegram 私聊的 chat_id 就是对方 user_id）；
//   - 群/超级群（chat_id 为负）：除 chat 匹配外，发送者必须在 TG_ALLOWED_USER_IDS 白名单里，
//     否则群里任何成员都能下单。
//
// 这层挡的是「secret 泄漏 / 兼容模式」下的越权，不是伪造来源 —— 伪造来源由 secret 挡。
func IsAuthorizedActor(state *app.State, chatID, userID interface{}) bool {
	want := normalizeID(state.Config.Get().TgChatID)
	if want == "" {
		return false
	}
	gotChat := normalizeID(idToString(chatID))
	gotUser := normalizeID(idToString(userID))

	if gotChat != "" && gotChat == want {
		if strings.HasPrefix(gotChat, "-") {
			allow := strings.TrimSpace(os.Getenv("TG_ALLOWED_USER_IDS"))
			if allow == "" {
				return false
			}
			return idInCSV(gotUser, allow)
		}
		return true
	}
	// 兼容：配置里填的是 user id，私聊时 chat_id 与之相等
	if gotUser != "" && gotUser == want && (gotChat == "" || gotChat == gotUser) {
		return true
	}
	return false
}

func idInCSV(id, csv string) bool {
	if id == "" {
		return false
	}
	for _, p := range strings.Split(csv, ",") {
		if normalizeID(strings.TrimSpace(p)) == id {
			return true
		}
	}
	return false
}

// normalizeID 去掉 @ 前缀和小数点尾巴（JSON 数字解出来是 float64）
func normalizeID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	if i := strings.Index(s, "."); i >= 0 {
		s = s[:i]
	}
	return s
}

// idToString 把 chat_id / user_id（float64 / json.Number / string）统一成字符串
func idToString(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case int:
		return fmt.Sprintf("%d", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// ChatIDString 导出给 handler 做频率限制 key
func ChatIDString(v interface{}) string { return normalizeID(idToString(v)) }

// --- 进程内频率限制 ---

type rateBucket struct {
	windowStart time.Time
	count       int
}

var (
	rateMu   sync.Mutex
	rateByID = map[string]*rateBucket{}
)

// AllowRate 按 chat（取不到则 user）维度限流，返回是否放行。
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

func ClampQuantity(q int) int {
	if q < 1 {
		return 1
	}
	if q > MaxQuantityPerOrder {
		return MaxQuantityPerOrder
	}
	return q
}

func QueueLen(state *app.State) int {
	state.QueueMu.Lock()
	defer state.QueueMu.Unlock()
	return len(state.Queue)
}

func CanEnqueue(state *app.State, n int) bool {
	return QueueLen(state)+n <= MaxQueueLen
}

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
