package catalog

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// 移植自上游 e7ded72(issue #12):目录未命中的型号 30 秒后重新确认,已命中的保留原缓存;
// 并发刷新合并成一次请求。
func TestMonitorCatalogMissRefreshesSoonWithoutRefreshingKnownPlans(t *testing.T) {
	resetCatalogCaches()
	original := fetchSubsidiaryCatalog
	t.Cleanup(func() { fetchSubsidiaryCatalog = original; resetCatalogCaches() })
	const plan = "27sysle02-v1"
	st := testState(t)
	st.Accounts = []types.OVHAccount{{ID: "issue12", Endpoint: "ovh-eu", Zone: "IE"}}
	var calls atomic.Int32
	fetchSubsidiaryCatalog = func(_ *app.State, subsidiary string) (*subsidiaryCatalog, error) {
		calls.Add(1)
		if subsidiary != "IE" {
			return nil, errors.New("wrong subsidiary")
		}
		return &subsidiaryCatalog{plans: map[string]planConfig{
			plan: {addonFamilies: map[string][]string{"memory": {"ram-128g"}}},
		}, fetchedAt: time.Now()}, nil
	}
	for _, tc := range []struct {
		name    string
		present bool
		age     time.Duration
		fetches int32
	}{
		{"recent miss is cached", false, time.Second, 0},
		{"known plan keeps normal cache", true, time.Minute, 0},
		{"missing plan is refreshed after 30 seconds", false, time.Minute, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetCatalogCaches()
			calls.Store(0)
			plans := map[string]planConfig{}
			if tc.present {
				plans[plan] = planConfig{addonFamilies: map[string][]string{"memory": {"ram-128g"}}}
			}
			regionCache["IE"] = &subsidiaryCatalog{plans: plans, fetchedAt: time.Now().Add(-tc.age)}
			var wg sync.WaitGroup
			for i := 0; i < 16; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					families, err := AddonFamiliesForPlan(st, "issue12", plan)
					if tc.present || tc.fetches > 0 {
						if err != nil || len(families["memory"]) != 1 {
							t.Errorf("listed plan was not recovered: families=%v err=%v", families, err)
						}
					} else if !errors.Is(err, ErrPlanNotInCatalog) {
						t.Errorf("recent missing plan must retain its cached result: %v", err)
					}
				}()
			}
			wg.Wait()
			if calls.Load() != tc.fetches {
				t.Fatalf("catalog requests=%d, want %d", calls.Load(), tc.fetches)
			}
		})
	}
}

