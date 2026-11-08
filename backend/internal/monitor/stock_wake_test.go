package monitor

import (
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// 只通知不下单的订阅发现 gra 新出现有货时,也要立刻唤醒同型号同机房的队列任务
// (队列任务自己按 60 秒间隔查库存,不唤醒就要干等)。
func TestMonitorDetectionWakesQueueTasks(t *testing.T) {
	plan := "24sk-wake"
	m, state, options, _ := multiAccountFixture(t, plan, "IE", false)
	account := state.Accounts[0].ID
	state.QueueMu.Lock()
	state.Queue = []types.QueueItem{
		{ID: "q-gra", AccountID: account, PlanCode: plan, Datacenter: "gra", Status: "running", RetryInterval: 60},
		{ID: "q-rbx", AccountID: account, PlanCode: plan, Datacenter: "rbx", Status: "running", RetryInterval: 60},
	}
	state.QueueMu.Unlock()
	m.subscriptions = []*Subscription{{PlanCode: plan, Datacenters: []string{"gra"}, Options: options}}
	m.CheckAvailabilityChange(m.subscriptions[0], "stock-wake")

	if !state.TakeStockWake("q-gra") {
		t.Fatal("gra 新出现有货,同型号 gra 的队列任务应被唤醒")
	}
	if state.TakeStockWake("q-rbx") {
		t.Fatal("rbx 没有货,不该被唤醒")
	}
}
