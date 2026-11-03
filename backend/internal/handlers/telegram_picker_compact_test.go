package handlers

import (
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

func TestGetTopPickModelsAndFuzzyMatching(t *testing.T) {
	st := &app.State{
		ServerPlans: []types.ServerPlan{
			{
				PlanCode: "24ska01",
				Name:     "KS-1 | Atom 4C",
				Datacenters: []types.Datacenter{
					{Datacenter: "gra", Availability: "available"},
					{Datacenter: "rbx", Availability: "unavailable"},
				},
			},
			{
				PlanCode: "ks-5",
				Name:     "KS-5 | Xeon 4C 32G",
				Datacenters: []types.Datacenter{
					{Datacenter: "gra", Availability: "unavailable"},
				},
			},
			{
				PlanCode: "24sys01",
				Name:     "SYS-1 | Xeon 6C",
				Datacenters: []types.Datacenter{
					{Datacenter: "bhs", Availability: "unavailable"},
				},
			},
			{
				PlanCode: "24rise01",
				Name:     "Rise-1 | Ryzen 6C",
				Datacenters: []types.Datacenter{
					{Datacenter: "fra", Availability: "unavailable"},
				},
			},
		},
		Queue: []types.QueueItem{
			{PlanCode: "24sys01", Status: "running"},
		},
	}

	// 1. 测试精选机型排序 (Screen 1 数据源)
	picks := getTopPickModels(st, nil)
	if len(picks) == 0 {
		t.Fatalf("expected non-empty picks")
	}
	if len(picks) > 6 {
		t.Fatalf("picks should not exceed 6, got %d", len(picks))
	}

	// 第一个应当是有现货的 24ska01 (Badge: 🟢)
	if picks[0].Code != "24ska01" || picks[0].Badge != "🟢" {
		t.Errorf("expected 24ska01 with green badge as first pick, got code=%s, badge=%s", picks[0].Code, picks[0].Badge)
	}

	// 排队中的 24sys01 应当被标记为 📋
	foundSys := false
	for _, p := range picks {
		if p.Code == "24sys01" && p.Badge == "📋" {
			foundSys = true
			break
		}
	}
	if !foundSys {
		t.Errorf("expected 24sys01 to have 📋 badge")
	}

	// 2. 测试模糊简写识别 (Natural language / Shortcode matching)
	canonical, _ := findMatchingPlans(st, "ks5")
	if canonical != "ks-5" {
		t.Errorf("expected 'ks5' to match 'ks-5', got %s", canonical)
	}

	canonical, _ = findMatchingPlans(st, "24ska01")
	if canonical != "24ska01" {
		t.Errorf("expected '24ska01' to match '24ska01', got %s", canonical)
	}

	canonical, _ = findMatchingPlans(st, "sys1")
	if canonical != "24sys01" {
		t.Errorf("expected 'sys1' to match '24sys01' via name/code, got %s", canonical)
	}

	canonical, _ = findMatchingPlans(st, "rise1")
	if canonical != "24rise01" {
		t.Errorf("expected 'rise1' to match '24rise01' via name/code, got %s", canonical)
	}
}

func TestAutoPreselectInStockDCs(t *testing.T) {
	st, _ := newWatchTestMonitor(t)
	st.ServerPlansMu.Lock()
	st.ServerPlans = []types.ServerPlan{
		{
			PlanCode: "24ska01",
			Name:     "KS-1",
			Datacenters: []types.Datacenter{
				{Datacenter: "gra", Availability: "available"},
				{Datacenter: "rbx", Availability: "12H"},
				{Datacenter: "sbg", Availability: "unavailable"},
			},
		},
	}
	st.ServerPlansMu.Unlock()

	chatID := int64(999999)
	msgID := int64(888888)
	sess := getPickerSession(chatID, msgID, "b", "24ska01")
	if len(sess.Selected) != 0 {
		t.Fatalf("initial session should be empty")
	}

	// 模拟第一次进入 showDCPicker 的自动预选逻辑
	stock := inStockDisplayDCs(st, "24ska01", "")
	if len(sess.Selected) == 0 && len(stock) > 0 {
		for _, d := range stock {
			sess.Selected[d] = true
		}
	}
	if len(stock) > 0 && !sess.Selected[stock[0]] {
		t.Errorf("expected stock dc %s to be auto selected", stock[0])
	}
}
