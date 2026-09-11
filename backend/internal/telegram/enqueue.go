package telegram

import (
	"fmt"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/price"
	"github.com/ovh-webui/server/internal/types"
)

// EnqueueSingle 受控入队：账户绑定 + 去重 + 可选询价 + 队列硬顶。
// 用于按钮一键下单与 /buy 单配置路径。
func EnqueueSingle(state *app.State, accountID, planCode, datacenter string, options []string, requirePrice bool) OrderResult {
	item := NewTelegramQueueItem(accountID, planCode, datacenter, options)
	return enqueuePrepared(state, item, requirePrice)
}

func enqueuePrepared(state *app.State, item types.QueueItem, requirePrice bool) OrderResult {
	if item.AccountID == "" {
		item.AccountID = DefaultAccountID(state)
	}
	item.PlanCode = strings.TrimSpace(item.PlanCode)
	item.Datacenter = strings.ToLower(strings.TrimSpace(item.Datacenter))
	accountID := item.AccountID
	planCode := item.PlanCode
	datacenter := item.Datacenter
	options := item.Options
	if accountID == "" {
		return OrderResult{Success: false, Message: "未配置任何 OVH 账户"}
	}
	if planCode == "" || datacenter == "" {
		return OrderResult{Success: false, Message: "缺少 planCode 或 datacenter"}
	}
	if HasActiveDuplicate(state, planCode, datacenter, options) {
		return OrderResult{Success: false, Message: "已存在相同配置的购买任务，请勿重复点击"}
	}
	if RecentSuccessDuplicate(state, planCode, datacenter, options) {
		return OrderResult{Success: false, Message: "刚刚已成功下过同配置订单，稍后再试"}
	}
	if len(options) == 0 {
		avail := catalog.CheckServerAvailabilityWithConfigs(state, planCode, accountID)
		for _, cfg := range avail {
			if st, ok := cfg.Datacenters[datacenter]; ok && st != "" && st != "unavailable" && st != "unknown" && len(cfg.Options) > 0 {
				options = append([]string{}, cfg.Options...)
				item.Options = options
				break
			}
		}
	}
	if requirePrice {
		pr := price.GetInternal(state, accountID, planCode, datacenter, options)
		if !pr.Success {
			err := pr.Error
			if err == "" {
				err = "价格校验失败"
			}
			return OrderResult{Success: false, Message: "价格校验失败：" + err}
		}
	}
	if item.ID == "" {
		item = NewTelegramQueueItem(accountID, planCode, datacenter, options)
	}
	// QuickOrder 只表示队列优先级,间隔一律走「抢购参数」里的新任务默认值。
	// 监控自动下单那条 2 秒间隔由 quick_order 入队时自己写上,这里不当成 Telegram 指令的默认。
	item.RetryInterval = types.ClampRetryInterval(item.RetryInterval, state.Config.RetryInterval())
	created, errMsg := appendAndSave(state, []types.QueueItem{item})
	if errMsg != "" {
		return OrderResult{Success: false, Message: errMsg}
	}
	state.Logger.Info(fmt.Sprintf("Telegram 受控入队: %s@%s account=%s opts=%v",
		planCode, datacenter, accountID, options), "telegram")
	ids := make([]string, 0, len(created))
	for _, it := range created {
		ids = append(ids, it.ID)
	}
	return OrderResult{
		Success:       true,
		Message:       fmt.Sprintf("已加入队列: %s @ %s", planCode, strings.ToUpper(datacenter)),
		TotalOrders:   1,
		CreatedOrders: len(created),
		ItemIDs:       ids,
	}
}

// appendAndSave 在同一把 QueueMu 下做容量检查 + 活跃去重,然后交给 EnqueueItems
// 入队并落盘。落盘失败由 EnqueueItems 整批撤回。RecentSuccessDuplicate 在锁外先滤一遍(HistoryMu)。
func appendAndSave(state *app.State, items []types.QueueItem) ([]types.QueueItem, string) {
	if len(items) == 0 {
		return nil, "没有可入队的任务"
	}
	filtered := make([]types.QueueItem, 0, len(items))
	for _, it := range items {
		if RecentSuccessDuplicate(state, it.PlanCode, it.Datacenter, it.Options) {
			continue
		}
		filtered = append(filtered, it)
	}
	if len(filtered) == 0 {
		return nil, "刚刚已成功下过同配置订单，请勿重复提交"
	}

	state.QueueMu.Lock()
	keep := make([]types.QueueItem, 0, len(filtered))
	for _, it := range filtered {
		fp := OptionsFingerprint(it.Options)
		dup := false
		for _, q := range state.Queue {
			if q.PlanCode == it.PlanCode && q.Datacenter == it.Datacenter &&
				(q.Status == "running" || q.Status == "pending" || q.Status == "paused") &&
				OptionsFingerprint(q.Options) == fp {
				dup = true
				break
			}
		}
		if !dup {
			keep = append(keep, it)
		}
	}
	if len(keep) == 0 {
		state.QueueMu.Unlock()
		return nil, "已存在相同配置的购买任务，请勿重复提交"
	}
	if len(state.Queue)+len(keep) > MaxQueueLen {
		n := len(keep)
		state.QueueMu.Unlock()
		return nil, fmt.Sprintf("队列容量不足（上限 %d），当前待添加 %d 个任务，请清理后再试", MaxQueueLen, n)
	}
	state.QueueMu.Unlock()

	if err := state.EnqueueItems(keep, false); err != nil {
		state.Logger.Error("Telegram 入队落盘失败: "+err.Error(), "telegram")
		return nil, "订单入队落盘失败，请重试"
	}
	return keep, ""
}

// EnqueueTelegram 带进度绑定 / 极速抢 参数的入队。
func EnqueueTelegram(state *app.State, item types.QueueItem, requirePrice bool) OrderResult {
	if item.AccountID == "" {
		item.AccountID = DefaultAccountID(state)
	}
	item.PlanCode = strings.TrimSpace(item.PlanCode)
	item.Datacenter = strings.ToLower(strings.TrimSpace(item.Datacenter))
	if item.QuickOrder && item.Priority == 0 {
		item.Priority = 100
	}
	if item.QuickOrder && item.MaxRetries == 0 {
		item.MaxRetries = 20
	}
	return enqueuePrepared(state, item, requirePrice)
}
