package monitor

import (
	"fmt"
	"time"
)

// AddSubscription 对应 Python: add_subscription
// autoOrderAccountID:auto_order 触发时用哪个账户下单;空 = 只通知不下单
func (m *Monitor) AddSubscription(planCode string, datacenters []string, notifyAvailable, notifyUnavailable bool,
	serverName string, lastStatus map[string]string, history []HistoryEntry, autoOrder bool, quantity int,
	autoOrderAccountID string) {

	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	for _, s := range m.subscriptions {
		if s.PlanCode == planCode {
			m.state.Logger.Warn(fmt.Sprintf("订阅已存在: %s，将更新配置（不会重置状态，避免重复通知）", planCode), "monitor")
			if datacenters == nil {
				datacenters = []string{}
			}
			s.Datacenters = datacenters
			s.NotifyAvailable = notifyAvailable
			s.NotifyUnavailable = notifyUnavailable
			s.AutoOrder = autoOrder
			if autoOrder {
				if quantity < 1 {
					quantity = 1
				}
				s.Quantity = quantity
			} else {
				s.Quantity = 0
			}
			s.ServerName = serverName
			s.AutoOrderAccountID = autoOrderAccountID
			if s.History == nil {
				s.History = []HistoryEntry{}
			}
			return
		}
	}

	if datacenters == nil {
		datacenters = []string{}
	}
	if lastStatus == nil {
		lastStatus = map[string]string{}
	}
	if history == nil {
		history = []HistoryEntry{}
	}
	sub := &Subscription{
		PlanCode:           planCode,
		Datacenters:        datacenters,
		NotifyAvailable:    notifyAvailable,
		NotifyUnavailable:  notifyUnavailable,
		LastStatus:         lastStatus,
		CreatedAt:          time.Now().Format(time.RFC3339Nano),
		History:            history,
		AutoOrderAccountID: autoOrderAccountID,
	}
	if autoOrder {
		if quantity < 1 {
			quantity = 1
		}
		sub.AutoOrder = true
		sub.Quantity = quantity
	}
	if serverName != "" {
		sub.ServerName = serverName
	}
	m.subscriptions = append(m.subscriptions, sub)
	displayName := planCode
	if serverName != "" {
		displayName = planCode + " (" + serverName + ")"
	}
	dcsStr := "全部"
	if len(datacenters) > 0 {
		dcsStr = fmt.Sprintf("%v", datacenters)
	}
	m.state.Logger.Info(fmt.Sprintf("添加订阅: %s, 数据中心: %s", displayName, dcsStr), "monitor")
}

// RemoveSubscription 对应 Python: remove_subscription
func (m *Monitor) RemoveSubscription(planCode string) bool {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	original := len(m.subscriptions)
	kept := make([]*Subscription, 0, len(m.subscriptions))
	for _, s := range m.subscriptions {
		if s.PlanCode != planCode {
			kept = append(kept, s)
		}
	}
	m.subscriptions = kept
	if len(m.subscriptions) < original {
		m.state.Logger.Info("删除订阅: "+planCode, "monitor")
		return true
	}
	return false
}

// ClearSubscriptions 对应 Python: clear_subscriptions
func (m *Monitor) ClearSubscriptions() int {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	count := len(m.subscriptions)
	m.subscriptions = []*Subscription{}
	m.state.Logger.Info(fmt.Sprintf("清空所有订阅 (%d 项)", count), "monitor")
	return count
}

// FindSubscription 按 planCode 查找
func (m *Monitor) FindSubscription(planCode string) *Subscription {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	for _, s := range m.subscriptions {
		if s.PlanCode == planCode {
			return s
		}
	}
	return nil
}

// SetKnownServers 用于从持久化恢复
func (m *Monitor) SetKnownServers(set map[string]struct{}) {
	m.subsMu.Lock()
	m.knownServers = set
	m.subsMu.Unlock()
}

// KnownServers 返回当前已知服务器集合（用于持久化）
func (m *Monitor) KnownServers() []string {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	out := make([]string, 0, len(m.knownServers))
	for k := range m.knownServers {
		out = append(out, k)
	}
	return out
}

// MessageUUIDCacheLookup 用于 webhook 回调时取回完整配置
func (m *Monitor) MessageUUIDCacheLookup(id string) *CachedMessage {
	m.cacheLock.Lock()
	defer m.cacheLock.Unlock()
	if cm, ok := m.messageUUIDCache[id]; ok {
		if time.Now().Unix()-int64(cm.Timestamp) < int64(m.messageUUIDCacheTTL.Seconds()) {
			return cm
		}
		delete(m.messageUUIDCache, id)
		m.state.Logger.Warn("UUID缓存已过期: "+id, "telegram")
	}
	return nil
}

// OptionsCacheLookup 兼容旧机制
func (m *Monitor) OptionsCacheLookup(key string) []string {
	m.cacheLock.Lock()
	defer m.cacheLock.Unlock()
	if c, ok := m.optionsCache[key]; ok {
		if time.Now().Unix()-int64(c.Timestamp) < int64(m.optionsCacheTTL.Seconds()) {
			return c.Options
		}
		delete(m.optionsCache, key)
		m.state.Logger.Warn("options缓存已过期: "+key, "telegram")
	}
	return nil
}

// cleanupExpiredCaches 对应 Python: _cleanup_expired_caches
func (m *Monitor) cleanupExpiredCaches() {
	now := time.Now().Unix()
	m.cacheLock.Lock()
	defer m.cacheLock.Unlock()
	expUUIDs := []string{}
	for k, v := range m.messageUUIDCache {
		if now-int64(v.Timestamp) >= int64(m.messageUUIDCacheTTL.Seconds()) {
			expUUIDs = append(expUUIDs, k)
		}
	}
	for _, k := range expUUIDs {
		delete(m.messageUUIDCache, k)
	}
	expOpts := []string{}
	for k, v := range m.optionsCache {
		if now-int64(v.Timestamp) >= int64(m.optionsCacheTTL.Seconds()) {
			expOpts = append(expOpts, k)
		}
	}
	for _, k := range expOpts {
		delete(m.optionsCache, k)
	}
	if len(expUUIDs) > 0 || len(expOpts) > 0 {
		m.state.Logger.Debug(fmt.Sprintf("清理过期缓存: UUID=%d个, Options=%d个", len(expUUIDs), len(expOpts)), "monitor")
	}
}

// AddMessageUUID 缓存按钮对应的配置
func (m *Monitor) AddMessageUUID(id, planCode, datacenter string, options []string, configInfo map[string]interface{}) {
	m.cacheLock.Lock()
	defer m.cacheLock.Unlock()
	m.messageUUIDCache[id] = &CachedMessage{
		PlanCode:   planCode,
		Datacenter: datacenter,
		Options:    options,
		ConfigInfo: configInfo,
		Timestamp:  float64(time.Now().Unix()),
	}
}
