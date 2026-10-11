package order_test

import (
	"errors"
	"testing"

	"github.com/ovh-webui/server/internal/service/order"
	"github.com/ovh-webui/server/internal/types"
)

// fakeStore 内存版 QueueStore + HistoryStore,可注入落库失败。
type fakeStore struct {
	queue    []types.QueueItem
	history  []types.PurchaseHistoryEntry
	saveErr  error
	deleted  []string
	enqueued int
}

func (f *fakeStore) QueueSnapshot() []types.QueueItem {
	return append([]types.QueueItem{}, f.queue...)
}

func (f *fakeStore) EnqueueItems(items []types.QueueItem, prepend bool) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.enqueued += len(items)
	if prepend {
		f.queue = append(append([]types.QueueItem{}, items...), f.queue...)
	} else {
		f.queue = append(f.queue, items...)
	}
	return nil
}

func (f *fakeStore) RemoveQueueItem(id string) (*types.QueueItem, error) {
	f.deleted = append(f.deleted, id)
	var removed *types.QueueItem
	kept := f.queue[:0:0]
	for _, it := range f.queue {
		if it.ID == id {
			cp := it
			removed = &cp
			continue
		}
		kept = append(kept, it)
	}
	f.queue = kept
	return removed, f.saveErr
}

func (f *fakeStore) ClearQueueItems() ([]types.QueueItem, error) {
	old := f.queue
	f.queue = []types.QueueItem{}
	return old, f.saveErr
}

func (f *fakeStore) MutateQueueItem(id string, fn func(*types.QueueItem)) (types.QueueItem, bool, error) {
	for i := range f.queue {
		if f.queue[i].ID == id {
			fn(&f.queue[i])
			return f.queue[i], true, f.saveErr
		}
	}
	return types.QueueItem{}, false, nil
}

func (f *fakeStore) HistorySnapshot() []types.PurchaseHistoryEntry {
	return append([]types.PurchaseHistoryEntry{}, f.history...)
}

func (f *fakeStore) ClearHistoryEntries() error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.history = nil
	return nil
}

func newSvc() (*fakeStore, order.Service) {
	st := &fakeStore{
		queue:   []types.QueueItem{{ID: "t1", PlanCode: "24ska01", Datacenter: "gra", Status: "running", RetryInterval: 30}},
		history: []types.PurchaseHistoryEntry{{ID: "h1", Status: "success"}},
	}
	return st, order.NewService(st, st)
}

func TestUpdateQueueInterval(t *testing.T) {
	st, svc := newSvc()

	if err := svc.UpdateQueueInterval("t1", 60); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.queue[0].RetryInterval != 60 {
		t.Fatalf("interval = %d, want 60 (seconds, no unit conversion)", st.queue[0].RetryInterval)
	}

	for _, bad := range []int{types.MinRetryInterval - 1, types.MaxRetryInterval + 1} {
		if err := svc.UpdateQueueInterval("t1", bad); !errors.Is(err, order.ErrInvalidInterval) {
			t.Errorf("interval %d: err = %v, want ErrInvalidInterval", bad, err)
		}
	}
	if err := svc.UpdateQueueInterval("missing", 60); !errors.Is(err, order.ErrItemNotFound) {
		t.Errorf("missing: err = %v, want ErrItemNotFound", err)
	}

	st.saveErr = errors.New("disk full")
	if err := svc.UpdateQueueInterval("t1", 90); !errors.Is(err, order.ErrPersist) {
		t.Errorf("save failure: err = %v, want ErrPersist", err)
	}
}

func TestUpdateQueueStatus(t *testing.T) {
	st, svc := newSvc()

	item, err := svc.UpdateQueueStatus("t1", "paused")
	if err != nil || item.Status != "paused" || st.queue[0].Status != "paused" {
		t.Fatalf("pause: item=%+v err=%v", item, err)
	}
	// 空状态兼容旧客户端 → pending
	if item, err = svc.UpdateQueueStatus("t1", ""); err != nil || item.Status != "pending" {
		t.Fatalf("empty status: item=%+v err=%v", item, err)
	}
	// completed / failed 只能由处理器写,用户接口必须拒绝
	for _, bad := range []string{"completed", "failed", "bogus"} {
		if _, err := svc.UpdateQueueStatus("t1", bad); !errors.Is(err, order.ErrInvalidStatus) {
			t.Errorf("status %q: err = %v, want ErrInvalidStatus", bad, err)
		}
	}
	if _, err := svc.UpdateQueueStatus("missing", "paused"); !errors.Is(err, order.ErrItemNotFound) {
		t.Errorf("missing: err = %v, want ErrItemNotFound", err)
	}
}

func TestRemoveAndClear(t *testing.T) {
	st, svc := newSvc()

	removed, err := svc.RemoveQueueItem("t1")
	if err != nil || removed == nil || removed.ID != "t1" || len(st.queue) != 0 {
		t.Fatalf("remove: removed=%v err=%v len=%d", removed, err, len(st.queue))
	}
	// 不存在的任务:不报错、返回 nil,但仍要求 store 取消它可能在跑的下单
	removed, err = svc.RemoveQueueItem("ghost")
	if err != nil || removed != nil {
		t.Fatalf("remove ghost: removed=%v err=%v", removed, err)
	}
	if len(st.deleted) != 2 || st.deleted[1] != "ghost" {
		t.Errorf("store.RemoveQueueItem not called for ghost: %v", st.deleted)
	}

	_ = svc.Enqueue(types.QueueItem{ID: "a"}, false)
	_ = svc.Enqueue(types.QueueItem{ID: "b"}, true)
	if got := svc.ListQueue(); len(got) != 2 || got[0].ID != "b" {
		t.Fatalf("prepend order wrong: %+v", got)
	}
	old, err := svc.ClearQueue()
	if err != nil || len(old) != 2 || len(svc.ListQueue()) != 0 {
		t.Fatalf("clear: old=%d err=%v", len(old), err)
	}
}

func TestHistory(t *testing.T) {
	st, svc := newSvc()
	if len(svc.ListHistory()) != 1 {
		t.Fatal("expected 1 history entry")
	}
	st.saveErr = errors.New("locked")
	if err := svc.ClearHistory(); !errors.Is(err, order.ErrPersist) {
		t.Fatalf("err = %v, want ErrPersist", err)
	}
	st.saveErr = nil
	if err := svc.ClearHistory(); err != nil || len(svc.ListHistory()) != 0 {
		t.Fatalf("clear history: err=%v", err)
	}
}
