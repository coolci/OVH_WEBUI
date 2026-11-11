package purchase

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// runQueueOnce 跑处理器直到 fake 购买被调用一次并返回、结果写回队列,然后停掉循环。
func runQueueOnce(t *testing.T, st *app.State, buy func(context.Context, *app.State, *types.QueueItem) Outcome, settled func(types.QueueItem) bool) types.QueueItem {
	t.Helper()
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		processQueueLoop(ctx, st, func(c context.Context, s *app.State, it *types.QueueItem) Outcome {
			if calls.Add(1) > 1 {
				return Outcome{}
			}
			return buy(c, s, it)
		})
	}()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st.QueueMu.Lock()
		item := st.Queue[0]
		st.QueueMu.Unlock()
		if calls.Load() >= 1 && settled(item) {
			return item
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("队列任务未在期限内得出结果")
	return types.QueueItem{}
}

func dbUncertain(t *testing.T, st *app.State, id string) bool {
	t.Helper()
	rows, err := st.DB.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == id {
			return r.UncertainCheckout
		}
	}
	t.Fatalf("库里没有任务 %s", id)
	return false
}

// 结账前必须先把"未决"落库:结账请求不接受取消,进程在它返回前退出时,
// 重启后要先对账而不是再买一台。结论明确后标记要清掉。
func TestQueueMarksUncertainBeforeCheckoutAndClearsAfter(t *testing.T) {
	st := queueCompletionState(t)
	st.Queue = []types.QueueItem{{ID: "t1", PlanCode: "plan", Status: "running", RetryInterval: 60}}
	if err := st.SaveQueue(); err != nil {
		t.Fatal(err)
	}
	var duringCheckout atomic.Bool
	item := runQueueOnce(t, st, func(ctx context.Context, s *app.State, it *types.QueueItem) Outcome {
		if err := saveCheckoutJournal(ctx, "cart-1"); err != nil {
			t.Errorf("结账凭据保存失败: %v", err)
		}
		duringCheckout.Store(dbUncertain(t, s, it.ID))
		return Outcome{Attempted: true} // 结账被明确拒绝
	}, func(it types.QueueItem) bool { return it.LastCheckTime > 0 && !it.UncertainCheckout })
	if !duringCheckout.Load() {
		t.Fatal("结账请求发出前库里没有未决标记,进程此刻退出会导致重启后重复购买")
	}
	if item.UncertainCheckout || dbUncertain(t, st, "t1") {
		t.Fatal("明确失败后未决标记应被清除")
	}
}

// 未决标记以前只在快照上清,队列里的任务永远带着它,以后每轮都先对账、对账失败就自我暂停。
func TestQueueClearsStaleUncertainAfterDefiniteOutcome(t *testing.T) {
	st := queueCompletionState(t)
	st.Queue = []types.QueueItem{{ID: "t2", PlanCode: "plan", Status: "running", RetryInterval: 60, UncertainCheckout: true, UncertainTime: types.NowISO()}}
	if err := st.SaveQueue(); err != nil {
		t.Fatal(err)
	}
	item := runQueueOnce(t, st, func(context.Context, *app.State, *types.QueueItem) Outcome {
		return Outcome{} // 对账确认无单,本轮无货
	}, func(it types.QueueItem) bool { return !it.UncertainCheckout })
	if item.Status != "running" || dbUncertain(t, st, "t2") {
		t.Fatalf("未决标记未落库清除: %+v", item)
	}
}

// 优惠码被拒这类"根本没结账"的暂停不能标成结账未决,
// 否则恢复时会去对账,把用户自己下的同型号单误认成本任务的。
func TestQueuePauseWithoutCheckoutIsNotUncertain(t *testing.T) {
	st := queueCompletionState(t)
	st.Queue = []types.QueueItem{{ID: "t3", PlanCode: "plan", Status: "running", RetryInterval: 60}}
	if err := st.SaveQueue(); err != nil {
		t.Fatal(err)
	}
	item := runQueueOnce(t, st, func(context.Context, *app.State, *types.QueueItem) Outcome {
		return Outcome{PauseTask: true, Reason: "优惠码被拒"}
	}, func(it types.QueueItem) bool { return it.Status == "paused" })
	if item.UncertainCheckout || dbUncertain(t, st, "t3") {
		t.Fatal("没有结账的暂停被误标成结账未决")
	}
}

// 真正的结账未决才保留标记
func TestQueueCheckoutUncertainKeepsFlag(t *testing.T) {
	st := queueCompletionState(t)
	st.Queue = []types.QueueItem{{ID: "t4", PlanCode: "plan", Status: "running", RetryInterval: 60}}
	if err := st.SaveQueue(); err != nil {
		t.Fatal(err)
	}
	item := runQueueOnce(t, st, func(ctx context.Context, s *app.State, it *types.QueueItem) Outcome {
		_ = saveCheckoutJournal(ctx, "cart-4")
		return Outcome{PauseTask: true, CheckoutUncertain: true}
	}, func(it types.QueueItem) bool { return it.Status == "paused" })
	if !item.UncertainCheckout || !dbUncertain(t, st, "t4") {
		t.Fatal("结账未决的任务必须带着标记暂停并落库")
	}
}
