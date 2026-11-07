package db

import (
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// 购买时长必须跨重启保留:任务、历史、监控订阅三张表都要存下来。
// 丢了的话年付任务重启后会按月付下单,而这在界面上完全看不出来。
func TestDurationSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := types.NowISO()
	if err := d.ReplaceQueue([]types.QueueItem{
		{ID: "yearly", AccountID: "a", PlanCode: "p", Datacenter: "gra", Options: []string{}, Status: "paused", CreatedAt: now, UpdatedAt: now, Duration: "P12M"},
		{ID: "legacy", AccountID: "a", PlanCode: "p", Datacenter: "gra", Options: []string{}, Status: "paused", CreatedAt: now, UpdatedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.ReplaceHistory([]types.PurchaseHistoryEntry{{ID: "h", AccountID: "a", TaskID: "yearly", PlanCode: "p", Datacenter: "gra", Options: []string{}, Status: "success", PurchaseTime: now, Duration: "P24M"}}); err != nil {
		t.Fatal(err)
	}
	sub := types.Subscription{PlanCode: "p", Datacenters: []string{}, Options: []string{}, CreatedAt: now, AutoOrder: true, Quantity: 1, Duration: "P6M"}
	if err := d.UpsertMonitorSubscription(sub); err != nil {
		t.Fatal(err)
	}
	// 就地更新(ON CONFLICT 分支)也要改到时长
	sub.Duration = "P12M"
	if err := d.UpsertMonitorSubscription(sub); err != nil {
		t.Fatal(err)
	}
	d.Close()

	d, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	queue, err := d.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	for _, q := range queue {
		byID[q.ID] = q.Duration
	}
	if byID["yearly"] != "P12M" || byID["legacy"] != "" {
		t.Fatalf("queue durations after reopen: %v", byID)
	}
	history, err := d.ListHistory()
	if err != nil || len(history) != 1 || history[0].Duration != "P24M" {
		t.Fatalf("history duration lost: %+v err=%v", history, err)
	}
	subs, err := d.ListMonitorSubscriptions()
	if err != nil || len(subs) != 1 || subs[0].Duration != "P12M" {
		t.Fatalf("subscription duration lost: %+v err=%v", subs, err)
	}
	// ReplaceMonitorSubscriptions 路径
	subs[0].Duration = "P3M"
	if err := d.ReplaceMonitorSubscriptions(subs); err != nil {
		t.Fatal(err)
	}
	subs, _ = d.ListMonitorSubscriptions()
	if len(subs) != 1 || subs[0].Duration != "P3M" {
		t.Fatalf("replace lost duration: %+v", subs)
	}
}
