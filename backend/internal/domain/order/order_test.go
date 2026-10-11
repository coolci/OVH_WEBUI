package order_test

import (
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/domain/order"
)

func TestNewQueueItem_Validation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		id         string
		accountID  string
		planCode   string
		datacenter string
		quantity   int
		intervalMs int
		autoPay    bool
		wantErr    error
	}{
		{
			name:       "valid queue item",
			id:         "item-1",
			accountID:  "acc-1",
			planCode:   "24ska01",
			datacenter: "gra",
			quantity:   1,
			intervalMs: 1000,
			autoPay:    true,
			wantErr:    nil,
		},
		{
			name:       "empty plan code",
			id:         "item-2",
			accountID:  "acc-1",
			planCode:   "",
			datacenter: "gra",
			quantity:   1,
			intervalMs: 1000,
			autoPay:    false,
			wantErr:    order.ErrEmptyPlanCode,
		},
		{
			name:       "empty account id",
			id:         "item-3",
			accountID:  "",
			planCode:   "24ska01",
			datacenter: "gra",
			quantity:   1,
			intervalMs: 1000,
			autoPay:    false,
			wantErr:    order.ErrEmptyAccountID,
		},
		{
			name:       "quantity out of upper bounds",
			id:         "item-4",
			accountID:  "acc-1",
			planCode:   "24ska01",
			datacenter: "gra",
			quantity:   21,
			intervalMs: 1000,
			autoPay:    false,
			wantErr:    order.ErrInvalidQuantity,
		},
		{
			name:       "quantity zero",
			id:         "item-5",
			accountID:  "acc-1",
			planCode:   "24ska01",
			datacenter: "gra",
			quantity:   0,
			intervalMs: 1000,
			autoPay:    false,
			wantErr:    order.ErrInvalidQuantity,
		},
		{
			name:       "interval too low",
			id:         "item-6",
			accountID:  "acc-1",
			planCode:   "24ska01",
			datacenter: "gra",
			quantity:   1,
			intervalMs: 200,
			autoPay:    false,
			wantErr:    order.ErrInvalidInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := order.NewQueueItem(tt.id, tt.accountID, tt.planCode, tt.datacenter, tt.quantity, tt.intervalMs, tt.autoPay, now)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if item.Datacenter != "GRA" {
				t.Errorf("expected normalized datacenter GRA, got %s", item.Datacenter)
			}
			if item.Status != order.StatusQueued {
				t.Errorf("expected initial status queued, got %s", item.Status)
			}
		})
	}
}

func TestQueueItem_StateTransitions(t *testing.T) {
	now := time.Now()
	item, err := order.NewQueueItem("1", "acc", "24ska01", "rbx", 1, 1000, true, now)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Legal transition: queued -> processing
	if err := item.TransitionTo(order.StatusProcessing, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Status != order.StatusProcessing {
		t.Errorf("expected processing, got %s", item.Status)
	}

	// Illegal transition: processing -> queued directly
	if err := item.TransitionTo(order.StatusQueued, now); err == nil {
		t.Fatalf("expected error transitioning processing -> queued, got nil")
	}

	// Record attempt
	item.RecordAttempt("stock unavailable", now)
	if item.Attempts != 1 || item.LastError != "stock unavailable" {
		t.Errorf("record attempt failed: attempts=%d, err=%s", item.Attempts, item.LastError)
	}

	// Record success with order ID
	if err := item.RecordSuccess("OVH-998811", now); err != nil {
		t.Fatalf("failed to record success: %v", err)
	}
	if item.OrderId != "OVH-998811" || item.Status != order.StatusSuccess {
		t.Errorf("expected success with orderId, got status=%s, orderId=%s", item.Status, item.OrderId)
	}
	if !item.IsTerminal() {
		t.Errorf("expected item to be terminal")
	}

	// Terminal item cannot transition or update interval
	if err := item.TransitionTo(order.StatusProcessing, now); err == nil {
		t.Fatalf("expected error transitioning from terminal state, got nil")
	}
	if err := item.SetInterval(2000, now); err == nil {
		t.Fatalf("expected error setting interval on terminal item, got nil")
	}
}
