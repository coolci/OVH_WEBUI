package monitor

import (
	"fmt"
	"time"

	"github.com/ovh-webui/server/internal/db"
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

// MessageUUIDCacheLookup 用于 webhook 回调时取回完整配置。
// 先查内存，再查 SQLite（进程重启后按钮仍可用）。
func (m *Monitor) MessageUUIDCacheLookup(id string) *CachedMessage {
	ttlSec := int64(m.messageUUIDCacheTTL.Seconds())
	now := time.Now().Unix()

	// 注意：lookup 允许读取未消费配置；是否已 used 由 webhook 层单独判断。
	// 不要在这里因 used_at 返回 nil，否则「先 consume 再 lookup」或并发路径会丢配置。

	m.cacheLock.Lock()
	if cm, ok := m.messageUUIDCache[id]; ok {
		if now-int64(cm.Timestamp) < ttlSec {
			m.cacheLock.Unlock()
			return cm
		}
		delete(m.messageUUIDCache, id)
		m.cacheLock.Unlock()
		m.state.Logger.Warn("UUID缓存已过期: "+id, "telegram")
		if m.state.DB != nil {
			_ = m.state.DB.DeleteTelegramButton(id)
		}
		return nil
	}
	m.cacheLock.Unlock()

	// 内存未命中 → SQLite（部署/重启后恢复）
	if m.state.DB == nil {
		return nil
	}
	row, ok, err := m.state.DB.GetTelegramButton(id)
	if err != nil {
		m.state.Logger.Warn("读取 UUID 持久化缓存失败: "+err.Error(), "telegram")
		return nil
	}
	if !ok {
		return nil
	}
	if now-int64(row.CreatedAt) >= ttlSec {
		m.state.Logger.Warn("UUID持久化缓存已过期: "+id, "telegram")
		_ = m.state.DB.DeleteTelegramButton(id)
		return nil
	}
	cm := &CachedMessage{
		PlanCode:   row.PlanCode,
		Datacenter: row.Datacenter,
		Options:    db.ParseTelegramButtonOptions(row.Options),
		ConfigInfo: db.ParseTelegramButtonConfigInfo(row.ConfigInfo),
		Timestamp:  row.CreatedAt,
	}
	// 回灌内存，避免每次点按钮都查库
	m.cacheLock.Lock()
	m.messageUUIDCache[id] = cm
	m.cacheLock.Unlock()
	m.state.Logger.Info("✅ 从 SQLite 恢复 UUID 按钮配置: "+id+" → "+cm.PlanCode+"@"+cm.Datacenter, "telegram")
	return cm
}

// InvalidateMessageUUID 入队成功后从内存删除按钮（DB used_at 由 TryConsume 负责）
func (m *Monitor) InvalidateMessageUUID(id string) {
	if id == "" {
		return
	}
	m.cacheLock.Lock()
	delete(m.messageUUIDCache, id)
	m.cacheLock.Unlock()
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
	ttlUUID := int64(m.messageUUIDCacheTTL.Seconds())
	ttlOpts := int64(m.optionsCacheTTL.Seconds())
	m.cacheLock.Lock()
	expUUIDs := []string{}
	for k, v := range m.messageUUIDCache {
		if now-int64(v.Timestamp) >= ttlUUID {
			expUUIDs = append(expUUIDs, k)
		}
	}
	for _, k := range expUUIDs {
		delete(m.messageUUIDCache, k)
	}
	expOpts := []string{}
	for k, v := range m.optionsCache {
		if now-int64(v.Timestamp) >= ttlOpts {
			expOpts = append(expOpts, k)
		}
	}
	for _, k := range expOpts {
		delete(m.optionsCache, k)
	}
	m.cacheLock.Unlock()

	// 同步清理 SQLite 过期按钮
	if m.state.DB != nil {
		if n, err := m.state.DB.DeleteExpiredTelegramButtons(float64(now - ttlUUID)); err != nil {
			m.state.Logger.Warn("清理过期 TG 按钮失败: "+err.Error(), "monitor")
		} else if n > 0 {
			m.state.Logger.Debug(fmt.Sprintf("清理过期 TG 按钮: %d 条", n), "monitor")
		}
	}
	if len(expUUIDs) > 0 || len(expOpts) > 0 {
		m.state.Logger.Debug(fmt.Sprintf("清理过期缓存: UUID=%d个, Options=%d个", len(expUUIDs), len(expOpts)), "monitor")
	}
}

// AddMessageUUID 缓存按钮对应的配置（内存 + SQLite 双写）
func (m *Monitor) AddMessageUUID(id, planCode, datacenter string, options []string, configInfo map[string]interface{}) {
	ts := float64(time.Now().Unix())
	if options == nil {
		options = []string{}
	}
	m.cacheLock.Lock()
	m.messageUUIDCache[id] = &CachedMessage{
		PlanCode:   planCode,
		Datacenter: datacenter,
		Options:    append([]string{}, options...),
		ConfigInfo: configInfo,
		Timestamp:  ts,
	}
	m.cacheLock.Unlock()

	if m.state.DB != nil {
		if err := m.state.DB.UpsertTelegramButton(id, planCode, datacenter, options, configInfo, ts); err != nil {
			m.state.Logger.Warn("持久化 TG 一键下单按钮失败: "+err.Error(), "monitor")
		}
	}
}

// LoadMessageUUIDCacheFromDB 启动时回灌近 TTL 内的按钮配置
func (m *Monitor) LoadMessageUUIDCacheFromDB() {
	if m.state.DB == nil {
		return
	}
	since := float64(time.Now().Add(-m.messageUUIDCacheTTL).Unix())
	rows, err := m.state.DB.ListTelegramButtonsSince(since)
	if err != nil {
		m.state.Logger.Warn("加载 TG 一键下单按钮缓存失败: "+err.Error(), "monitor")
		return
	}
	m.cacheLock.Lock()
	for _, row := range rows {
		m.messageUUIDCache[row.ID] = &CachedMessage{
			PlanCode:   row.PlanCode,
			Datacenter: row.Datacenter,
			Options:    db.ParseTelegramButtonOptions(row.Options),
			ConfigInfo: db.ParseTelegramButtonConfigInfo(row.ConfigInfo),
			Timestamp:  row.CreatedAt,
		}
	}
	n := len(rows)
	m.cacheLock.Unlock()
	if n > 0 {
		m.state.Logger.Info(fmt.Sprintf("已从 SQLite 回灌 %d 个 TG 一键下单按钮", n), "monitor")
	}
}
