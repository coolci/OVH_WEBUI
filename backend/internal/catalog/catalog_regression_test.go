package catalog

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

func TestRegression_BUG_OVH_05_MatchEcoOptionRejectsDowngradePrefixAndMediaCollision(t *testing.T) {
	// 1. Reverse-prefix silent hardware downgrade MUST be rejected
	opts1 := []map[string]interface{}{{"planCode": "softraid-2x960nvme-24rise01"}}
	if matched, got1, _ := MatchEcoOption(opts1, "softraid-2x960nvme-2x6000sa-24rise01"); matched != nil || got1 != "" {
		t.Fatalf("expected reverse-prefix downgrade to be rejected, got %q", got1)
	}

	// 2. Unanchored standardized prefix collision (480 vs 4800) MUST be rejected
	opts2 := []map[string]interface{}{{"planCode": "softraid-2x4800nvme-24rise01"}}
	if matched, got2, _ := MatchEcoOption(opts2, "softraid-2x480-24rise01"); matched != nil || got2 != "" {
		t.Fatalf("expected 480 vs 4800 prefix collision to be rejected, got %q", got2)
	}

	// 3. Storage media mismatch (NVMe vs SSD / HDD / SAS) MUST be rejected
	opts3 := []map[string]interface{}{{"planCode": "softraid-2x960-ssd"}}
	if matched, got3, _ := MatchEcoOption(opts3, "softraid-2x960-nvme"); matched != nil || got3 != "" {
		t.Fatalf("expected NVMe vs SSD media mismatch to be rejected, got %q", got3)
	}
}

func TestRegression_BUG_OVH_06_CheckServerAvailabilityExtractsSystemStorageAndGPU(t *testing.T) {
	resetCatalogCaches()
	origFetch := fetchSubsidiaryCatalog
	defer func() { fetchSubsidiaryCatalog = origFetch; resetCatalogCaches() }()

	fetchSubsidiaryCatalog = func(_ *app.State, _ string) (*subsidiaryCatalog, error) {
		return &subsidiaryCatalog{
			plans: map[string]planConfig{
				"24advstor01-v3": {
					addonFamilies: map[string][]string{
						"memory":         {"ram-64g-ecc-2933-24advstor01-v3"},
						"storage":        {"softraid-4x16000sa-24advstor01-v3"},
						"system-storage": {"softraid-2x480ssd-24advstor01-v3"},
						"gpu":            {"gpu-rtx4000-ada-24advstor01-v3"},
					},
				},
			},
			fetchedAt: time.Now(),
		}, nil
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		if strings.Contains(r.URL.Path, "/dedicated/server/datacenter/availabilities") {
			fmt.Fprint(w, `[{
				"fqn": "24advstor01-v3.ram-64g-ecc-2933.softraid-4x16000sa.softraid-2x480ssd.gpu-rtx4000-ada",
				"memory": "ram-64g-ecc-2933",
				"storage": "softraid-4x16000sa",
				"systemStorage": "softraid-2x480ssd",
				"gpu": "gpu-rtx4000-ada",
				"datacenters": [{"datacenter": "gra", "availability": "1H-low"}]
			}]`)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()
	origEP := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = origEP }()

	restoreTransport := netfp.SwapSharedTransportForTesting(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.String(), srv.URL) {
			return http.DefaultTransport.RoundTrip(r)
		}
		return nil, fmt.Errorf("blocked external request in test: %s", r.URL.String())
	}))
	defer restoreTransport()

	st := testState(t)
	acc := types.OVHAccount{ID: "eu", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}
	st.Accounts = []types.OVHAccount{acc}
	st.OVH = ovh.NewFactory(nil, st.FindAccount)

	cfgs := CheckServerAvailabilityWithConfigs(st, "24advstor01-v3", "eu")
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(cfgs))
	}
	var gotOpts []string
	for _, c := range cfgs {
		gotOpts = c.Options
		if c.SystemStorage != "softraid-2x480ssd" || c.GPU != "gpu-rtx4000-ada" {
			t.Fatalf("expected SystemStorage and GPU fields populated, got sys=%q gpu=%q", c.SystemStorage, c.GPU)
		}
	}
	wantSet := map[string]bool{
		"ram-64g-ecc-2933-24advstor01-v3":   false,
		"softraid-4x16000sa-24advstor01-v3": false,
		"softraid-2x480ssd-24advstor01-v3":  false,
		"gpu-rtx4000-ada-24advstor01-v3":    false,
	}
	for _, o := range gotOpts {
		wantSet[o] = true
	}
	for want, seen := range wantSet {
		if !seen {
			t.Fatalf("missing mandatory hardware option %q in extracted Options=%v", want, gotOpts)
		}
	}
}

func TestRegression_BUG_OVH_14_IsEcoPlanCodeStandardizeAndDeduplicateDCs(t *testing.T) {
	if !IsEcoPlanCode("24adv01-v1") {
		t.Fatal("expected IsEcoPlanCode('24adv01-v1') == true")
	}
	if !IsEcoPlanCode("ks-le-b") {
		t.Fatal("expected IsEcoPlanCode('ks-le-b') == true")
	}
	if got := StandardizeConfig("softraid-2x960nvme-24adv01"); got != "softraid-2x960nvme" {
		t.Fatalf("expected StandardizeConfig('softraid-2x960nvme-24adv01') == 'softraid-2x960nvme', got %q", got)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case strings.Contains(r.URL.Path, "/order/catalog/public/eco"):
			fmt.Fprint(w, `{
				"plans": [{"planCode": "24ska01", "invoiceName": "KS-A", "pricings": [{"price": 999, "interval": 1, "capacities": ["renew"]}]}],
				"products": []
			}`)
		case strings.Contains(r.URL.Path, "/dedicated/server/datacenter/availabilities"):
			fmt.Fprint(w, `[
				{"fqn":"24ska01.ram-32g.ssd-1","datacenters":[{"datacenter":"gra","availability":"1H-low"},{"datacenter":"rbx","availability":"unavailable"}]},
				{"fqn":"24ska01.ram-64g.ssd-2","datacenters":[{"datacenter":"gra","availability":"unavailable"},{"datacenter":"rbx","availability":"1H-low"}]}
			]`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	origEP := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = origEP }()

	restoreTransport := netfp.SwapSharedTransportForTesting(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.String(), srv.URL) {
			return http.DefaultTransport.RoundTrip(r)
		}
		return nil, fmt.Errorf("blocked external request in test: %s", r.URL.String())
	}))
	defer restoreTransport()

	st := testState(t)
	acc := types.OVHAccount{ID: "eu", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}
	st.Accounts = []types.OVHAccount{acc}
	st.OVH = ovh.NewFactory(nil, st.FindAccount)

	plans, _ := LoadServerList(st, "eu")
	if len(plans) != 1 || len(plans[0].Datacenters) != 2 {
		t.Fatalf("expected 2 deduplicated DC entries across 2 FQNs, got plans=%d dcs=%+v", len(plans), plans)
	}
	for _, d := range plans[0].Datacenters {
		if d.Availability != "1H-low" {
			t.Fatalf("expected merged best availability '1H-low' for %s, got %q", d.Datacenter, d.Availability)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
