package db

import (
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

func TestQueueForceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	in := []types.QueueItem{{
		ID:         "q1",
		AccountID:  "acc-1",
		PlanCode:   "custom-sku",
		Datacenter: "gra",
		Options:    []string{},
		Status:     "running",
		CreatedAt:  types.NowISO(),
		UpdatedAt:  types.NowISO(),
		Force:      true,
	}}
	if err := d.ReplaceQueue(in); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := d.ListQueue()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 条,实际 %d", len(got))
	}
	if !got[0].Force {
		t.Fatal("Force 没存下来")
	}
}
