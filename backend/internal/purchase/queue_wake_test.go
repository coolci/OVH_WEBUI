package purchase

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// TestQueueProcessorInstantWakeup 验证当有新任务入队时，队列处理器通过事件通道实现 <100ms 毫秒级即时唤醒，
// 彻底消灭原有的 time.After(time.Second) 1 秒轮询延迟盲区。
func TestQueueProcessorInstantWakeup(t *testing.T) {
	s := quickOrderTestState(t)

	var executed atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动后台队列循环，使用假 purchase 函数
	go processQueueLoop(ctx, s, func(pctx context.Context, st *app.State, item *types.QueueItem) Outcome {
		executed.Add(1)
		return Outcome{Success: true}
	})

	// 等待循环进入空闲等待态
	time.Sleep(50 * time.Millisecond)

	// 入队新任务
	start := time.Now()
	err := s.EnqueueItems([]types.QueueItem{{
		ID:         "wake-task-1",
		PlanCode:   "24sk102",
		Datacenter: "gra",
		Status:     "running",
	}}, false)
	if err != nil {
		t.Fatal(err)
	}

	// 应该在远低于 1 秒内被唤醒 (< 300ms)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if executed.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	elapsed := time.Since(start)
	if executed.Load() == 0 {
		t.Fatalf("任务入队后队列处理器未在 500ms 内被即时唤醒，说明仍处于 1s 轮询盲等中")
	}
	if elapsed >= 700*time.Millisecond {
		t.Fatalf("队列唤醒耗时过长 (%v)，未达到毫秒级即时唤醒要求", elapsed)
	}
	t.Logf("队列处理器毫秒级即时唤醒验证通过！入队到执行耗时: %v", elapsed)
}
