package catalogwatch

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

// 用户已为新机型建了预订任务:开售(进入销售目录)那一刻,目录上新不能再为同一型号同一机房
// 自动下一单 —— 以前两边各下一单,用户要 1 台却买到 2 台。
func TestCatalogWatchDefersToExistingQueueTask(t *testing.T) {
	w, s := fixture(t)
	s.Queue = []types.QueueItem{{ID: "pre", AccountID: "a", PlanCode: "new", Datacenter: "gra", Options: []string{"ram-64"}, Status: "running", Preorder: true}}
	var bought atomic.Int32
	w.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
		bought.Add(1)
		return purchase.Outcome{Success: true}
	}
	ready(t, w)
	// 开售信号:同型号同机房的队列任务被唤醒,不用等满预订轮询间隔
	if !s.TakeStockWake("pre") {
		t.Fatal("catalog launch must wake the preorder task immediately")
	}
	w.Attempt(context.Background())
	if bought.Load() != 0 {
		t.Fatalf("catalog watch ordered %d time(s) although a queue task owns new@gra", bought.Load())
	}
	j := sole(t, w)
	if j.Status != "cancelled" {
		t.Fatalf("job should step aside: %+v", j)
	}
}

// 没有队列任务负责的新组合照常由目录上新下单
func TestCatalogWatchStillOrdersUncoveredLaunch(t *testing.T) {
	w, s := fixture(t)
	s.Queue = []types.QueueItem{{ID: "other-dc", AccountID: "a", PlanCode: "new", Datacenter: "rbx", Status: "running"}, {ID: "done", AccountID: "a", PlanCode: "new", Datacenter: "gra", Status: "completed"}}
	var bought atomic.Int32
	w.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
		bought.Add(1)
		return purchase.Outcome{Success: true}
	}
	ready(t, w)
	w.Attempt(context.Background())
	if bought.Load() != 1 {
		t.Fatalf("uncovered combination must still be ordered once, got %d", bought.Load())
	}
}
