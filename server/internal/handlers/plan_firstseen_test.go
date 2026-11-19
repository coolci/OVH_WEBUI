package handlers

import (
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// 第一次记录时已在售的型号记 0(不算新品);之后新冒出来的型号记当前时间;
// 另一个目录视角(别的子公司账户)第一次出现时也只建基线,不把它的老型号当成新品。
func TestPlanFirstSeenBaselineThenNew(t *testing.T) {
	st := newTestState(t)
	plans := func(codes ...string) []types.ServerPlan {
		out := make([]types.ServerPlan, 0, len(codes))
		for _, c := range codes {
			out = append(out, types.ServerPlan{PlanCode: c})
		}
		return out
	}

	got := stampPlanFirstSeen(st, "default", plans("24ska01", "25skle01"))
	if got["24ska01"] != 0 || got["25skle01"] != 0 {
		t.Fatalf("基线型号应记 0: %v", got)
	}
	got = stampPlanFirstSeen(st, "default", plans("24ska01", "25skle01", "26sk50b-v1"))
	if got["26sk50b-v1"] <= 0 || got["24ska01"] != 0 {
		t.Fatalf("基线之后新出现的型号应记当前时间: %v", got)
	}
	first := got["26sk50b-v1"]
	got = stampPlanFirstSeen(st, "default", plans("26sk50b-v1"))
	if got["26sk50b-v1"] != first {
		t.Fatalf("首次上架时间不该被刷新: %d → %d", first, got["26sk50b-v1"])
	}
	got = stampPlanFirstSeen(st, "acc-us", plans("24ska01-us", "26sk50b-v1"))
	if got["24ska01-us"] != 0 || got["26sk50b-v1"] != first {
		t.Fatalf("新视角只建基线、已知型号沿用原时间: %v", got)
	}
}
