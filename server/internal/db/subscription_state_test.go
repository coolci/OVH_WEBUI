package db_test

import (
	"testing"

	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/types"
)

// TestEditSubscriptionKeepsState verifies PRD D-02 / D-04:
// When a user edits a subscription (e.g. changing datacenters or options with empty last_status),
// the database UPSERT must NOT wipe the existing last_status and history.
func TestEditSubscriptionKeepsState(t *testing.T) {
	tmpDir := t.TempDir()
	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	// 1. Initial subscription with active stock state
	initial := types.Subscription{
		PlanCode:        "24sk10",
		Datacenters:     []string{"gra", "rbx"},
		NotifyAvailable: true,
		LastStatus: map[string]string{
			"gra": "available",
			"rbx": "unavailable",
		},
		History: []types.SubscriptionHistoryEntry{
			{Datacenter: "gra", Status: "available", Timestamp: "2026-10-01T00:00:00Z"},
		},
		CreatedAt: "2026-10-01T00:00:00Z",
	}

	if err := database.UpsertMonitorSubscription(initial); err != nil {
		t.Fatalf("upsert initial: %v", err)
	}

	// 2. User edits subscription from UI: changes datacenters, but has no last_status or empty map
	edit := types.Subscription{
		PlanCode:        "24sk10",
		Datacenters:     []string{"gra", "rbx", "sbg"},
		NotifyAvailable: false,
		LastStatus:      nil, // UI edit does not have runtime stock state
		History:         nil,
		CreatedAt:       "2026-10-01T00:00:00Z",
	}

	if err := database.UpsertMonitorSubscription(edit); err != nil {
		t.Fatalf("upsert edit: %v", err)
	}

	// 3. Verify that last_status and history are preserved intact!
	subs, err := database.ListMonitorSubscriptions()
	if err != nil {
		t.Fatalf("list monitor subs: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(subs))
	}
	res := subs[0]
	if len(res.Datacenters) != 3 {
		t.Errorf("expected 3 datacenters, got %v", res.Datacenters)
	}
	if res.NotifyAvailable != false {
		t.Errorf("expected notifyAvailable=false, got %v", res.NotifyAvailable)
	}
	if res.LastStatus["gra"] != "available" || res.LastStatus["rbx"] != "unavailable" {
		t.Errorf("PRD D-02 violation: last_status was wiped! got: %v", res.LastStatus)
	}
	if len(res.History) != 1 || res.History[0].Datacenter != "gra" {
		t.Errorf("PRD D-02 violation: history was wiped! got: %v", res.History)
	}
}
