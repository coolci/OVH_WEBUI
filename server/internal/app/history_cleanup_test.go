package app

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/types"
)

func TestRemoveHistoryEntryIsExactAndKeepsOrderJournals(t *testing.T) {
	s := newTestState(t)
	s.History = []types.PurchaseHistoryEntry{
		{ID: "selected", AccountID: "a", OrderID: "1001"},
		{ID: "selected-sibling", AccountID: "b", OrderID: "1001"},
	}
	if err := s.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.DB.ClaimOrderPayment("a", 1001); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if err := s.DB.ChangeOrderCancellation("b", 1001, "abandon"); err != nil {
		t.Fatal(err)
	}
	s.Queue = []types.QueueItem{{ID: "untouched"}}
	for i := 0; i < 2; i++ {
		removed, err := s.RemoveHistoryEntry("selected")
		if err != nil || removed != (i == 0) {
			t.Fatal(removed, err)
		}
	}
	if err := s.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	stored, err := s.DB.ListHistory()
	if err != nil || len(stored) != 1 || stored[0].ID != "selected-sibling" || len(s.QueueSnapshot()) != 1 {
		t.Fatalf("unselected state changed: %+v (%v)", stored, err)
	}
	var journal map[string]any
	if ok, err := s.DB.GetKV(db.OrderPaymentKey("a", 1001), &journal); !ok || err != nil {
		t.Fatal("payment journal removed", err)
	}
	if action, err := s.DB.GetOrderCancellation("b", 1001); err != nil || action.AbandonedAt == "" {
		t.Fatal("cancellation journal removed", err)
	}
}

func TestRemoveHistoryEntryFailureLeavesMemoryUntouched(t *testing.T) {
	s := newTestState(t)
	s.History = []types.PurchaseHistoryEntry{{ID: "keep"}}
	s.DB.Close()
	if _, err := s.RemoveHistoryEntry("keep"); err == nil {
		t.Fatal("closed database accepted deletion")
	}
	if len(s.HistorySnapshot()) != 1 {
		t.Fatal("failed deletion changed memory")
	}
}

func TestConcurrentHistoryCleanupAndNewOrdersKeepUnselectedRecords(t *testing.T) {
	s := newTestState(t)
	const count = 12
	for i := 0; i < count; i++ {
		s.History = append(s.History, types.PurchaseHistoryEntry{ID: fmt.Sprintf("remove-%d", i)})
	}
	if err := s.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if _, err := s.RemoveHistoryEntry(fmt.Sprintf("remove-%d", i)); err != nil {
				t.Error(err)
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			s.HistoryMu.Lock()
			s.History = append(s.History, types.PurchaseHistoryEntry{ID: fmt.Sprintf("new-%d", i)})
			s.HistoryMu.Unlock()
			if err := s.SaveHistory(); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	stored, err := s.DB.ListHistory()
	if err != nil || len(stored) != count || len(s.HistorySnapshot()) != count {
		t.Fatal(len(stored), err)
	}
	for _, row := range stored {
		if len(row.ID) < 4 || row.ID[:4] != "new-" {
			t.Fatal("deleted row resurrected", row.ID)
		}
	}
}
