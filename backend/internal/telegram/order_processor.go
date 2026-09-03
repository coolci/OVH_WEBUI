package telegram

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// OrderResult Telegram 下单结果
type OrderResult struct {
	Success       bool     `json:"success"`
	Message       string   `json:"message"`
	TotalOrders   int      `json:"total_orders"`
	CreatedOrders int      `json:"created_orders"`
	ItemIDs       []string `json:"item_ids,omitempty"`
}

// accountRegionLabel 给用户看的账户归属描述:"名字(子公司 US / US 区)"。
// TG 是唯一没有账户选择器的入口,下单落在哪个账户、哪个区,必须在回复里说清楚。
func accountRegionLabel(acc types.OVHAccount) (string, string) {
	sub := strings.ToUpper(strings.TrimSpace(acc.Zone))
	if sub == "" {
		sub = ovh.DefaultSubsidiaryForEndpoint(acc.Endpoint)
	}
	return sub, fmt.Sprintf("%s(子公司 %s / %s 区)", acc.Name, sub, ovh.SubsidiaryRegion(sub))
}

// emptyAvailabilityReason 解释"可用性查不到"到底是哪一种情况。
//
// 为什么需要它:EU / US / CA 三个站点的目录是彼此独立的系统,同一台机器在不同区
// 是不同的 planCode(实测 FR 与 CA 的 eco 目录 planCode 完全一致 99/99,
// 而 US 的 143 个里只有 28 个和 FR 重合;美区机型带 -us 后缀)。
// 拿欧区 planCode 打美区的 /dedicated/server/datacenter/availabilities,
// OVH 返回的是 HTTP 200 + 空数组 —— 不是错误,就是"没有这个机型"。
// 只回一句"无法获取可用性信息"会让用户以为是抢购程序坏了,而真正该做的是换本区的 planCode。
//
// 用目录判断而不是猜后缀:后缀规则是观察出来的,目录才是权威。
// 目录走 catalog 的 2 小时缓存,不额外消耗账户配额。
// 注意 eco 目录只覆盖 Kimsufi/Rise/Advance 这一档,所以"目录里没有"只用来补充说明,
// 不作为拦截条件 —— 否则会误杀不在 eco 目录里但确实能买的机型。
func emptyAvailabilityReason(state *app.State, accountID, planCode string, acc types.OVHAccount) string {
	sub, label := accountRegionLabel(acc)
	base := fmt.Sprintf("无法获取 %s 的可用性信息(本次使用账户 %s)", planCode, label)
	if _, err := catalog.AddonFamiliesForPlan(state, accountID, planCode); err != nil {
		if strings.Contains(err.Error(), "目录里没有 planCode") {
			return base + fmt.Sprintf(
				"\n\n%s 的目录里没有 %s。OVH 的 EU / US / CA 是三套彼此独立的系统,同一台机器在不同区是不同的 planCode"+
					"(美区机型通常带 -us 后缀),用别区的 planCode 查库存 OVH 只会返回空数组而不是报错。"+
					"\n请改用 %s 区的 planCode,或把 /buy 落到对应区的账户上。", sub, planCode, ovh.SubsidiaryRegion(sub))
		}
		// 目录本身没拉到(网络/429):不能据此断言 planCode 不存在,只说明少了一条判据
		return base + fmt.Sprintf("\n\n(顺带一提:%s 的目录本次也没拉到 —— %s,所以无法确认这个 planCode 是不是属于别的区)", sub, err.Error())
	}
	// planCode 在本区目录里确实存在 → 是真的全区无货/已下架,不是区域搞错了
	return base + fmt.Sprintf("\n\n%s 的目录里有 %s,所以不是区域搞错了,而是这个机型当前在所有机房都没有可售配置。", sub, planCode)
}

func ProcessOrder(state *app.State, planCode, datacenter string, quantity int, options []string) OrderResult {
	if quantity < 1 {
		quantity = 1
	}
	// TG /buy 没有账户维度,落默认账户 —— 但必须在这里就把它解析成具体 ID 并写进队列项。
	// 以前 QueueItem.AccountID 留空,下单时才由 purchase 现取默认账户:
	// 中间只要有人改过默认账户(或删掉它),这一单就会用另一个账户、另一个区的凭据去下,
	// 而可用性/目录判断用的还是此刻这个账户的子公司,两边对不上。
	acc, ok := state.FindAccount("")
	if !ok {
		return OrderResult{Success: false, Message: "未配置任何 OVH 账户"}
	}
	accountID := acc.ID
	sub, accLabel := accountRegionLabel(acc)

	availByConfig := catalog.CheckServerAvailabilityWithConfigs(state, planCode, accountID)
	if len(availByConfig) == 0 {
		return OrderResult{Success: false, Message: emptyAvailabilityReason(state, accountID, planCode, acc)}
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

	// 如果未指定具体选项，默认使用第 1 个配置，避免把所有内存/硬盘组合都同时建一遍单
	if len(options) == 0 && len(configsToOrder) > 1 {
		configsToOrder = configsToOrder[:1]
	}

	allKnownDCs := map[string]struct{}{}
	availableDCs := map[string]struct{}{}
	for _, e := range configsToOrder {
		for dc, status := range e.data.Datacenters {
			allKnownDCs[dc] = struct{}{}
			if catalog.IsAvailableForOrder(status) {
				availableDCs[dc] = struct{}{}
			}
		}
	}

	dcsToOrder := []string{}
	targetInStock := false
	if datacenter != "" {
		apiDC := ovh.ConvertDisplayDCToAPIDC(datacenter)
		dcsToOrder = append(dcsToOrder, apiDC)
		if _, ok := availableDCs[apiDC]; ok {
			targetInStock = true
		}
	} else {
		// 未显式指定机房：优先取有现货的机房；若全区缺货，则将已知机房加入抢购队列
		if len(availableDCs) > 0 {
			targetInStock = true
			for dc := range availableDCs {
				dcsToOrder = append(dcsToOrder, dc)
			}
		} else {
			for dc := range allKnownDCs {
				dcsToOrder = append(dcsToOrder, dc)
			}
			if len(dcsToOrder) == 0 {
				for _, d := range StandardDCs {
					dcsToOrder = append(dcsToOrder, ovh.ConvertDisplayDCToAPIDC(d))
				}
			}
		}
	}

	totalOrders := len(configsToOrder) * len(dcsToOrder) * quantity
	ordersToCreate := []types.QueueItem{}
	state.Logger.Info(fmt.Sprintf("[Telegram下单] 账户=%s, 子公司=%s, planCode=%s, 机房=%v, 现货状态=%v",
		accLabel, sub, planCode, dcsToOrder, targetInStock), "telegram")

	for _, ce := range configsToOrder {
		configOptions := append([]string{}, ce.data.Options...)
		for _, dc := range dcsToOrder {
			for i := 0; i < quantity; i++ {
				now := types.NowISO()
				item := types.QueueItem{
					ID:            uuid.NewString(),
					AccountID:     accountID,
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
	msgSuffix := "（当前有现货，系统将立即尝试结账）"
	if !targetInStock {
		msgSuffix = "（目标机房当前缺货，已加入抢购队列挂机，放货瞬间秒级开抢）"
	}
	return OrderResult{
		Success:       true,
		Message:       fmt.Sprintf("已创建 %d/%d 个任务(账户 %s)%s", created, totalOrders, accLabel, msgSuffix),
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
