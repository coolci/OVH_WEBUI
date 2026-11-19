package db

import (
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// 加了列忘了同步行结构/INSERT/upsert 的任何一处,都会让写入被静默丢弃 ——
// 界面上开关还亮着,重启就没了。VPS 那边正是这么踩过一次。
func TestMonitorSubOptionsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	in := []types.Subscription{{
		PlanCode:           "24sk602",
		Datacenters:        []string{"gra", "rbx"},
		NotifyAvailable:    true,
		Options:            []string{"ram-64g-noecc-2133", "softraid-2x480ssd"},
		AutoOrder:          true,
		Quantity:           2,
		AutoOrderAccountID: "acc-1",
		CreatedAt:          types.NowISO(),
	}}
	if err := d.ReplaceMonitorSubscriptions(in); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := d.ListMonitorSubscriptions()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 条,实际 %d", len(got))
	}
	if len(got[0].Options) != 2 || got[0].Options[0] != "ram-64g-noecc-2133" {
		t.Fatalf("Options 没存下来: %+v", got[0].Options)
	}
	if got[0].Quantity != 2 || !got[0].AutoOrder || got[0].AutoOrderAccountID != "acc-1" {
		t.Fatalf("其余自动下单字段也要一起活着: %+v", got[0])
	}

	// 空 options = 盯全部配置,读回来不能变 nil 之外的怪东西
	in[0].Options = nil
	if err := d.ReplaceMonitorSubscriptions(in); err != nil {
		t.Fatalf("replace2: %v", err)
	}
	got, _ = d.ListMonitorSubscriptions()
	if len(got[0].Options) != 0 {
		t.Fatalf("空 options 应读回空,实际 %+v", got[0].Options)
	}
}
