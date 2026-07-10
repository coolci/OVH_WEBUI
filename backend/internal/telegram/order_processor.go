package telegram

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/types"
)

// OrderResult 对应 Python: process_telegram_order 返回
type OrderResult struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	TotalOrders   int    `json:"total_orders"`
	CreatedOrders int    `json:"created_orders"`
}

// ProcessOrder 对应 Python: process_telegram_order
func ProcessOrder(state *app.State, planCode, datacenter string, quantity int, options []string) OrderResult {
	if quantity < 1 {
		quantity = 1
	}
	if !state.HasAnyAccount() {
		return OrderResult{Success: false, Message: "未配置任何 OVH 账户"}
	}
	// TG /buy 没有 account 概念,落默认账户的 subsidiary
	availByConfig := catalog.CheckServerAvailabilityWithConfigs(state, planCode, "")
	if len(availByConfig) == 0 {
		return OrderResult{Success: false, Message: "无法获取 " + planCode + " 的可用性信息"}
	}

	// 过滤配置
	type configEntry struct {
		key  string
		data *catalog.ConfigAvailability
	}
	configsToOrder := []configEntry{}
	if len(options) > 0 {
		for k, d := range availByConfig {
			// 检查用户 options 是否被该配置完全覆盖
			if subset(options, d.Options) {
				configsToOrder = append(configsToOrder, configEntry{key: k, data: d})
			}
		}
	} else {
		for k, d := range availByConfig {
			configsToOrder = append(configsToOrder, configEntry{key: k, data: d})
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
		}
	}

	totalOrders := len(configsToOrder) * len(dcsToOrder) * quantity
	ordersToCreate := []types.QueueItem{}
	for _, ce := range configsToOrder {
		configOptions := append([]string{}, ce.data.Options...)
		state.Logger.Info(fmt.Sprintf("[Telegram下单] 处理配置: memory=%s, storage=%s, options=%v (数量: %d)",
			ce.data.Memory, ce.data.Storage, configOptions, len(configOptions)), "telegram")
		if len(configOptions) == 0 {
			state.Logger.Warn(fmt.Sprintf("[Telegram下单] ⚠️ 配置选项为空！memory=%s, storage=%s",
				ce.data.Memory, ce.data.Storage), "telegram")
		}
		for _, dc := range dcsToOrder {
			if status, ok := ce.data.Datacenters[dc]; ok && (status == "unavailable" || status == "unknown") {
				continue
			}
			for i := 0; i < quantity; i++ {
				now := types.NowISO()
				item := types.QueueItem{
					ID:            uuid.NewString(),
					PlanCode:      planCode,
					Datacenter:    dc,
					Options:       append([]string{}, configOptions...),
					Status:        "running",
					CreatedAt:     now,
					UpdatedAt:     now,
					RetryInterval: 30,
					RetryCount:    0,
					LastCheckTime: 0,
					FromTelegram:  true,
				}
				ordersToCreate = append(ordersToCreate, item)
				state.Logger.Debug(fmt.Sprintf("[Telegram下单] 创建订单项: planCode=%s, datacenter=%s, options=%v (ID: %s)",
					planCode, dc, item.Options, item.ID[:8]), "telegram")
			}
		}
	}

	batchSize := 10
	totalBatches := (len(ordersToCreate) + batchSize - 1) / batchSize
	state.Logger.Info(fmt.Sprintf("开始并发创建订单: 总数=%d, 批次大小=%d, 总批次数=%d",
		len(ordersToCreate), batchSize, totalBatches), "telegram")
	created := 0
	var mu sync.Mutex
	for batchIdx := 0; batchIdx < totalBatches; batchIdx++ {
		start := batchIdx * batchSize
		end := start + batchSize
		if end > len(ordersToCreate) {
			end = len(ordersToCreate)
		}
		batch := ordersToCreate[start:end]
		var wg sync.WaitGroup
		for _, item := range batch {
			wg.Add(1)
			go func(it types.QueueItem) {
				defer wg.Done()
				state.QueueMu.Lock()
				state.Queue = append(state.Queue, it)
				state.QueueMu.Unlock()
				mu.Lock()
				created++
				mu.Unlock()
			}(item)
		}
		wg.Wait()
		state.Logger.Info(fmt.Sprintf("批次 %d/%d 完成: 本批次创建 %d 个订单", batchIdx+1, totalBatches, len(batch)), "telegram")
	}
	if created > 0 {
		_ = state.SaveQueue()
		state.Logger.Info(fmt.Sprintf("并发创建订单完成: 共创建 %d/%d 个订单", created, totalOrders), "telegram")
	}
	_ = time.Second
	return OrderResult{
		Success:       true,
		Message:       fmt.Sprintf("已创建 %d/%d 个订单", created, totalOrders),
		TotalOrders:   totalOrders,
		CreatedOrders: created,
	}
}

func subset(needle, haystack []string) bool {
	set := map[string]struct{}{}
	for _, h := range haystack {
		set[h] = struct{}{}
	}
	for _, n := range needle {
		if _, ok := set[n]; !ok {
			return false
		}
	}
	return true
}
