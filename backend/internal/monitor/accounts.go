package monitor

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/ovh-webui/server/internal/app"
)

// NormalizeOrderAccounts preserves legacy single-account subscriptions. A nil
// list falls back to the legacy ID; an explicitly empty list clears all targets.
// An empty legacy ID never means all accounts.
func NormalizeOrderAccounts(legacy string, ids []string) []string {
	if ids == nil {
		ids = []string{legacy}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	return out
}

func cloneAccountIDs(ids []string) []string {
	if ids == nil {
		return nil
	}
	return append([]string{}, ids...)
}

func optionalOrderAccounts(legacy string, ids [][]string) []string {
	if len(ids) > 0 && ids[0] != nil {
		return NormalizeOrderAccounts(legacy, ids[0])
	}
	// Keep nil for old call sites so persisted inventory keys retain their meaning.
	return nil
}

func firstOrderAccount(ids []string) string {
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

func accountStatusPrefix(id string) string {
	encoded, _ := json.Marshal(id)
	return "account:" + string(encoded) + "|"
}

// setOrderAccountsLocked migrates a legacy inventory baseline before adding
// account targets. Existing accounts must not receive a duplicate initial order.
func (s *Subscription) setOrderAccountsLocked(ids []string, legacy ...string) {
	oldID := s.AutoOrderAccountID
	if ids == nil && s.AutoOrderAccountIDs != nil {
		// Older clients can still select one account. Keep scoped state rather
		// than forgetting its baseline and treating sustained stock as new.
		target := ""
		if len(legacy) > 0 {
			target = legacy[0]
		}
		ids = NormalizeOrderAccounts(target, nil)
	}
	if ids != nil && s.AutoOrderAccountIDs == nil && oldID != "" {
		migrated := make(map[string]string, len(s.LastStatus))
		for key, value := range s.LastStatus {
			if strings.HasPrefix(key, "account:") {
				migrated[key] = value
			} else {
				migrated[accountStatusPrefix(oldID)+key] = value
			}
		}
		s.LastStatus = migrated
	}
	s.AutoOrderAccountIDs = cloneAccountIDs(ids)
	s.AutoOrderAccountID = firstOrderAccount(ids)
	if ids == nil && len(legacy) > 0 {
		s.AutoOrderAccountID = strings.TrimSpace(legacy[0])
	}
}

// guardCheckEnqueue makes the last revision check atomic with queue insertion.
// Lock order matches subscription edits/pause: subsMu -> sub.mu -> QueueMu.
func (m *Monitor) guardCheckEnqueue(sub *Subscription, cfg subCheckConfig, enqueue func() error) error {
	if sub.checkParent != nil {
		sub = sub.checkParent
	}
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	found := false
	for _, current := range m.subscriptions {
		if current == sub {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("监控订阅已删除,取消本轮自动下单")
	}
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.Paused || sub.revision != cfg.Revision {
		return fmt.Errorf("监控配置已更新或暂停,取消旧配置自动下单")
	}
	return enqueue()
}

func monitorQueueScopes(planCode string, cfg subCheckConfig) []app.MonitorQueueScope {
	ids := NormalizeOrderAccounts(cfg.AutoOrderAccountID, cfg.AutoOrderAccountIDs)
	if len(ids) == 0 {
		// Preserve the legacy pause behavior for notify-only subscriptions.
		ids = []string{""}
	}
	scopes := make([]app.MonitorQueueScope, 0, len(ids))
	for _, id := range ids {
		scopes = append(scopes, app.MonitorQueueScope{PlanCode: planCode, AccountID: id, Datacenters: cfg.Datacenters, Options: cfg.Options})
	}
	return scopes
}

// CheckAvailabilityChange runs the existing stock/price/order state machine for
// each explicitly selected account. Isolated baselines prevent one account's
// availability or price failure from suppressing another account's orders.
func (m *Monitor) CheckAvailabilityChange(sub *Subscription, traceID string) {
	cfg := sub.checkConfig()
	if cfg.Paused || !m.checkCurrent(sub, cfg) {
		return
	}
	ids := NormalizeOrderAccounts(cfg.AutoOrderAccountID, cfg.AutoOrderAccountIDs)
	if cfg.AutoOrderAccountIDs == nil || len(ids) == 0 {
		m.checkAccountAvailabilityChange(sub, traceID)
		return
	}
	checks := make([]*Subscription, len(ids))
	var wg sync.WaitGroup
	for index, id := range ids {
		check := sub.snapshot()
		// The account list belongs to cfg's revision, even if an edit happened
		// between taking cfg and creating this isolated working snapshot.
		check.revision = cfg.Revision
		check.checkParent = sub
		check.AutoOrderAccountID = id
		check.AutoOrderAccountIDs = nil
		check.LastStatus = map[string]string{}
		prefix := accountStatusPrefix(id)
		for key, value := range sub.statusSnapshot() {
			if strings.HasPrefix(key, prefix) {
				check.LastStatus[strings.TrimPrefix(key, prefix)] = value
			}
		}
		checks[index] = check
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					check.setCheckError(fmt.Sprintf("账户 %s 检查失败: %v", id, recovered))
				}
			}()
			m.checkAccountAvailabilityChange(check, traceID)
		}()
	}
	wg.Wait()
	if !m.checkCurrent(sub, cfg) {
		return
	}
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.revision != cfg.Revision || sub.Paused {
		return
	}
	if sub.LastStatus == nil {
		sub.LastStatus = map[string]string{}
	}
	errors := []string{}
	for index, check := range checks {
		prefix := accountStatusPrefix(ids[index])
		for key, value := range check.LastStatus {
			sub.LastStatus[prefix+key] = value
		}
		if check.LastCheckError != "" {
			errors = append(errors, ids[index]+": "+check.LastCheckError)
		}
		if index == 0 {
			sub.LastCheckAt = check.LastCheckAt
			sub.LastCheckAccountID = check.LastCheckAccountID
			sub.LastCheckRegion = check.LastCheckRegion
			sub.LastCheckSubsidiary = check.LastCheckSubsidiary
		}
	}
	sub.LastCheckError = strings.Join(errors, "; ")
}
