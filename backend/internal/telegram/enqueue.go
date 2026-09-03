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
	if !CanEnqueue(state, 1) {
		return OrderResult{Success: false, Message: fmt.Sprintf("队列已满（上限 %d），请清理后再试", MaxQueueLen)}
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
	state.QueueMu.Lock()
	state.Queue = append(state.Queue, item)
	state.QueueMu.Unlock()
	if err := state.SaveQueue(); err != nil {
		// 落盘失败回滚内存，避免只在内存里可执行、重启却丢失
		state.QueueMu.Lock()
		kept := state.Queue[:0]
		for _, q := range state.Queue {
			if q.ID != item.ID {
				kept = append(kept, q)
			}
		}
		state.Queue = kept
		state.QueueMu.Unlock()
		state.Logger.Error("Telegram 入队落盘失败: "+err.Error(), "telegram")
		return OrderResult{Success: false, Message: "入队保存失败，请重试"}
	}
	state.Logger.Info(fmt.Sprintf("Telegram 受控入队: %s@%s account=%s opts=%v",
		planCode, datacenter, accountID, options), "telegram")
	return OrderResult{
		Success:       true,
		Message:       fmt.Sprintf("已加入队列: %s @ %s", planCode, strings.ToUpper(datacenter)),
		TotalOrders:   1,
		CreatedOrders: 1,
		ItemIDs:       []string{item.ID},
	}
}

// EnqueueTelegram 带进度绑定 / 极速抢 参数的入队。
func EnqueueTelegram(state *app.State, item types.QueueItem, requirePrice bool) OrderResult {
	if item.AccountID == "" {
		item.AccountID = DefaultAccountID(state)
	}
	item.PlanCode = strings.TrimSpace(item.PlanCode)
	item.Datacenter = strings.ToLower(strings.TrimSpace(item.Datacenter))
	if item.QuickOrder && item.RetryInterval == 0 {
		item.RetryInterval = 2
	}
	if item.QuickOrder && item.Priority == 0 {
		item.Priority = 100
	}
	if item.QuickOrder && item.MaxRetries == 0 {
		item.MaxRetries = 20
	}
	return enqueuePrepared(state, item, requirePrice)
}
