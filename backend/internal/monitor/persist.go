package monitor

import (
	"encoding/json"
	"fmt"

	"github.com/ovh-webui/server/internal/types"
)

// monitor 包内部用 Subscription / HistoryEntry，
// 而 SQLite 层用 types.Subscription / types.SubscriptionHistoryEntry。
// 字段一一对应，下面提供双向转换。

func toDBSub(s *Subscription) types.Subscription {
	if s == nil {
		return types.Subscription{}
	}
	hist := make([]types.SubscriptionHistoryEntry, 0, len(s.History))
	for _, h := range s.History {
		hist = append(hist, types.SubscriptionHistoryEntry{
			Timestamp:  h.Timestamp,
			Datacenter: h.Datacenter,
			Status:     h.Status,
			ChangeType: h.ChangeType,
			OldStatus:  h.OldStatus,
			Config:     h.Config,
		})
	}
	dcs := s.Datacenters
	if dcs == nil {
		dcs = []string{}
	}
	last := s.LastStatus
	if last == nil {
		last = map[string]string{}
	}
	return types.Subscription{
		PlanCode:           s.PlanCode,
		Datacenters:        dcs,
		NotifyAvailable:    s.NotifyAvailable,
		NotifyUnavailable:  s.NotifyUnavailable,
		LastStatus:         last,
		CreatedAt:          s.CreatedAt,
		History:            hist,
		ServerName:         s.ServerName,
		AutoOrder:          s.AutoOrder,
		Quantity:           s.Quantity,
		AutoOrderAccountID: s.AutoOrderAccountID,
	}
}

func fromDBSub(s types.Subscription) *Subscription {
	hist := make([]HistoryEntry, 0, len(s.History))
	for _, h := range s.History {
		hist = append(hist, HistoryEntry{
			Timestamp:  h.Timestamp,
			Datacenter: h.Datacenter,
			Status:     h.Status,
			ChangeType: h.ChangeType,
			OldStatus:  h.OldStatus,
			Config:     h.Config,
		})
	}
	dcs := s.Datacenters
	if dcs == nil {
		dcs = []string{}
	}
	last := s.LastStatus
	if last == nil {
		last = map[string]string{}
	}
	return &Subscription{
		PlanCode:           s.PlanCode,
		Datacenters:        dcs,
		NotifyAvailable:    s.NotifyAvailable,
		NotifyUnavailable:  s.NotifyUnavailable,
		LastStatus:         last,
		CreatedAt:          s.CreatedAt,
		History:            hist,
		ServerName:         s.ServerName,
		AutoOrder:          s.AutoOrder,
		Quantity:           s.Quantity,
		AutoOrderAccountID: s.AutoOrderAccountID,
	}
}

// LoadFromDB 启动时从 SQLite 加载订阅 + known_servers
func (m *Monitor) LoadFromDB() {
	subs, err := m.state.DB.ListMonitorSubscriptions()
	if err != nil {
		// 失败时保留空列表，但绝不能随后 SaveToDB 全表覆盖（会抹掉库里的真实订阅）
		m.state.Logger.Error("加载监控订阅失败（不会写回空列表）: "+err.Error(), "monitor")
		subs = nil
	}
	known := []string{}
	if _, err := m.state.DB.GetKV("monitor_known_servers", &known); err != nil {
		m.state.Logger.Warn("加载已知服务器失败: "+err.Error(), "monitor")
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	m.subscriptions = make([]*Subscription, 0, len(subs))
	for _, s := range subs {
		m.subscriptions = append(m.subscriptions, fromDBSub(s))
	}
	knownSet := map[string]struct{}{}
	for _, k := range known {
		knownSet[k] = struct{}{}
	}
	m.knownServers = knownSet
	// 全局强制 5 秒
	m.checkInterval = 5
	m.state.Logger.Info("检查间隔已强制设置为: 5秒（全局固定值）", "monitor")
	m.state.Logger.Info(fmt.Sprintf("已加载订阅: %d 条", len(m.subscriptions)), "monitor")
	// TG 一键下单 UUID 在 LoadFromDB 返回后由调用方 LoadMessageUUIDCacheFromDB()
}

// SaveToDB 把订阅 + known_servers 写回 SQLite
func (m *Monitor) SaveToDB() {
	m.subsMu.Lock()
	subs := make([]types.Subscription, 0, len(m.subscriptions))
	for _, s := range m.subscriptions {
		subs = append(subs, toDBSub(s))
	}
	known := make([]string, 0, len(m.knownServers))
	for k := range m.knownServers {
		known = append(known, k)
	}
	m.checkInterval = 5
	n := len(subs)
	m.subsMu.Unlock()

	// Replace 会先清空表再写入；允许空列表（用户主动 clear），但打醒目日志便于排查
	if n == 0 {
		m.state.Logger.Warn("保存监控订阅: 当前内存列表为空，将清空 SQLite 订阅表", "monitor")
	}
	if err := m.state.DB.ReplaceMonitorSubscriptions(subs); err != nil {
		m.state.Logger.Error("保存监控订阅失败: "+err.Error(), "monitor")
		return
	}
	if err := m.state.DB.SetKV("monitor_known_servers", known); err != nil {
		m.state.Logger.Error("保存已知服务器失败: "+err.Error(), "monitor")
		return
	}
	m.state.Logger.Info(fmt.Sprintf("订阅数据已保存: %d 条（检查间隔固定为5秒）", n), "monitor")
}

// SubscriptionAsJSON 帮助 handler 返回订阅
func (m *Monitor) SubscriptionAsJSON(planCode string) ([]byte, bool) {
	sub := m.FindSubscription(planCode)
	if sub == nil {
		return nil, false
	}
	b, _ := json.Marshal(sub)
	return b, true
}
