package vps

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

func TestRegression_BUG_OVH_07_VPSMonitorMatchesShortAndRuleDatacenterCodes(t *testing.T) {
	state, _ := newVPSOrderState(t)
	state.VPSCheckInterval = 1

	// Subscription created with catalog vps_datacenter values ("DE", "UK", "YNM") as done by BatchAddAllVPS / GetVPSModels
	sub := types.VPSSubscription{
		ID:              "sub-de-uk",
		PlanCode:        "vps-2025-model1",
		OvhSubsidiary:   "IE",
		Datacenters:     []string{"DE", "UK", "YNM"},
		MonitorLinux:    true,
		NotifyAvailable: true,
		LastStatus:      map[string]string{},
	}
	state.VPSSubscriptions = []types.VPSSubscription{sub}

	var checks atomic.Int32
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/vps/order/rule/datacenter/aggregated") {
			checks.Add(1)
			body := `{"models":[{"planCode":"vps-2025-model1","datacenters":[
				{"datacenter":"DE","code":"eu-west-lim","status":"available","linuxStatus":"available"},
				{"datacenter":"UK","code":"eu-west-eri","status":"available","linuxStatus":"available"},
				{"datacenter":"YNM","code":"ap-south-mum","status":"available","linuxStatus":"available"}
			]}]}`
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
	})
	restore := netfp.SwapSharedTransportForTesting(transport)
	defer restore()
	origClient := availabilityHTTPClient
	availabilityHTTPClient = func(_ *app.State, _ string, timeout time.Duration) (*http.Client, error) {
		return &http.Client{Transport: transport, Timeout: timeout}, nil
	}
	defer func() { availabilityHTTPClient = origClient }()

	notify.AllowLocalWebhookForTesting = true
	defer func() { notify.AllowLocalWebhookForTesting = false }()
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, `{"code":0}`) }))
	defer hook.Close()
	state.Config.Set(types.Config{NotifyWebhookURL: hook.URL})

	Stop(state)
	waitLoopExit()
	if !Start(state) {
		t.Fatal("start failed")
	}
	defer func() { Stop(state); waitLoopExit() }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state.VPSSubsMu.Lock()
		n := len(state.VPSSubscriptions[0].LastStatus)
		state.VPSSubsMu.Unlock()
		if n == 3 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	Stop(state)
	waitLoopExit()

	state.VPSSubsMu.Lock()
	gotStatus := state.VPSSubscriptions[0].LastStatus
	state.VPSSubsMu.Unlock()
	for _, wantCode := range []string{"eu-west-lim", "eu-west-eri", "ap-south-mum"} {
		if gotStatus[wantCode] != "available" {
			t.Fatalf("expected DE/UK/YNM subscription to match %q as 'available', got LastStatus=%v", wantCode, gotStatus)
		}
	}

	if MatchVPSDatacenter("EU-WEST-LZ-AMS", "eu-west-lz-mad", "EU-WEST-LZ-MAD") {
		t.Fatal("distinct Local Zones must never match each other")
	}
	if got := restockOrderDatacenter(map[string]interface{}{"code": "eu-west-lim"}); got != "DE" {
		t.Fatalf("expected restockOrderDatacenter fallback for eu-west-lim to yield 'DE', got %q", got)
	}
}

func TestRegression_BUG_OVH_08_VPSMonitorRevisionDiscardsStaleInFlightRound(t *testing.T) {
	state, _ := newVPSOrderState(t)
	state.VPSCheckInterval = 10

	state.VPSSubscriptions = []types.VPSSubscription{{
		ID:              "sub-race-sub",
		PlanCode:        "vps-2025-model1",
		OvhSubsidiary:   "FR",
		MonitorLinux:    true,
		NotifyAvailable: true,
		LastStatus:      map[string]string{},
	}}
	_ = SaveSubscriptions(state)

	enteredHTTP := make(chan struct{})
	releaseHTTP := make(chan struct{})
	roundDone := make(chan struct{})
	var frChecks atomic.Int32

	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/vps/order/rule/datacenter/aggregated") {
			if strings.Contains(r.URL.RawQuery, "ovhSubsidiary=FR") && frChecks.Add(1) == 1 {
				close(enteredHTTP)
				<-releaseHTTP
				defer func() {
					time.AfterFunc(120*time.Millisecond, func() {
						select {
						case <-roundDone:
						default:
							close(roundDone)
						}
					})
				}()
			}
			body := `{"models":[{"planCode":"vps-2025-model1","datacenters":[{"datacenter":"GRA","code":"eu-west-gra","status":"available","linuxStatus":"available"}]}]}`
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
	})
	restore := netfp.SwapSharedTransportForTesting(transport)
	defer restore()
	origClient := availabilityHTTPClient
	availabilityHTTPClient = func(_ *app.State, _ string, timeout time.Duration) (*http.Client, error) {
		return &http.Client{Transport: transport, Timeout: timeout}, nil
	}
	defer func() { availabilityHTTPClient = origClient }()

	var webhookHits atomic.Int32
	notify.AllowLocalWebhookForTesting = true
	defer func() { notify.AllowLocalWebhookForTesting = false }()
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookHits.Add(1)
		fmt.Fprint(w, `{"code":0}`)
	}))
	defer hook.Close()
	state.Config.Set(types.Config{NotifyWebhookURL: hook.URL})

	Stop(state)
	waitLoopExit()
	if !Start(state) {
		t.Fatal("start failed")
	}
	defer func() { Stop(state); waitLoopExit() }()

	<-enteredHTTP
	revBefore := SubscriptionRevision("sub-race-sub")

	// User edits subsidiary from FR -> US while FR check is in-flight, resetting LastStatus={}
	_, _, err := state.MutateVPSSubscription("sub-race-sub", func(s *types.VPSSubscription) bool {
		s.OvhSubsidiary = "US"
		s.LastStatus = map[string]string{}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}

	close(releaseHTTP)
	<-roundDone
	Stop(state)
	waitLoopExit()

	state.VPSSubsMu.Lock()
	gotSub := state.VPSSubscriptions[0].OvhSubsidiary
	gotStatus := state.VPSSubscriptions[0].LastStatus
	state.VPSSubsMu.Unlock()

	if gotSub != "US" {
		t.Fatalf("expected OvhSubsidiary to remain 'US', got %q", gotSub)
	}
	if len(gotStatus) != 0 {
		t.Fatalf("expected stale FR in-flight round to be discarded so US LastStatus stays empty, got %v", gotStatus)
	}
	if revAfter := SubscriptionRevision("sub-race-sub"); revAfter <= revBefore {
		t.Fatalf("expected SubscriptionRevision to advance after mid-flight edit (%d -> %d)", revBefore, revAfter)
	}
	if webhookHits.Load() != 0 {
		t.Fatalf("expected 0 stale notifications from invalidated FR round, got %d", webhookHits.Load())
	}
}

func TestRegression_BUG_OVH_11_PurchaseVPSReleasesCartTrackingOnSuccess(t *testing.T) {
	state, _ := newVPSOrderState(t)
	sub := addAutoSub(t, state)

	out := PurchaseVPS(state, sub, "GRA")
	if !out.Success || out.OrderID != "555" {
		t.Fatalf("expected PurchaseVPS to succeed with OrderID=555, got %+v", out)
	}
	if ovh.CartInUse("auto-cart") {
		t.Fatal("expected PurchaseVPS to release 'auto-cart' from ovh.inUseCarts upon checkout success")
	}
}
