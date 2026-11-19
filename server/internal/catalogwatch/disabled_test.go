package catalogwatch

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

func TestRetiredCatalogWatchDefaultsOff(t *testing.T) {
	_, s := fixture(t)
	if New(s).data.Enabled || For(s).data.Enabled {
		t.Fatal("fresh installation enabled retired automatic ordering")
	}
}

func TestRetiredCatalogWatchStopsLegacyJobsAndOnlyReconciles(t *testing.T) {
	legacy, s := fixture(t)
	ready(t, legacy)
	pendingID := sole(t, legacy).ID
	legacy.data.Jobs["submitted"] = &Job{
		ID: "submitted", AccountID: "a", Endpoint: "ovh-eu",
		Status: "uncertain", PerDatacenter: true,
		Checkout: &purchase.CheckoutJournal{CartID: "offline-cart", Submitted: true},
	}
	if err := legacy.save(); err != nil {
		t.Fatal(err)
	}
	w := For(s)
	if w != For(s) || w.data.Enabled || w.data.Jobs[pendingID].Status != "cancelled" || w.data.Jobs["submitted"].Status != "uncertain" {
		t.Fatalf("unsafe legacy state: %+v", w.data)
	}
	var raw string
	if err := s.DB.Get(&raw, "SELECT data FROM catalog_watch WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	var stored Data
	if err := json.Unmarshal([]byte(raw), &stored); err != nil || stored.Enabled || len(stored.Jobs) != 2 {
		t.Fatalf("disabled state or original jobs lost: %s (%v)", raw, err)
	}
	var fetches, buys atomic.Int32
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) {
		fetches.Add(1)
		return snapshot("new-model"), nil
	}
	w.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
		buys.Add(1)
		return purchase.Outcome{}
	}
	checked := make(chan struct{}, 1)
	w.check = func(_ context.Context, _ *app.State, account string, journal *purchase.CheckoutJournal) (purchase.CheckoutCheck, error) {
		if account != "a" || journal == nil || journal.CartID != "offline-cart" {
			t.Error("reconciliation lost original checkout identity")
		}
		checked <- struct{}{}
		return purchase.CheckoutCheck{NotSubmitted: true}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Run(ctx)
	select {
	case <-checked:
	case <-time.After(5 * time.Second):
		t.Error("submitted checkout was no longer reconciled")
	}
	cancel()
	if !w.Wait(5 * time.Second) {
		t.Fatal("read-only worker did not stop")
	}
	if fetches.Load() != 0 || buys.Load() != 0 || w.data.Jobs["submitted"].Status != "cancelled" {
		t.Fatalf("retired worker resumed ordering: fetch=%d buy=%d status=%s", fetches.Load(), buys.Load(), w.data.Jobs["submitted"].Status)
	}
	if For(s).data.Enabled || New(s).data.Enabled {
		t.Fatal("restart restored retired ordering")
	}
}
