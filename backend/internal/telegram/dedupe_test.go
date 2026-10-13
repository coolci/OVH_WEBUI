package telegram

import (
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

func TestRecentSuccessDuplicateParsesNowISO(t *testing.T) {
	state := &app.State{
		History: []types.PurchaseHistoryEntry{{
			PlanCode:     "24ska01",
			Datacenter:   "gra",
			Status:       "success",
			Options:      []string{"ram-64g-noecc-2133"},
			PurchaseTime: types.NowISO(),
		}},
	}
	if !RecentSuccessDuplicate(state, "24ska01", "gra", []string{"ram-64g-noecc-2133"}) {
		t.Fatal("NowISO() 写出的时间戳应被当成 120 秒内的成功单")
	}
}

func TestRecentSuccessDuplicateIgnoresOld(t *testing.T) {
	state := &app.State{
		History: []types.PurchaseHistoryEntry{{
			PlanCode:     "24ska01",
			Datacenter:   "gra",
			Status:       "success",
			PurchaseTime: "2000-01-01T00:00:00.000000",
		}},
	}
	if RecentSuccessDuplicate(state, "24ska01", "gra", nil) {
		t.Fatal("多年前的成功单不应挡住入队")
	}
}

func TestMaxQueueLenIs500(t *testing.T) {
	if MaxQueueLen != 500 {
		t.Fatalf("MaxQueueLen=%d, want 500", MaxQueueLen)
	}
}

func TestAuditTelegramDifferentAccountNotDuplicate(t *testing.T) {
	state := &app.State{Queue: []types.QueueItem{{ID: "q-a", AccountID: "account-a", PlanCode: "plan-test", Datacenter: "gra", Options: []string{"ram-test"}, Status: "running"}}}
	result := EnqueueSingle(state, "account-b", "plan-test", "gra", []string{"ram-test"}, false)
	if !result.Success {
		t.Fatalf("account-b blocked by account-a task: %s", result.Message)
	}
}

