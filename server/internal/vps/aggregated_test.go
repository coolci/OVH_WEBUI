package vps

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

func TestPrefetchAggregatedGroupsAndFilters(t *testing.T) {
	state, _ := newVPSOrderState(t)
	var calls atomic.Int32
	var lastQuery atomic.Value
	orig := availabilityHTTPClient
	availabilityHTTPClient = func(_ *app.State, _ string, timeout time.Duration) (*http.Client, error) {
		return &http.Client{Timeout: timeout, Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			if !strings.HasSuffix(r.URL.Path, "/vps/order/rule/datacenter/aggregated") {
				return nil, fmt.Errorf("unexpected request %s", r.URL)
			}
			calls.Add(1)
			lastQuery.Store(r.URL.Query().Get("planCodes"))
			body := `{"models":[
			 {"planCode":"vps-a","datacenters":[{"datacenter":"GRA","code":"eu-west-gra","remoteStorage":false,"status":"available"},
			                                    {"datacenter":"GRA","code":"eu-west-gra","remoteStorage":true,"status":"out-of-stock"}]},
			 {"planCode":"vps-b","datacenters":[{"datacenter":"SBG","code":"eu-west-sbg","status":"out-of-stock"}]}]}`
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})}, nil
	}
	t.Cleanup(func() { availabilityHTTPClient = orig })

	subs := []types.VPSSubscription{
		{ID: "1", PlanCode: "vps-a", OvhSubsidiary: "IE"},
		{ID: "2", PlanCode: "vps-b", OvhSubsidiary: "ie"},
		{ID: "3", PlanCode: "vps-a", OvhSubsidiary: "IE"},                  // 同型号只查一次
		{ID: "4", PlanCode: "vps-c", OvhSubsidiary: "IE", OS: "Debian 12"}, // 带系统过滤:不走聚合
	}
	got := prefetchAggregated(state, subs)
	if calls.Load() != 1 {
		t.Fatalf("one subsidiary/account group must be one request, got %d", calls.Load())
	}
	if q := lastQuery.Load().(string); q != "vps-a,vps-b" {
		t.Fatalf("planCodes must be the unique models without OS filter: %q", q)
	}
	a := got[aggregatedKey{"IE", "", "vps-a"}]
	dcs, _ := a["datacenters"].([]interface{})
	if len(dcs) != 1 {
		t.Fatalf("remote-storage entries must be dropped (per-model API never returns them): %v", dcs)
	}
	if _, ok := got[aggregatedKey{"IE", "", "vps-c"}]; ok {
		t.Fatal("an OS-filtered subscription must not use the aggregated result")
	}
}

func TestPrefetchAggregatedFailureFallsBack(t *testing.T) {
	state, _ := newVPSOrderState(t)
	orig := availabilityHTTPClient
	availabilityHTTPClient = func(_ *app.State, _ string, timeout time.Duration) (*http.Client, error) {
		return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"message":"down"}`)), Request: r}, nil
		})}, nil
	}
	t.Cleanup(func() { availabilityHTTPClient = orig })
	if got := prefetchAggregated(state, []types.VPSSubscription{{ID: "1", PlanCode: "vps-a", OvhSubsidiary: "IE"}}); len(got) != 0 {
		t.Fatalf("a failed batch must leave no prefetched data so the loop queries per model: %v", got)
	}
}
