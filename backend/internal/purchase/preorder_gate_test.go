package purchase

import (
	"context"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/types"
)

// 同型号同配置按机房 × 数量扇出的多个预订任务,开售前只需一个真打 OVH:
// 以前每个任务每轮约 6 次请求(查库存、建车、绑定、加购被拒、删车…),N 个任务就是 6N 次。
func TestPreorderSiblingsShareOneProbe(t *testing.T) {
	s, f := newPreorderFixture(t)
	task := func(id, dc string) *types.QueueItem {
		it := preorderTask()
		it.ID, it.Datacenter = id, dc
		return it
	}
	first := PurchaseServer(context.Background(), s, task("t1", "gra"))
	if !first.WaitingForCatalog {
		t.Fatalf("not launched yet: %+v", first)
	}
	afterProbe := f.apiCalls.Load()
	for _, id := range []string{"t2", "t3", "t4"} {
		out := PurchaseServer(context.Background(), s, task(id, "rbx"))
		if !out.WaitingForCatalog || out.Attempted || out.Fatal {
			t.Fatalf("sibling must just wait: %+v", out)
		}
	}
	if extra := f.apiCalls.Load() - afterProbe; extra != 0 {
		t.Fatalf("siblings hit OVH %d times while another task just confirmed it is not open", extra)
	}

	// 开售信号(目录 / 库存唤醒)无视闸门,立即真试并下单
	f.set(func(f *preorderFixture) { f.launched, f.optionsOpen = true, true })
	out := PurchaseServer(WithStockWoken(context.Background()), s, task("t2", "gra"))
	if !out.Success || f.checkouts.Load() != 1 {
		t.Fatalf("a woken sibling must order right away: %+v checkouts=%d", out, f.checkouts.Load())
	}
	// 成功后闸门打开,其它任务也不会再被拦
	if preorderGateClosed(preorderGateKey(task("t3", "rbx"), "P1M"), "t3", time.Now()) {
		t.Fatal("gate must open after a successful order")
	}
}
