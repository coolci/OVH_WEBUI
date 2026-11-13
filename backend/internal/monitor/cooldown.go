package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/ovh-webui/server/internal/catalog"
)

type planCooldownReason string

const (
	cooldownNone         planCooldownReason = ""
	cooldownPlanNotFound planCooldownReason = "plan_not_found"
	cooldownCatalogFail  planCooldownReason = "catalog_missing_fail"
	cooldownUpstream504  planCooldownReason = "upstream_504"
	cooldownCatalog429   planCooldownReason = "catalog_missing_429"
)

const (
	planNotFoundBaseCooldown = 15 * time.Minute
	planNotFoundMaxCooldown  = 60 * time.Minute

	catalogFailBaseCooldown = 10 * time.Minute
	catalogFailMaxCooldown  = 30 * time.Minute

	upstream504Threshold    = 2
	upstream504BaseCooldown = 10 * time.Minute
	upstream504MaxCooldown  = 30 * time.Minute

	catalog429BaseCooldown = 3 * time.Minute
	catalog429MaxCooldown  = 30 * time.Minute

	planCooldownCleanupGrace = 30 * time.Minute
)

type planCooldownEntry struct {
	accountID             string
	subsidiary            string
	planCode              string
	reason                planCooldownReason
	until                 time.Time
	failCount             int
	consecutive504        int
	wasMissingFromCatalog bool
	lastError             string
	skipLoggedUntil       time.Time
	updatedAt             time.Time
	prewarmSkippedCatalog bool
}

func (r planCooldownReason) label() string {
	switch r {
	case cooldownPlanNotFound:
		return "400 Plan code not found / 跨区目录不匹配"
	case cooldownCatalogFail:
		return "销售目录未收录/预售暂不可加车"
	case cooldownUpstream504:
		return "连续 504 上游超时"
	case cooldownCatalog429:
		return "未收录机型触发 429 限流"
	default:
		return string(r)
	}
}

func (m *Monitor) resolvePlanCooldownKey(accountID, subsidiary, planCode string) (key, canonAccountID, canonSubsidiary, displayPlan string) {
	canonAccountID = strings.TrimSpace(accountID)
	canonSubsidiary = strings.ToUpper(strings.TrimSpace(subsidiary))
	displayPlan = strings.TrimSpace(planCode)

	if m.state != nil {
		if acc, ok := m.state.FindAccount(canonAccountID); ok {
			canonAccountID = acc.ID
			if canonSubsidiary == "" {
				canonSubsidiary = catalog.SubsidiaryOfAccount(acc)
			}
		}
	}
	canonSubsidiary = strings.ToUpper(strings.TrimSpace(canonSubsidiary))
	canonPlan := strings.ToLower(displayPlan)
	key = canonAccountID + "|" + canonSubsidiary + "|" + canonPlan
	return key, canonAccountID, canonSubsidiary, displayPlan
}

func (m *Monitor) getOrCreateCooldownEntryLocked(key, accountID, subsidiary, planCode string, now time.Time) *planCooldownEntry {
	if m.planCooldowns == nil {
		m.planCooldowns = make(map[string]*planCooldownEntry)
	}
	entry, ok := m.planCooldowns[key]
	if !ok {
		entry = &planCooldownEntry{
			accountID:  accountID,
			subsidiary: subsidiary,
			planCode:   planCode,
			updatedAt:  now,
		}
		m.planCooldowns[key] = entry
	}
	return entry
}

func (m *Monitor) logCooldownInfo(msg string) {
	if m.state != nil && m.state.Logger != nil {
		m.state.Logger.Info(msg, "monitor")
	}
}

func (m *Monitor) logCooldownWarn(msg string) {
	if m.state != nil && m.state.Logger != nil {
		m.state.Logger.Warn(msg, "monitor")
	}
}

func isPlanNotFoundError(errStr string) bool {
	if errStr == "" {
		return false
	}
	lower := strings.ToLower(errStr)
	if strings.Contains(lower, "plan code not found") ||
		strings.Contains(errStr, "不在子公司") ||
		strings.Contains(errStr, "不在该账户所属子公司") {
		return true
	}
	kind, _ := ClassifyPriceCheckError(errStr)
	return kind == ErrorKindRegionBlocked
}

func isUpstream504Error(errStr string) bool {
	if errStr == "" {
		return false
	}
	lower := strings.ToLower(errStr)
	return strings.Contains(lower, "(ovh 504)") ||
		strings.Contains(lower, "status code 504") ||
		strings.Contains(lower, "504 gateway") ||
		strings.Contains(lower, "gateway time-out") ||
		strings.Contains(lower, "gateway timeout") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "deadline exceeded")
}

func isRateLimit429Error(errStr string) bool {
	if errStr == "" {
		return false
	}
	lower := strings.ToLower(errStr)
	return strings.Contains(lower, "(ovh 429)") ||
		strings.Contains(lower, "status code 429") ||
		strings.Contains(lower, "too many requests") ||
		strings.Contains(lower, "被 ovh 限流了")
}

func isPreSaleOrNotAvailableError(errStr string) bool {
	if errStr == "" {
		return false
	}
	lower := strings.ToLower(errStr)
	// "is not available in <dc>" 属于单机房无库存拒收，不应触发整机预售冷却；
	// 只识别明确未开售的产品级报错（如 "Product 27skle01-v1 is not available"）。
	if strings.Contains(lower, "product ") &&
		strings.Contains(lower, "is not available") &&
		!strings.Contains(lower, "is not available in") {
		return true
	}
	kind, _ := ClassifyPriceCheckError(errStr)
	return kind == ErrorKindPreSale
}

func computeEscalatedCooldown(failCount int, base, maxDur time.Duration) time.Duration {
	if failCount <= 1 {
		return base
	}
	d := base
	for i := 1; i < failCount; i++ {
		if d >= maxDur {
			return maxDur
		}
		d *= 2
	}
	if d > maxDur {
		return maxDur
	}
	return d
}

// isPlanInCooldown reports whether (accountID, subsidiary, planCode) is currently cooled down.
func (m *Monitor) isPlanInCooldown(accountID, subsidiary, planCode string) bool {
	active, _ := m.isPlanInCooldownDetail(accountID, subsidiary, planCode)
	return active
}

func (m *Monitor) isPlanInCooldownDetail(accountID, subsidiary, planCode string) (bool, *planCooldownEntry) {
	if m == nil {
		return false, nil
	}
	key, _, _, _ := m.resolvePlanCooldownKey(accountID, subsidiary, planCode)
	now := time.Now()

	m.planCooldownMu.Lock()
	defer m.planCooldownMu.Unlock()

	if m.planCooldowns == nil {
		return false, nil
	}
	entry, ok := m.planCooldowns[key]
	if !ok || entry.until.IsZero() {
		return false, nil
	}
	if !now.Before(entry.until) {
		return false, nil
	}
	cp := *entry
	return true, &cp
}

// logPlanCooldownSkip logs at most once per active cooldown window per (account, subsidiary, plan).
func (m *Monitor) logPlanCooldownSkip(accountID, subsidiary, planCode, source string) {
	if m == nil {
		return
	}
	key, canonAcc, canonSub, displayPlan := m.resolvePlanCooldownKey(accountID, subsidiary, planCode)
	now := time.Now()

	var logMsg string
	m.planCooldownMu.Lock()
	if m.planCooldowns != nil {
		if entry, ok := m.planCooldowns[key]; ok && now.Before(entry.until) && !entry.skipLoggedUntil.Equal(entry.until) {
			entry.skipLoggedUntil = entry.until
			rem := entry.until.Sub(now).Round(time.Second)
			logMsg = fmt.Sprintf("[monitor/cooldown] %s 跳过 (%s, %s, %s)：原因=%s，剩余冷却 %s",
				source, displayPlan, canonAcc, canonSub, entry.reason.label(), rem)
		}
	}
	m.planCooldownMu.Unlock()

	if logMsg != "" {
		m.logCooldownInfo(logMsg)
	}
}

// observePlanCatalogStatus auto-heals missing-catalog / not-found cooldowns when a plan newly appears in catalog.
func (m *Monitor) observePlanCatalogStatus(accountID, subsidiary, planCode string, inCatalog, anyCatalogMissing bool) {
	if m == nil {
		return
	}
	key, canonAcc, canonSub, displayPlan := m.resolvePlanCooldownKey(accountID, subsidiary, planCode)
	now := time.Now()

	var healMsg string
	m.planCooldownMu.Lock()
	if m.planCooldowns != nil {
		if entry, ok := m.planCooldowns[key]; ok {
			if anyCatalogMissing {
				entry.wasMissingFromCatalog = true
				entry.updatedAt = now
			} else if inCatalog {
				entry.prewarmSkippedCatalog = false
				if entry.reason == cooldownCatalogFail ||
					entry.reason == cooldownCatalog429 ||
					(entry.reason == cooldownPlanNotFound && entry.wasMissingFromCatalog) {
					prevReason := entry.reason.label()
					delete(m.planCooldowns, key)
					healMsg = fmt.Sprintf("[monitor/cooldown] 机型 %s 已纳入账户 %s (%s) 销售目录，解除「%s」冷却并恢复正常验价",
						displayPlan, canonAcc, canonSub, prevReason)
				} else {
					entry.wasMissingFromCatalog = false
					entry.updatedAt = now
				}
			}
		}
	}
	m.planCooldownMu.Unlock()

	if healMsg != "" {
		m.logCooldownInfo(healMsg)
	}
}

// recordPlanVerifySuccess resets cooldown and consecutive failure counters after a successful verification or prewarm.
func (m *Monitor) recordPlanVerifySuccess(accountID, subsidiary, planCode string) {
	if m == nil {
		return
	}
	key, canonAcc, canonSub, displayPlan := m.resolvePlanCooldownKey(accountID, subsidiary, planCode)

	var clearMsg string
	m.planCooldownMu.Lock()
	if m.planCooldowns != nil {
		if entry, ok := m.planCooldowns[key]; ok {
			if !entry.until.IsZero() && entry.reason != cooldownNone {
				clearMsg = fmt.Sprintf("[monitor/cooldown] %s (%s, %s) 验价/预热恢复成功，清除「%s」冷却状态",
					displayPlan, canonAcc, canonSub, entry.reason.label())
			}
			delete(m.planCooldowns, key)
		}
	}
	m.planCooldownMu.Unlock()

	if clearMsg != "" {
		m.logCooldownInfo(clearMsg)
	}
}

// recordPlanVerifyFailure classifies a verification or prewarm failure and activates/escalates cooldown if appropriate.
func (m *Monitor) recordPlanVerifyFailure(accountID, subsidiary, planCode string, catalogMissing bool, errStr string) {
	if m == nil {
		return
	}
	key, canonAcc, canonSub, displayPlan := m.resolvePlanCooldownKey(accountID, subsidiary, planCode)
	now := time.Now()

	var enterMsg string
	var warn bool

	m.planCooldownMu.Lock()
	entry := m.getOrCreateCooldownEntryLocked(key, canonAcc, canonSub, displayPlan, now)
	if catalogMissing {
		entry.wasMissingFromCatalog = true
	}
	entry.lastError = errStr
	entry.updatedAt = now

	var dur time.Duration
	var reason planCooldownReason

	switch {
	case isPlanNotFoundError(errStr):
		entry.consecutive504 = 0
		if entry.reason != cooldownPlanNotFound {
			entry.failCount = 0
		}
		entry.failCount++
		reason = cooldownPlanNotFound
		dur = computeEscalatedCooldown(entry.failCount, planNotFoundBaseCooldown, planNotFoundMaxCooldown)
		warn = true

	case isUpstream504Error(errStr):
		entry.consecutive504++
		if entry.consecutive504 >= upstream504Threshold {
			if entry.reason != cooldownUpstream504 {
				entry.failCount = 0
			}
			entry.failCount++
			reason = cooldownUpstream504
			dur = computeEscalatedCooldown(entry.failCount, upstream504BaseCooldown, upstream504MaxCooldown)
			warn = true
		}

	case catalogMissing && isRateLimit429Error(errStr):
		entry.consecutive504 = 0
		if entry.reason != cooldownCatalog429 {
			entry.failCount = 0
		}
		entry.failCount++
		reason = cooldownCatalog429
		dur = computeEscalatedCooldown(entry.failCount, catalog429BaseCooldown, catalog429MaxCooldown)
		warn = false

	case catalogMissing:
		entry.consecutive504 = 0
		if entry.reason != cooldownCatalogFail {
			entry.failCount = 0
		}
		entry.failCount++
		reason = cooldownCatalogFail
		dur = computeEscalatedCooldown(entry.failCount, catalogFailBaseCooldown, catalogFailMaxCooldown)
		warn = false

	case isPreSaleOrNotAvailableError(errStr):
		entry.consecutive504 = 0
		if entry.reason != cooldownCatalogFail {
			entry.failCount = 0
		}
		entry.failCount++
		reason = cooldownCatalogFail
		dur = computeEscalatedCooldown(entry.failCount, catalogFailBaseCooldown, catalogFailMaxCooldown)
		warn = false

	default:
		entry.consecutive504 = 0
		if entry.until.IsZero() && entry.failCount == 0 {
			delete(m.planCooldowns, key)
		}
	}

	if reason != cooldownNone && dur > 0 {
		entry.reason = reason
		entry.until = now.Add(dur)
		entry.skipLoggedUntil = time.Time{}
		enterMsg = fmt.Sprintf("[monitor/cooldown] %s (%s, %s) 进入长冷却 %s（第 %d 次，原因: %s；详情: %s）",
			displayPlan, canonAcc, canonSub, dur, entry.failCount, reason.label(), errStr)
	}
	m.planCooldownMu.Unlock()

	if enterMsg != "" {
		if warn {
			m.logCooldownWarn(enterMsg)
		} else {
			m.logCooldownInfo(enterMsg)
		}
	}
}

// shouldSkipPrewarmForCatalog handles deduped logging when an Eco plan is confirmed absent from the account's catalog.
func (m *Monitor) markPrewarmCatalogMissing(accountID, subsidiary, planCode string) {
	if m == nil {
		return
	}
	key, canonAcc, canonSub, displayPlan := m.resolvePlanCooldownKey(accountID, subsidiary, planCode)
	now := time.Now()

	var logMsg string
	m.planCooldownMu.Lock()
	entry := m.getOrCreateCooldownEntryLocked(key, canonAcc, canonSub, displayPlan, now)
	entry.wasMissingFromCatalog = true
	entry.updatedAt = now
	if !entry.prewarmSkippedCatalog {
		entry.prewarmSkippedCatalog = true
		logMsg = fmt.Sprintf("[monitor/prewarm] 跳过 %s（账户 %s / %s）：销售目录尚未收录该 Eco 机型，等待收录后再预热",
			displayPlan, canonAcc, canonSub)
	}
	m.planCooldownMu.Unlock()

	if logMsg != "" {
		m.logCooldownInfo(logMsg)
	}
}

func (m *Monitor) cleanupExpiredPlanCooldowns() {
	if m == nil {
		return
	}
	now := time.Now()

	m.planCooldownMu.Lock()
	defer m.planCooldownMu.Unlock()

	for key, entry := range m.planCooldowns {
		if entry == nil {
			delete(m.planCooldowns, key)
			continue
		}
		if entry.prewarmSkippedCatalog && now.Sub(entry.updatedAt) < 2*time.Hour {
			continue
		}
		if !entry.until.IsZero() && now.Before(entry.until.Add(planCooldownCleanupGrace)) {
			continue
		}
		if entry.until.IsZero() && entry.consecutive504 > 0 && now.Sub(entry.updatedAt) < planCooldownCleanupGrace {
			continue
		}
		delete(m.planCooldowns, key)
	}
}

