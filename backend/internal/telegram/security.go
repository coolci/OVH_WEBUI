package telegram

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

const (
	// 文本 /buy 扇出上限（与 ParseOrderMessage 的 MaxOrderQuantity 对齐）
	MaxQuantityPerOrder  = 20
	MaxOrdersPerRequest  = 10
	MaxConfigsWhenNoOpts = 1
	MaxDCsWhenNoDC       = 1
	MaxQueueLen          = 500
)

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
			if t, ok := types.ParseTS(h.PurchaseTime); ok {
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
		RetryInterval: 0,
		RetryCount:    0,
		MaxRetries:    0,
		LastCheckTime: 0,
		FromTelegram:  true,
		Priority:      50,
	}
}
