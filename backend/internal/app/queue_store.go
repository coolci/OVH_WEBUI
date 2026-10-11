package app

import "github.com/ovh-webui/server/internal/types"

// 队列 / 历史的"内存 + 落库"存储操作。
//
// 运行时内存(State.Queue / State.History)才是真相源:处理器每一轮都在内存里改
// retryCount、lastCheckTime、status,然后整表落库。所以上层(service/order)
// 绝不能绕过内存直接写库 —— 否则下一次 SaveQueue 会用内存快照把它写的东西覆盖掉。
// 这里把"改内存 + 落库"封装成原子语义,service 只依赖这组方法(见 order.QueueStore)。

// QueueSnapshot 返回队列副本(永不为 nil,序列化为 [] 而不是 null)。
func (s *State) QueueSnapshot() []types.QueueItem {
	s.QueueMu.Lock()
	defer s.QueueMu.Unlock()
	cp := make([]types.QueueItem, len(s.Queue))
	copy(cp, s.Queue)
	return cp
}

// RemoveQueueItem 删除一条任务并落库。
//
// 先 MarkTaskDeleted(打标记 + 取消正在进行的下单),再从内存移除 —— 顺序与原 handler 一致:
// 即使这条任务已经不在队列里,也要确保正在跑的那一轮被取消。
// 返回被删的副本;不存在时返回 nil。err 非 nil 表示内存已删但落库失败。
func (s *State) RemoveQueueItem(id string) (*types.QueueItem, error) {
	s.MarkTaskDeleted(id)

	s.QueueMu.Lock()
	var removed *types.QueueItem
	// 重新分配新 slice,避免与原 backing array 共享导致快照读到已被覆盖的元素
	kept := make([]types.QueueItem, 0, len(s.Queue))
	for i := range s.Queue {
		if s.Queue[i].ID == id {
			cp := s.Queue[i]
			removed = &cp
			continue
		}
		kept = append(kept, s.Queue[i])
	}
	s.Queue = kept
	s.QueueMu.Unlock()

	return removed, s.SaveQueue()
}

// ClearQueueItems 清空队列并落库,返回被清掉的任务副本。
// 每条都会 MarkTaskDeleted,取消正在进行的下单。
func (s *State) ClearQueueItems() ([]types.QueueItem, error) {
	s.QueueMu.Lock()
	old := append([]types.QueueItem{}, s.Queue...)
	for _, it := range s.Queue {
		s.MarkTaskDeleted(it.ID)
	}
	s.Queue = []types.QueueItem{}
	s.QueueMu.Unlock()

	return old, s.SaveQueue()
}

// MutateQueueItem 在队列锁内对指定任务执行 fn,然后落库。
//
// found=false 表示任务不存在(不会落库)。found=true 且 err!=nil 表示内存已改、落库失败 ——
// 不回滚:回滚会把处理器在这期间写入的 retryCount / status 一起抹掉,得不偿失。
func (s *State) MutateQueueItem(id string, fn func(*types.QueueItem)) (types.QueueItem, bool, error) {
	s.QueueMu.Lock()
	var out types.QueueItem
	found := false
	for i := range s.Queue {
		if s.Queue[i].ID == id {
			fn(&s.Queue[i])
			s.Queue[i].UpdatedAt = types.NowISO()
			out = s.Queue[i]
			found = true
			break
		}
	}
	s.QueueMu.Unlock()
	if !found {
		return types.QueueItem{}, false, nil
	}
	return out, true, s.SaveQueue()
}

// HistorySnapshot 返回购买历史副本(永不为 nil)。
func (s *State) HistorySnapshot() []types.PurchaseHistoryEntry {
	s.HistoryMu.Lock()
	defer s.HistoryMu.Unlock()
	cp := make([]types.PurchaseHistoryEntry, len(s.History))
	copy(cp, s.History)
	return cp
}

// ClearHistoryEntries 清空购买历史并落库;落库失败时回滚内存。
//
// 回滚是拼接而不是覆盖:清空到回滚之间下单完成会 append 新记录,不能丢。
// 不回滚的话界面是空的、库里是满的,下一次任何写历史的操作都会按"空"把整表覆盖掉。
func (s *State) ClearHistoryEntries() error {
	s.HistoryMu.Lock()
	backup := make([]types.PurchaseHistoryEntry, len(s.History))
	copy(backup, s.History)
	s.History = []types.PurchaseHistoryEntry{}
	s.HistoryMu.Unlock()

	if err := s.SaveHistory(); err != nil {
		s.HistoryMu.Lock()
		s.History = append(backup, s.History...)
		s.HistoryMu.Unlock()
		return err
	}
	return nil
}

// ReloadQueueFromDB 从数据库重载队列并同步清理被级联删除的运行中任务。
func (s *State) ReloadQueueFromDB() error {
	items, err := s.DB.ListQueue()
	if err != nil {
		return err
	}
	s.QueueMu.Lock()
	alive := make(map[string]struct{}, len(items))
	for _, it := range items {
		alive[it.ID] = struct{}{}
	}
	gone := []string{}
	for _, it := range s.Queue {
		if _, ok := alive[it.ID]; !ok {
			gone = append(gone, it.ID)
		}
	}
	if items == nil {
		s.Queue = []types.QueueItem{}
	} else {
		s.Queue = items
	}
	s.QueueMu.Unlock()

	for _, id := range gone {
		s.MarkTaskDeleted(id)
	}
	return nil
}

// ReloadHistoryFromDB 从数据库重载购买历史到内存。
func (s *State) ReloadHistoryFromDB() error {
	items, err := s.DB.ListHistory()
	if err != nil {
		return err
	}
	s.HistoryMu.Lock()
	if items == nil {
		s.History = []types.PurchaseHistoryEntry{}
	} else {
		s.History = items
	}
	s.HistoryMu.Unlock()
	return nil
}

