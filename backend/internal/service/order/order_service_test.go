package order_test

import (
	"testing"

	"github.com/ovh-webui/server/internal/service/order"
	"github.com/ovh-webui/server/internal/types"
)

type mockQueueRepo struct {
	items []types.QueueItem
}

func (m *mockQueueRepo) ListQueue() ([]types.QueueItem, error) {
	return m.items, nil
}

func (m *mockQueueRepo) ReplaceQueue(items []types.QueueItem) error {
	m.items = items
	return nil
}

func (m *mockQueueRepo) DeleteQueueItem(id string) error {
	var next []types.QueueItem
	for _, it := range m.items {
		if it.ID != id {
			next = append(next, it)
		}
	}
	m.items = next
	return nil
}

func (m *mockQueueRepo) ClearQueue() (int64, error) {
	n := int64(len(m.items))
	m.items = nil
	return n, nil
}

type mockHistoryRepo struct {
	items []types.PurchaseHistoryEntry
}

func (m *mockHistoryRepo) ListHistory() ([]types.PurchaseHistoryEntry, error) {
	return m.items, nil
}

func (m *mockHistoryRepo) ReplaceHistory(items []types.PurchaseHistoryEntry) error {
	m.items = items
	return nil
}

func (m *mockHistoryRepo) ClearHistory() (int64, error) {
	n := int64(len(m.items))
	m.items = nil
	return n, nil
}

func TestOrderService(t *testing.T) {
	qRepo := &mockQueueRepo{
		items: []types.QueueItem{
			{
				ID:            "task-1",
				PlanCode:      "24ska01",
				Datacenter:    "gra",
				Status:        "queued",
				RetryInterval: 1,
			},
		},
	}
	hRepo := &mockHistoryRepo{
		items: []types.PurchaseHistoryEntry{
			{
				ID:       "hist-1",
				PlanCode: "24ska01",
				Status:   "success",
			},
		},
	}

	svc := order.NewService(qRepo, hRepo)

	// ListQueue
	list, err := svc.ListQueue()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 item, got %d, err=%v", len(list), err)
	}

	// UpdateQueueInterval
	if err := svc.UpdateQueueInterval("task-1", 2000); err != nil {
		t.Fatalf("unexpected error updating interval: %v", err)
	}
	if qRepo.items[0].RetryInterval != 2 {
		t.Errorf("expected retry interval 2s, got %d", qRepo.items[0].RetryInterval)
	}

	// UpdateQueueInterval invalid interval rejection
	if err := svc.UpdateQueueInterval("task-1", 100); err == nil {
		t.Fatalf("expected error for interval < 500ms, got nil")
	}

	// RemoveQueueItem
	if err := svc.RemoveQueueItem("task-1"); err != nil {
		t.Fatalf("unexpected error removing item: %v", err)
	}
	if len(qRepo.items) != 0 {
		t.Errorf("expected 0 items after remove, got %d", len(qRepo.items))
	}

	// ListHistory
	hList, err := svc.ListHistory()
	if err != nil || len(hList) != 1 {
		t.Fatalf("expected 1 history item, got %d, err=%v", len(hList), err)
	}

	// ClearHistory
	cleared, err := svc.ClearHistory()
	if err != nil || cleared != 1 {
		t.Fatalf("expected 1 cleared, got %d, err=%v", cleared, err)
	}
}
