package app

import (
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// 监控在欧区账户上看到 24sk202@gra 有货:只唤醒同型号、同机房(写法可不同)、
// 同大区、运行中的队列任务;唤醒标记取一次即清。
func TestWakeQueueForStockMatchesPlanDCAndRegion(t *testing.T) {
	s := newTestState(t)
	s.AccountsMu.Lock()
	s.Accounts = []types.OVHAccount{
		{ID: "eu", Name: "eu", Endpoint: "ovh-eu", IsDefault: true},
		{ID: "us", Name: "us", Endpoint: "ovh-us"},
	}
	s.AccountsMu.Unlock()
	s.QueueMu.Lock()
	s.Queue = []types.QueueItem{
		{ID: "hit", AccountID: "eu", PlanCode: "24sk202", Datacenter: "GRA", Status: "running"},
		{ID: "default-acc", AccountID: "", PlanCode: "24sk202", Datacenter: "gra", Status: "running"},
		{ID: "paused", AccountID: "eu", PlanCode: "24sk202", Datacenter: "gra", Status: "paused"},
		{ID: "other-dc", AccountID: "eu", PlanCode: "24sk202", Datacenter: "rbx", Status: "running"},
		{ID: "other-plan", AccountID: "eu", PlanCode: "24sk204", Datacenter: "gra", Status: "running"},
		{ID: "us-region", AccountID: "us", PlanCode: "24sk202", Datacenter: "gra", Status: "running"},
	}
	s.QueueMu.Unlock()

	if n := s.WakeQueueForStock("24sk202", "gra", "ovh-eu"); n != 2 {
		t.Fatalf("应唤醒 2 个任务, got %d", n)
	}
	for id, want := range map[string]bool{"hit": true, "default-acc": true, "paused": false, "other-dc": false, "other-plan": false, "us-region": false} {
		if got := s.TakeStockWake(id); got != want {
			t.Errorf("%s: got %v want %v", id, got, want)
		}
	}
	if s.TakeStockWake("hit") {
		t.Fatal("唤醒标记应取一次即清")
	}

	// 机房写法不同(mum = OVH 的 ynm)也要对上
	s.QueueMu.Lock()
	s.Queue = []types.QueueItem{{ID: "mum", AccountID: "eu", PlanCode: "24sk202", Datacenter: "mum", Status: "running"}}
	s.QueueMu.Unlock()
	if n := s.WakeQueueForStock("24sk202", "ynm", "ovh-eu"); n != 1 {
		t.Fatalf("mum / ynm 应视为同一机房, got %d", n)
	}
}
