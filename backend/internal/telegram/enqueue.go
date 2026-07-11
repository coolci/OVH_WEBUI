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
	planCode = strings.TrimSpace(planCode)
	datacenter = strings.ToLower(strings.TrimSpace(datacenter))
	if accountID == "" {
		accountID = DefaultAccountID(state)
	}
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

	// 无 options 时尝试从可用性补全
	if len(options) == 0 {
		avail := catalog.CheckServerAvailabilityWithConfigs(state, planCode, accountID)
		for _, cfg := range avail {
			if st, ok := cfg.Datacenters[datacenter]; ok && st != "" && st != "unavailable" && st != "unknown" && len(cfg.Options) > 0 {
				options = append([]string{}, cfg.Options...)
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

	item := NewTelegramQueueItem(accountID, planCode, datacenter, options)
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
	}
}

// ProcessOrder 重写：带数量/扇出上限、去重、账户绑定、队列硬顶。
// 未指定机房时仅取 1 个可用机房；未指定 options 时仅取 1 套配置（防笛卡尔积爆炸）。
func ProcessOrder(state *app.State, planCode, datacenter string, quantity int, options []string) OrderResult {
	quantity = ClampQuantity(quantity)
	planCode = strings.TrimSpace(planCode)
	datacenter = strings.ToLower(strings.TrimSpace(datacenter))
	if planCode == "" {
		return OrderResult{Success: false, Message: "缺少 planCode"}
	}
	if !state.HasAnyAccount() {
		return OrderResult{Success: false, Message: "未配置任何 OVH 账户"}
	}
	accountID := DefaultAccountID(state)
	if accountID == "" {
		return OrderResult{Success: false, Message: "未配置任何 OVH 账户"}
	}

	// 生产建议：强制指定机房，避免「全机房 × 全配置」扇出
	// 若未指定：仅允许最多 MaxDCsWhenNoDC 个机房
	availByConfig := catalog.CheckServerAvailabilityWithConfigs(state, planCode, accountID)
	if len(availByConfig) == 0 {
		return OrderResult{Success: false, Message: "无法获取 " + planCode + " 的可用性信息"}
	}

	type configEntry struct {
		key  string
		data *catalog.ConfigAvailability
	}
	configsToOrder := []configEntry{}
	if len(options) > 0 {
		for k, d := range availByConfig {
			if subset(options, d.Options) {
				configsToOrder = append(configsToOrder, configEntry{key: k, data: d})
			}
		}
	} else {
		for k, d := range availByConfig {
			configsToOrder = append(configsToOrder, configEntry{key: k, data: d})
		}
		// 限制配置扇出
		if len(configsToOrder) > MaxConfigsWhenNoOpts {
			configsToOrder = configsToOrder[:MaxConfigsWhenNoOpts]
		}
	}
	if len(configsToOrder) == 0 {
		return OrderResult{Success: false, Message: fmt.Sprintf("未找到匹配的配置（指定选项: %v）", options)}
	}

	availableDCs := map[string]struct{}{}
	for _, e := range configsToOrder {
		for dc, status := range e.data.Datacenters {
			if status != "" && status != "unavailable" && status != "unknown" {
				availableDCs[dc] = struct{}{}
			}
		}
	}
	if len(availableDCs) == 0 {
		return OrderResult{Success: false, Message: "所有配置在所有机房都无货"}
	}

	dcsToOrder := []string{}
	if datacenter != "" {
		if _, ok := availableDCs[datacenter]; !ok {
			return OrderResult{Success: false, Message: "指定机房 " + datacenter + " 无货"}
		}
		dcsToOrder = append(dcsToOrder, datacenter)
	} else {
		for dc := range availableDCs {
			dcsToOrder = append(dcsToOrder, dc)
			if len(dcsToOrder) >= MaxDCsWhenNoDC {
				break
			}
		}
		state.Logger.Info(fmt.Sprintf("[Telegram下单] 未指定机房，限制为 %d 个: %v", len(dcsToOrder), dcsToOrder), "telegram")
	}

	// 预估并硬顶
	planned := len(configsToOrder) * len(dcsToOrder) * quantity
	if planned > MaxOrdersPerRequest {
		return OrderResult{Success: false, Message: fmt.Sprintf(
			"本次将创建 %d 个任务，超过单次上限 %d。请指定机房/配置或减小数量（≤%d）",
			planned, MaxOrdersPerRequest, MaxQuantityPerOrder)}
	}
	if !CanEnqueue(state, planned) {
		return OrderResult{Success: false, Message: fmt.Sprintf("队列空间不足（当前上限 %d）", MaxQueueLen)}
	}

	ordersToCreate := []types.QueueItem{}
	skippedDup := 0
	for _, ce := range configsToOrder {
		configOptions := append([]string{}, ce.data.Options...)
		for _, dc := range dcsToOrder {
			if status, ok := ce.data.Datacenters[dc]; ok && (status == "unavailable" || status == "unknown") {
				continue
			}
			// 队列中已有同配置活跃任务时整组跳过（不与 quantity 混用）
			if HasActiveDuplicate(state, planCode, dc, configOptions) {
				skippedDup += quantity
				continue
			}
			// quantity 表示同配置入队条数（每条独立抢购任务），允许同批重复
			for i := 0; i < quantity; i++ {
				item := NewTelegramQueueItem(accountID, planCode, dc, configOptions)
				ordersToCreate = append(ordersToCreate, item)
			}
		}
	}

	if len(ordersToCreate) == 0 {
		msg := "没有可创建的任务"
		if skippedDup > 0 {
			msg = "已存在相同配置的活跃任务，跳过重复入队"
		}
		return OrderResult{Success: false, Message: msg}
	}

	// 串行入队 + 一次落盘
	state.QueueMu.Lock()
	state.Queue = append(state.Queue, ordersToCreate...)
	state.QueueMu.Unlock()
	if err := state.SaveQueue(); err != nil {
		idSet := map[string]struct{}{}
		for _, it := range ordersToCreate {
			idSet[it.ID] = struct{}{}
		}
		state.QueueMu.Lock()
		kept := state.Queue[:0]
		for _, q := range state.Queue {
			if _, drop := idSet[q.ID]; !drop {
				kept = append(kept, q)
			}
		}
		state.Queue = kept
		state.QueueMu.Unlock()
		state.Logger.Error("Telegram 批量入队落盘失败: "+err.Error(), "telegram")
		return OrderResult{Success: false, Message: "入队保存失败，请重试"}
	}
	created := len(ordersToCreate)
	state.Logger.Info(fmt.Sprintf("Telegram 受控批量入队: %d 个 (skip_dup=%d)", created, skippedDup), "telegram")
	return OrderResult{
		Success:       true,
		Message:       fmt.Sprintf("已创建 %d 个订单", created),
		TotalOrders:   created,
		CreatedOrders: created,
	}
}
