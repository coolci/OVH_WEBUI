package app

import (
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// 入队这一个共同入口统一规整购买时长:空 → P1M,P1Y → P12M,非法值整批拒绝且不落库。
func TestEnqueueNormalizesDuration(t *testing.T) {
	s := newTestState(t)
	items := []types.QueueItem{
		{ID: "a", AccountID: "x", PlanCode: "p", Datacenter: "gra", Status: "paused"},
		{ID: "b", AccountID: "x", PlanCode: "p", Datacenter: "gra", Status: "paused", Duration: "p1y"},
	}
	if err := s.EnqueueItems(items, false); err != nil {
		t.Fatal(err)
	}
	s.QueueMu.Lock()
	got := map[string]string{}
	for _, q := range s.Queue {
		got[q.ID] = q.Duration
	}
	s.QueueMu.Unlock()
	if got["a"] != "P1M" || got["b"] != "P12M" {
		t.Fatalf("normalized durations: %v", got)
	}
	if err := s.EnqueueItems([]types.QueueItem{
		{ID: "ok", AccountID: "x", PlanCode: "p", Datacenter: "gra", Status: "paused"},
		{ID: "bad", AccountID: "x", PlanCode: "p", Datacenter: "gra", Status: "paused", Duration: "P0M"},
	}, false); err == nil {
		t.Fatal("invalid duration must reject the whole batch")
	}
	s.QueueMu.Lock()
	n := len(s.Queue)
	s.QueueMu.Unlock()
	if n != 2 {
		t.Fatalf("rejected batch must not be partially enqueued, queue=%d", n)
	}
}

func TestUpdateQueueItemDuration(t *testing.T) {
	s := newTestState(t)
	if err := s.EnqueueItems([]types.QueueItem{{ID: "t", AccountID: "x", PlanCode: "p", Datacenter: "gra", Status: "paused", Duration: "P12M"}}, false); err != nil {
		t.Fatal(err)
	}
	get := func() string {
		s.QueueMu.Lock()
		defer s.QueueMu.Unlock()
		return s.Queue[0].Duration
	}
	// nil = 旧客户端没传,保持原值
	if err := s.UpdateQueueItem("t", "gra", nil, 30, false, "", nil); err != nil || get() != "P12M" {
		t.Fatalf("nil duration must keep P12M: got %s err=%v", get(), err)
	}
	d := "P24M"
	if err := s.UpdateQueueItem("t", "gra", nil, 30, false, "", &d); err != nil || get() != "P24M" {
		t.Fatalf("update to P24M: got %s err=%v", get(), err)
	}
	bad := "monthly"
	if err := s.UpdateQueueItem("t", "gra", nil, 30, false, "", &bad); err == nil || get() != "P24M" {
		t.Fatalf("invalid duration must be rejected without change: got %s err=%v", get(), err)
	}
	persisted, err := s.DB.ListQueue()
	if err != nil || len(persisted) != 1 || persisted[0].Duration != "P24M" {
		t.Fatalf("edit must be persisted: %+v err=%v", persisted, err)
	}
}
