package purchase

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// 刚查过、重试间隔 60 秒的队列任务,在监控发现同型号有货后应立刻再查一次,
// 而不是等满 60 秒。
func TestStockWakeSkipsRetryInterval(t *testing.T) {
	s := quickOrderTestState(t)
	var runs atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.QueueMu.Lock()
	s.Queue = []types.QueueItem{{
		ID: "waiting", PlanCode: "24sk202", Datacenter: "gra", Status: "running",
		RetryInterval: 60, LastCheckTime: float64(time.Now().Unix()), // 刚查过
	}}
	s.QueueMu.Unlock()

	go processQueueLoop(ctx, s, func(context.Context, *app.State, *types.QueueItem) Outcome {
		runs.Add(1)
		return Outcome{}
	})
	time.Sleep(300 * time.Millisecond)
	if runs.Load() != 0 {
		t.Fatal("没唤醒时不应提前检查(间隔 60 秒)")
	}

	start := time.Now()
	if n := s.WakeQueueForStock("24sk202", "gra", ""); n != 1 {
		t.Fatalf("应唤醒 1 个任务, got %d", n)
	}
	deadline := time.Now().Add(time.Second)
	for runs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if runs.Load() == 0 {
		t.Fatal("监控唤醒后任务没有立即检查")
	}
	t.Logf("唤醒到开始检查: %v", time.Since(start))

	time.Sleep(300 * time.Millisecond)
	if runs.Load() != 1 {
		t.Fatalf("一次唤醒只应多查一次, got %d", runs.Load())
	}
}
