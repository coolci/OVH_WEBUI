package catalog

import (
	"errors"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

func TestClassifyPrereleaseKeepsRegionAndProductGuards(t *testing.T) {
	resetCatalogCaches()
	resetAvailProbeCaches()
	originalFetch, originalProbe := fetchSubsidiaryCatalog, probeRegionHasPlan
	t.Cleanup(func() {
		fetchSubsidiaryCatalog, probeRegionHasPlan = originalFetch, originalProbe
		resetCatalogCaches()
		resetAvailProbeCaches()
	})
	fetchSubsidiaryCatalog = func(*app.State, string) (*subsidiaryCatalog, error) {
		return &subsidiaryCatalog{plans: map[string]planConfig{"24sk202": {}}, fetchedAt: time.Now()}, nil
	}
	probeRegionHasPlan = func(region, plan string) (bool, error) {
		if plan == "27skle03-v1-us" {
			return region == "US", nil
		}
		if plan == "27skle99-v1" {
			return false, nil
		}
		return region == "EU", nil
	}
	st := testState(t)
	st.Accounts = []types.OVHAccount{{ID: "eu", Endpoint: "ovh-eu", Zone: "IE"}}
	for _, tc := range []struct {
		plan string
		want PlanVerdict
	}{
		{"27skle03-v1", PlanVerdictPrerelease},
		{"27sysstorle01-v1", PlanVerdictPrerelease},
		{"27risele01-v1", PlanVerdictPrerelease},
		{"24sk202", PlanVerdictOK},
		{"27skle03-v1-us", PlanVerdictCrossRegion},
		{"27skle99-v1", PlanVerdictNoSuchPlan},
		{"25scale01-v1", PlanVerdictNotEco},
	} {
		got, _ := ClassifyPlan(st, "eu", tc.plan, "test")
		if got != tc.want {
			t.Errorf("%s: verdict %v, want %v", tc.plan, got, tc.want)
		}
	}
}

func TestPreorderRefreshesCatalogAndSurvivesOutage(t *testing.T) {
	resetCatalogCaches()
	original := fetchSubsidiaryCatalog
	t.Cleanup(func() { fetchSubsidiaryCatalog = original; resetCatalogCaches() })
	st := testState(t)
	st.Accounts = []types.OVHAccount{
		{ID: "eu", Endpoint: "ovh-eu", Zone: "IE"},
		{ID: "us", Endpoint: "ovh-us", Zone: "US"},
	}
	const plan = "27skle03-v1"
	regionCache["IE"] = &subsidiaryCatalog{plans: map[string]planConfig{}, fetchedAt: time.Now().Add(-time.Minute)}
	// A normal reader still uses the two-hour cache.
	if _, err := loadSubsidiaryCatalog(st, "IE"); err != nil {
		t.Fatal(err)
	}
	fetches := 0
	fetchSubsidiaryCatalog = func(_ *app.State, sub string) (*subsidiaryCatalog, error) {
		fetches++
		plans := map[string]planConfig{}
		if sub == "IE" {
			plans[plan] = planConfig{}
		}
		return &subsidiaryCatalog{plans: plans, fetchedAt: time.Now()}, nil
	}
	for i := 0; i < 2; i++ {
		if ready, err := PreorderReady(st, "eu", plan); err != nil || !ready {
			t.Fatalf("listed: %v, %v", ready, err)
		}
	}
	if fetches != 1 {
		t.Fatalf("expected one fresh fetch, got %d", fetches)
	}
	if ready, err := PreorderReady(st, "us", plan); err != nil || ready {
		t.Fatalf("wrong account catalog: %v, %v", ready, err)
	}
	regionCache["US"].fetchedAt = time.Now().Add(-time.Minute)
	fetchSubsidiaryCatalog = func(*app.State, string) (*subsidiaryCatalog, error) { return nil, errors.New("HTTP 429") }
	if ready, _ := PreorderReady(st, "us", plan); ready {
		t.Fatal("catalog outage must not permit an order")
	}
}

func TestMatchPreorderEcoOptionNeverChangesHardware(t *testing.T) {
	for _, tc := range []struct {
		wanted, available string
		match             bool
	}{
		{"ram-64g-ddr4-ecc-2666", "ram-64g-ddr4-ecc-2666-27skle03-v1", true},
		{"softraid-2x480ssd", "softraid-2x480ssd-27skle03-v1", true},
		{"softraid-2x480ssd", "softraid-2x480ssd-2x8000sa-27skle03-v1", false},
		{"ram-64g-ddr4-ecc-2666", "ram-64g-ddr4-ecc-2133-27skle03-v1", false},
		{"ram-64g-ddr4-ecc-2666", "ram-32g-ddr4-ecc-2666-27skle03-v1", false},
	} {
		option, _, _ := MatchPreorderEcoOption([]map[string]interface{}{{"planCode": tc.available}}, tc.wanted)
		if (option != nil) != tc.match {
			t.Errorf("%s -> %s: match=%v", tc.wanted, tc.available, option != nil)
		}
	}
}
