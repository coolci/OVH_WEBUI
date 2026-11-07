package purchase

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// 一个任务卡在长时间结账里时,其他到点的任务必须照常派发;同一任务不能并发执行;
// 循环退出要等在途任务返回(Supervisor.Stop 依赖这一点再关库)。
func TestSlowCheckoutDoesNotBlockOtherTasks(t *testing.T) {
	st := queueCompletionState(t)
	st.Queue = []types.QueueItem{
		{ID: "slow", PlanCode: "slow-plan", Status: "running", RetryInterval: 1},
		{ID: "fast", PlanCode: "fast-plan", Status: "running", RetryInterval: 1},
	}
	if err := st.SaveQueue(); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	var fastCalls, slowActive, slowMax atomic.Int32
	var once sync.Once
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		processQueueLoop(ctx, st, func(_ context.Context, _ *app.State, item *types.QueueItem) Outcome {
			if item.ID == "slow" {
				n := slowActive.Add(1)
				if n > slowMax.Load() {
					slowMax.Store(n)
				}
				<-release
				slowActive.Add(-1)
				return Outcome{}
			}
			fastCalls.Add(1)
			return Outcome{}
		})
	}()
	t.Cleanup(func() { once.Do(func() { close(release) }); cancel(); <-done })

	// 慢任务一直卡着;快任务 1 秒间隔应在这期间被派发至少两次
	deadline := time.Now().Add(5 * time.Second)
	for fastCalls.Load() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("fast task starved while slow checkout in flight: fast=%d", fastCalls.Load())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if slowMax.Load() != 1 {
		t.Fatalf("slow task ran concurrently with itself: max=%d", slowMax.Load())
	}

	// 停止时必须等慢任务返回才退出
	cancel()
	select {
	case <-done:
		t.Fatal("loop exited while a checkout was still in flight")
	case <-time.After(200 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("loop did not exit after in-flight work finished")
	}
}
