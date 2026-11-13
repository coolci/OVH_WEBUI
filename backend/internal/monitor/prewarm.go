package monitor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/purchase"
)

// prewarmSem 限制全进程同时装配预热车的并发数，避免批量开启自动下单订阅时瞬时打爆 OVH 限流。
var prewarmSem = make(chan struct{}, 4)

type prewarmCoordinator struct {
	mu           sync.Mutex
	inFlight     map[string]bool      // key: accountID|planCode|dc|optionsKey
	lastAttempt  map[string]time.Time // key -> last attempt time
	failedCounts map[string]int       // key -> consecutive failures
}

var globalPrewarmCoord = &prewarmCoordinator{
	inFlight:     map[string]bool{},
	lastAttempt:  map[string]time.Time{},
	failedCounts: map[string]int{},
}

func prewarmCoordKey(accountID, planCode, datacenter string, options []string, coupon, duration string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s", accountID, planCode, strings.ToLower(datacenter), OptionsKey(options), strings.TrimSpace(coupon), duration)
}

// ResetPrewarmCoordinator 重置预热协调器状态（供测试与重置使用）
func ResetPrewarmCoordinator() {
	globalPrewarmCoord.mu.Lock()
	defer globalPrewarmCoord.mu.Unlock()
	globalPrewarmCoord.inFlight = map[string]bool{}
	globalPrewarmCoord.lastAttempt = map[string]time.Time{}
	globalPrewarmCoord.failedCounts = map[string]int{}
}

func (m *Monitor) shouldSkipPrewarmForAccount(accountID, planCode string) bool {
	if m.state == nil {
		return true
	}
	acc, ok := m.state.FindAccount(accountID)
	if !ok {
		return true
	}
	subsidiary := catalog.SubsidiaryOfAccount(acc)
	if m.isPlanInCooldown(acc.ID, subsidiary, planCode) {
		m.logPlanCooldownSkip(acc.ID, subsidiary, planCode, "预热")
		return true
	}
	if catalog.IsEcoPlanCode(planCode) {
		if _, err := catalog.AddonFamiliesForPlan(m.state, acc.ID, planCode); err == nil {
			m.observePlanCatalogStatus(acc.ID, subsidiary, planCode, true, false)
		} else if errors.Is(err, catalog.ErrPlanNotInCatalog) {
			m.markPrewarmCatalogMissing(acc.ID, subsidiary, planCode)
			return true
		}
	}
	return false
}

// TriggerPrewarmForSubscription 为指定订阅触发预热装车。
// 仅对 AutoOrder=true 且指定了 AutoOrderAccountID 的有效订阅生效。
func (m *Monitor) TriggerPrewarmForSubscription(sub *Subscription) {
	if sub == nil {
		return
	}
	cfg := sub.checkConfig()
	accounts := NormalizeOrderAccounts(cfg.AutoOrderAccountID, cfg.AutoOrderAccountIDs)
	if cfg.Paused || !cfg.AutoOrder || len(accounts) == 0 {
		return
	}
	planCode := sub.PlanCode
	options := cfg.Options
	dcs := cfg.Datacenters

	if len(dcs) == 0 {
		return
	}

	activeAccounts := make([]string, 0, len(accounts))
	for _, accountID := range accounts {
		if m.shouldSkipPrewarmForAccount(accountID, planCode) {
			continue
		}
		activeAccounts = append(activeAccounts, accountID)
	}
	if len(activeAccounts) == 0 {
		return
	}

	for _, dc := range dcs {
		dcTrimmed := strings.TrimSpace(dc)
		if dcTrimmed == "" {
			continue
		}
		for _, accountID := range activeAccounts {
			go m.assemblePrewarmedCartAsync(accountID, planCode, dcTrimmed, options, "", cfg.Duration)
		}
	}
}

// EnsureAllPrewarmedCarts 遍历所有处于运行中的自动下单订阅，确保其对应的目标机房在 OVH 侧均有预热车待命。
func (m *Monitor) EnsureAllPrewarmedCarts() {
	m.subsMu.Lock()
	subsCopy := make([]*Subscription, len(m.subscriptions))
	copy(subsCopy, m.subscriptions)
	m.subsMu.Unlock()

	for _, sub := range subsCopy {
		m.TriggerPrewarmForSubscription(sub)
	}
}

func (m *Monitor) assemblePrewarmedCartAsync(accountID, planCode, datacenter string, options []string, coupon, duration string) {
	if m.state == nil || m.state.OVH == nil {
		return
	}
	acc, ok := m.state.FindAccount(accountID)
	if !ok {
		return
	}
	subsidiary := catalog.SubsidiaryOfAccount(acc)
	if m.isPlanInCooldown(acc.ID, subsidiary, planCode) {
		m.logPlanCooldownSkip(acc.ID, subsidiary, planCode, "预热")
		return
	}

	key := prewarmCoordKey(accountID, planCode, datacenter, options, coupon, duration)

	globalPrewarmCoord.mu.Lock()
	if globalPrewarmCoord.inFlight[key] {
		globalPrewarmCoord.mu.Unlock()
		return
	}
	// 如果近期有失败，按指数退避冷却
	if last, ok := globalPrewarmCoord.lastAttempt[key]; ok {
		failCount := globalPrewarmCoord.failedCounts[key]
		if failCount > 0 {
			shift := failCount
			if shift > 5 {
				shift = 5
			}
			cooldown := time.Duration(1<<shift) * 30 * time.Second
			if time.Since(last) < cooldown {
				globalPrewarmCoord.mu.Unlock()
				return
			}
		}
	}
	// 检查预热池中是否已有有效车
	if purchase.HasPrewarmedCart(accountID, planCode, datacenter, options, coupon, duration) {
		globalPrewarmCoord.mu.Unlock()
		return
	}

	globalPrewarmCoord.inFlight[key] = true
	globalPrewarmCoord.lastAttempt[key] = time.Now()
	globalPrewarmCoord.mu.Unlock()

	defer func() {
		globalPrewarmCoord.mu.Lock()
		delete(globalPrewarmCoord.inFlight, key)
		globalPrewarmCoord.mu.Unlock()
	}()

	prewarmSem <- struct{}{}
	defer func() { <-prewarmSem }()

	if m.isPlanInCooldown(acc.ID, subsidiary, planCode) {
		m.logPlanCooldownSkip(acc.ID, subsidiary, planCode, "预热")
		return
	}
	if purchase.HasPrewarmedCart(accountID, planCode, datacenter, options, coupon, duration) {
		return
	}

	m.state.Logger.Info(fmt.Sprintf("[monitor/prewarm] 开始为订阅 %s@%s 预热购物车 (账户: %s)...",
		planCode, datacenter, accountID), "monitor")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cart, err := purchase.AssemblePrewarmedCart(ctx, m.state, accountID, planCode, datacenter, options, coupon, duration)
	if err != nil {
		globalPrewarmCoord.mu.Lock()
		globalPrewarmCoord.failedCounts[key]++
		globalPrewarmCoord.lastAttempt[key] = time.Now()
		globalPrewarmCoord.mu.Unlock()
		m.recordPlanVerifyFailure(acc.ID, subsidiary, planCode, false, err.Error())
		kind, friendly := ClassifyPriceCheckError(err.Error())
		if kind == ErrorKindPreSale {
			m.state.Logger.Info(fmt.Sprintf("[monitor/prewarm] %s@%s 暂未开放加车（%s，待放行自动预热）", planCode, datacenter, friendly), "monitor")
		} else if kind == ErrorKindRegionBlocked {
			m.state.Logger.Info(fmt.Sprintf("[monitor/prewarm] %s@%s 跨区配置受限: %v", planCode, datacenter, err), "monitor")
		} else {
			m.state.Logger.Warn(fmt.Sprintf("[monitor/prewarm] 预热装车 %s@%s 失败: %v", planCode, datacenter, err), "monitor")
		}
		return
	}

	globalPrewarmCoord.mu.Lock()
	globalPrewarmCoord.failedCounts[key] = 0
	globalPrewarmCoord.mu.Unlock()
	m.recordPlanVerifySuccess(acc.ID, subsidiary, planCode)

	m.state.Logger.Info(fmt.Sprintf("[monitor/prewarm] ✅ 预热装车就绪！CartID: %s (%s@%s)", cart.CartID, planCode, datacenter), "monitor")
}
