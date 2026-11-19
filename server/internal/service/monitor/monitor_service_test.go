package monitor_test

import (
	"testing"

	"github.com/ovh-webui/server/internal/service/monitor"
	"github.com/ovh-webui/server/internal/types"
)

type mockMonitorRepo struct {
	subs map[string]types.Subscription
}

func (m *mockMonitorRepo) ListMonitorSubscriptions() ([]types.Subscription, error) {
	var list []types.Subscription
	for _, s := range m.subs {
		list = append(list, s)
	}
	return list, nil
}

func (m *mockMonitorRepo) UpsertMonitorSubscription(sub types.Subscription) error {
	m.subs[sub.PlanCode] = sub
	return nil
}

func (m *mockMonitorRepo) DeleteMonitorSubscription(planCode string) error {
	delete(m.subs, planCode)
	return nil
}

func (m *mockMonitorRepo) ClearMonitorSubscriptions() (int64, error) {
	n := int64(len(m.subs))
	m.subs = make(map[string]types.Subscription)
	return n, nil
}

func TestMonitorService(t *testing.T) {
	repo := &mockMonitorRepo{subs: make(map[string]types.Subscription)}
	svc := monitor.NewService(repo)

	// Save subscription
	sub := types.Subscription{
		PlanCode:    "24ska01",
		Datacenters: []string{"gra", "rbx"},
	}
	if err := svc.SaveSubscription(sub); err != nil {
		t.Fatalf("unexpected error saving subscription: %v", err)
	}

	// Empty plan code validation failure
	if err := svc.SaveSubscription(types.Subscription{PlanCode: ""}); err == nil {
		t.Fatalf("expected error for empty plan code, got nil")
	}

	// List subscriptions
	list, err := svc.ListSubscriptions()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 subscription, got %d, err=%v", len(list), err)
	}

	// Delete subscription
	if err := svc.DeleteSubscription("24ska01"); err != nil {
		t.Fatalf("unexpected error deleting subscription: %v", err)
	}
	list, _ = svc.ListSubscriptions()
	if len(list) != 0 {
		t.Errorf("expected 0 subscriptions after delete, got %d", len(list))
	}
}
