package db

import (
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

func TestVPSSubscriptionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	want := types.VPSSubscription{
		ID: "sub-1", PlanCode: "vps-2027-model2", OvhSubsidiary: "IE",
		Datacenters:  []string{"gra", "bhs"},
		MonitorLinux: true, MonitorWindows: false,
		NotifyAvailable: true, NotifyUnavailable: true,
		LastStatus: map[string]string{"gra": "available"},
		History:    []map[string]interface{}{},
		CreatedAt:  types.NowISO(),
		AutoOrderAccountID: "acc-1",
		AutoOrder:          true,
		Quantity:           3,
		AutoPay:            true,
		OS:                 "debian12_64",
	}
	if err := database.ReplaceVPSSubscriptions([]types.VPSSubscription{want}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := database.ListVPSSubscriptions()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 条,实际 %d", len(got))
	}
	g := got[0]
	if !g.AutoOrder {
		t.Error("AutoOrder 没存住 —— VPS 自动下单会在重启后静默失效")
	}
	if g.Quantity != 3 {
		t.Errorf("Quantity 没存住:期望 3,实际 %d", g.Quantity)
	}
	if !g.AutoPay {
		t.Error("AutoPay 没存住 —— 用户开了自动付款,重启后变成不付款")
	}
	if g.OS != "debian12_64" {
		t.Errorf("OS 没存住:期望 debian12_64,实际 %q —— 自动下单会用 OVH 默认系统", g.OS)
	}
	if g.AutoOrderAccountID != "acc-1" || g.PlanCode != want.PlanCode || !g.MonitorLinux {
		t.Errorf("其它字段也不对: %+v", g)
	}
}
