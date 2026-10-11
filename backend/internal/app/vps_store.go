package app

import (
	"strings"

	"github.com/ovh-webui/server/internal/types"
)

// VPS 订阅的"内存 + 落库"存储操作。
// 封装 VPSSubsMu 并联动 SQLite 落库，避免 handlers 直接操纵锁和切片。

// VPSSubscriptionSnapshot 返回 VPS 订阅的深拷贝切片(保证非 nil)。
func (s *State) VPSSubscriptionSnapshot() []types.VPSSubscription {
	s.VPSSubsMu.Lock()
	defer s.VPSSubsMu.Unlock()

	if s.VPSSubscriptions == nil {
		return []types.VPSSubscription{}
	}
	return cloneVPSSubs(s.VPSSubscriptions)
}

// FindVPSSubscription 查找单个 VPS 订阅(返回深拷贝，避免并发读写底层 map/slice)。
func (s *State) FindVPSSubscription(id string) (*types.VPSSubscription, bool) {
	s.VPSSubsMu.Lock()
	defer s.VPSSubsMu.Unlock()

	for _, it := range s.VPSSubscriptions {
		if it.ID == id {
			cp := cloneVPSSub(it)
			return &cp, true
		}
	}
	return nil, false
}

// HasVPSSubscription 检查是否存在相同 planCode 与 subsidiary 的订阅。
func (s *State) HasVPSSubscription(planCode, subsidiary string) bool {
	norm := strings.ToUpper(strings.TrimSpace(subsidiary))
	s.VPSSubsMu.Lock()
	defer s.VPSSubsMu.Unlock()

	for _, it := range s.VPSSubscriptions {
		if it.PlanCode == planCode && strings.ToUpper(strings.TrimSpace(it.OvhSubsidiary)) == norm {
			return true
		}
	}
	return false
}

// AddVPSSubscription 添加单个订阅并落库。
func (s *State) AddVPSSubscription(sub types.VPSSubscription) error {
	if err := s.SaveBlocked("vps_subscriptions"); err != nil {
		s.Logger.Error(err.Error(), "vps")
		return err
	}
	s.VPSSubsMu.Lock()
	s.VPSSubscriptions = append(s.VPSSubscriptions, sub)
	subs := cloneVPSSubs(s.VPSSubscriptions)
	s.VPSSubsMu.Unlock()

	return s.DB.ReplaceVPSSubscriptions(subs)
}

// RemoveVPSSubscription 按 id 删除订阅并落库。
// 返回: removed (是否存在并已删), empty (删除后是否空队列), err (落库错误)
func (s *State) RemoveVPSSubscription(id string) (bool, bool, error) {
	if err := s.SaveBlocked("vps_subscriptions"); err != nil {
		s.Logger.Error(err.Error(), "vps")
		return false, false, err
	}
	s.VPSSubsMu.Lock()
	original := len(s.VPSSubscriptions)
	kept := make([]types.VPSSubscription, 0, original)
	for _, it := range s.VPSSubscriptions {
		if it.ID != id {
			kept = append(kept, it)
		}
	}
	s.VPSSubscriptions = kept
	removed := len(s.VPSSubscriptions) < original
	empty := len(s.VPSSubscriptions) == 0
	subs := cloneVPSSubs(s.VPSSubscriptions)
	s.VPSSubsMu.Unlock()

	if !removed {
		return false, empty, nil
	}
	err := s.DB.ReplaceVPSSubscriptions(subs)
	return true, empty, err
}

// ClearVPSSubscriptions 清空全部 VPS 订阅并落库。
func (s *State) ClearVPSSubscriptions() (int, error) {
	if err := s.SaveBlocked("vps_subscriptions"); err != nil {
		s.Logger.Error(err.Error(), "vps")
		return 0, err
	}
	s.VPSSubsMu.Lock()
	count := len(s.VPSSubscriptions)
	s.VPSSubscriptions = []types.VPSSubscription{}
	s.VPSSubsMu.Unlock()

	_, err := s.DB.ClearVPSSubscriptions()
	return count, err
}

// MutateVPSSubscription 查找指定订阅，并在锁保护下通过 mutateFn 更新，最后持久化落库。
// mutateFn 返回 true 表示需要落库，返回 false 表示跳过落库。
func (s *State) MutateVPSSubscription(id string, mutateFn func(*types.VPSSubscription) bool) (*types.VPSSubscription, bool, error) {
	if err := s.SaveBlocked("vps_subscriptions"); err != nil {
		s.Logger.Error(err.Error(), "vps")
		return nil, false, err
	}
	s.VPSSubsMu.Lock()
	idx := -1
	for i, sub := range s.VPSSubscriptions {
		if sub.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.VPSSubsMu.Unlock()
		return nil, false, nil
	}
	if !mutateFn(&s.VPSSubscriptions[idx]) {
		cp := cloneVPSSub(s.VPSSubscriptions[idx])
		s.VPSSubsMu.Unlock()
		return &cp, true, nil
	}
	updated := cloneVPSSub(s.VPSSubscriptions[idx])
	subs := cloneVPSSubs(s.VPSSubscriptions)
	s.VPSSubsMu.Unlock()

	err := s.DB.ReplaceVPSSubscriptions(subs)
	return &updated, true, err
}

// ReloadVPSSubscriptionsFromDB 从数据库重新加载 VPS 订阅列表到内存。
func (s *State) ReloadVPSSubscriptionsFromDB() error {
	subs, err := s.DB.ListVPSSubscriptions()
	if err != nil {
		return err
	}
	s.VPSSubsMu.Lock()
	if subs == nil {
		s.VPSSubscriptions = []types.VPSSubscription{}
	} else {
		s.VPSSubscriptions = subs
	}
	s.VPSSubsMu.Unlock()
	return nil
}

// VPSMonitorStatus 获取当前 VPS 监控订阅数量与检查间隔。
func (s *State) VPSMonitorStatus() (count int, interval int) {
	s.VPSSubsMu.Lock()
	defer s.VPSSubsMu.Unlock()
	return len(s.VPSSubscriptions), s.VPSCheckInterval
}

// SetVPSCheckInterval 设置并持久化 VPS 检查间隔。
func (s *State) SetVPSCheckInterval(interval int) error {
	s.VPSSubsMu.Lock()
	s.VPSCheckInterval = interval
	s.VPSSubsMu.Unlock()

	return s.DB.SetKV("vps_check_interval", interval)
}

// BatchAddVPSSubscriptions 批量添加未存在的订阅并持久化。
func (s *State) BatchAddVPSSubscriptions(subs []types.VPSSubscription) (int, error) {
	if err := s.SaveBlocked("vps_subscriptions"); err != nil {
		s.Logger.Error(err.Error(), "vps")
		return 0, err
	}
	s.VPSSubsMu.Lock()
	existingMap := make(map[string]bool, len(s.VPSSubscriptions))
	for _, it := range s.VPSSubscriptions {
		existingMap[it.PlanCode+"@"+strings.ToUpper(strings.TrimSpace(it.OvhSubsidiary))] = true
	}
	added := 0
	for _, it := range subs {
		key := it.PlanCode + "@" + strings.ToUpper(strings.TrimSpace(it.OvhSubsidiary))
		if existingMap[key] {
			continue
		}
		s.VPSSubscriptions = append(s.VPSSubscriptions, it)
		existingMap[key] = true
		added++
	}
	allSubs := cloneVPSSubs(s.VPSSubscriptions)
	s.VPSSubsMu.Unlock()

	if err := s.DB.ReplaceVPSSubscriptions(allSubs); err != nil {
		return added, err
	}
	return added, nil
}

func cloneVPSSub(it types.VPSSubscription) types.VPSSubscription {
	cp := it
	if it.Datacenters != nil {
		cp.Datacenters = append([]string{}, it.Datacenters...)
	} else {
		cp.Datacenters = []string{}
	}
	if it.LastStatus != nil {
		cp.LastStatus = make(map[string]string, len(it.LastStatus))
		for k, v := range it.LastStatus {
			cp.LastStatus[k] = v
		}
	} else {
		cp.LastStatus = map[string]string{}
	}
	if it.History != nil {
		cp.History = make([]map[string]interface{}, len(it.History))
		for i, h := range it.History {
			item := make(map[string]interface{}, len(h))
			for k, v := range h {
				item[k] = v
			}
			cp.History[i] = item
		}
	} else {
		cp.History = []map[string]interface{}{}
	}
	return cp
}

func cloneVPSSubs(in []types.VPSSubscription) []types.VPSSubscription {
	out := make([]types.VPSSubscription, len(in))
	for i, it := range in {
		out[i] = cloneVPSSub(it)
	}
	return out
}
