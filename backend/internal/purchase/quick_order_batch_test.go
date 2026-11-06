package purchase

import (
	"errors"
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

func TestMonitorQuickOrderBatchIsAccountScopedAndAtomic(t *testing.T) {
	st := quickOrderTestState(t)
	// Missing credentials deliberately prevent any external request. Monitor
	// inputs represent configurations already validated by its price check.
	st.Accounts = []types.OVHAccount{{ID: "batch-a"}, {ID: "batch-b"}}
	params := QuickOrderParams{PlanCode: "custom-batch-plan", Datacenter: "gra", Force: true,
		FromMonitor: true, SkipDuplicateCheck: true, Quantity: 2, AutoPay: true}
	for _, accountID := range []string{"batch-a", "batch-b"} {
		params.AccountID = accountID
		if result := EnqueueQuickOrder(st, params); !result.Success {
			t.Fatalf("%s: %+v", accountID, result)
		}
	}
	queue, err := st.DB.ListQueue()
	if err != nil || len(queue) != 4 {
		t.Fatalf("two accounts x two units must persist four tasks: %+v, %v", queue, err)
	}
	counts, ids := map[string]int{}, map[string]bool{}
	for _, item := range queue {
		counts[item.AccountID]++
		if ids[item.ID] || !item.Force || !item.AutoPay || item.PlanCode != params.PlanCode || item.Datacenter != "gra" {
			t.Fatalf("invalid batch task: %+v", item)
		}
		ids[item.ID] = true
	}
	if counts["batch-a"] != 2 || counts["batch-b"] != 2 {
		t.Fatalf("incorrect account distribution: %+v", counts)
	}
	if result := EnqueueQuickOrder(st, params); result.Success || len(st.QueueSnapshot()) != 4 {
		t.Fatalf("repeated monitor window must not create extra tasks: %+v", result)
	}
	if _, err := st.OrderService.UpdateQueueStatus(queue[0].ID, "paused"); err != nil {
		t.Fatal(err)
	}
	params.AccountID = queue[0].AccountID
	if result := EnqueueQuickOrder(st, params); result.Success || len(st.QueueSnapshot()) != 4 {
		t.Fatalf("paused account must not be restarted by another batch: %+v", result)
	}
}

func TestMonitorQuickOrderBatchLimitsAndRollback(t *testing.T) {
	for _, tc := range []struct {
		name           string
		quantity, want int
		breakDB        bool
	}{
		{"default quantity", 0, 1, false},
		{"quantity limit", types.MaxOrderQuantity + 1, types.MaxOrderQuantity, false},
		{"failed persistence", 3, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := quickOrderTestState(t)
			st.Accounts = []types.OVHAccount{{ID: tc.name}}
			if tc.breakDB {
				if _, err := st.DB.Exec("DROP TABLE queue"); err != nil {
					t.Fatal(err)
				}
			}
			result := EnqueueQuickOrder(st, QuickOrderParams{AccountID: tc.name, PlanCode: "batch-limit", Datacenter: "gra",
				Options: []string{"ram-64g"}, FromMonitor: true, Quantity: tc.quantity})
			if result.Success == tc.breakDB || len(st.QueueSnapshot()) != tc.want {
				t.Fatalf("batch published incorrectly: %+v; tasks=%d", result, len(st.QueueSnapshot()))
			}
		})
	}
}

func TestMonitorQuickOrderRechecksGenerationBeforeEnqueue(t *testing.T) {
	st := quickOrderTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "stale-generation"}}
	result := EnqueueQuickOrder(st, QuickOrderParams{AccountID: "stale-generation", PlanCode: "custom-stale", Datacenter: "gra",
		Force: true, FromMonitor: true, Quantity: 2, GuardEnqueue: func(func() error) error { return errors.New("subscription changed") }})
	if result.Success || len(st.QueueSnapshot()) != 0 {
		t.Fatalf("obsolete monitor check created tasks: %+v", result)
	}
}
