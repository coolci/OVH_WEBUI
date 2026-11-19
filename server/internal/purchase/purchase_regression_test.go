package purchase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

func TestRegression_BUG_OVH_02_OrderDetailAliasNormalizationPreventsDoublePurchase(t *testing.T) {
	// 1. Direct unit verification across all alias & AZ pairs
	nonContradictingCases := []struct {
		targetDC string
		desc     string
	}{
		{"mum", "Eco Server 24ska01 - Datacenter: ynm"},
		{"yyz", "Dedicated Server 24ska01 - Datacenter: tor"},
		{"ca-east-tor-a", "Dedicated Server 24ska01 - Datacenter: tor"},
		{"eu-west-par-a", "Dedicated Server 24ska01 - Datacenter: par"},
		{"par-a", "Dedicated Server 24ska01 - Datacenter: eu-west-par-a"},
		{"us-east-vin-a", "Dedicated Server 24ska01-us - Datacenter: vin"},
		{"eu-west-lim", "Dedicated Server 24ska01 - Datacenter: lim"},
		{"eu-west-eri", "Dedicated Server 24ska01 - Datacenter: eri"},
		{"ap-southeast-sgp", "Dedicated Server 24ska01 - Datacenter: sgp"},
		{"ap-southeast-syd", "Dedicated Server 24ska01 - Datacenter: syd"},
	}
	for _, tc := range nonContradictingCases {
		detail := map[string]interface{}{"description": tc.desc}
		if orderDetailContradictsDC(detail, tc.targetDC) {
			t.Fatalf("expected orderDetailContradictsDC(%q, %q) == false", tc.desc, tc.targetDC)
		}
	}

	// Distinct AZs or distinct cities must still contradict
	if !orderDetailContradictsDC(map[string]interface{}{"description": "Datacenter: eu-west-par-b"}, "eu-west-par-a") {
		t.Fatal("expected eu-west-par-b to contradict eu-west-par-a")
	}

	// 2. End-to-end PurchaseServer proof: existing Order #998877 in 'ynm' for task targeting 'mum'
	// must be reconciled without submitting a second checkout!
	var checkoutCalls atomic.Int32
	nowOrderDate := time.Now().UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/me/order" && r.Method == http.MethodGet:
			fmt.Fprint(w, `[998877]`)
		case r.URL.Path == "/me/order/998877" && r.Method == http.MethodGet:
			fmt.Fprintf(w, `{"orderId":998877,"date":%q,"url":"https://www.ovh.com/order/998877"}`, nowOrderDate)
		case r.URL.Path == "/me/order/998877/details" && r.Method == http.MethodGet:
			fmt.Fprint(w, `[1]`)
		case r.URL.Path == "/me/order/998877/details/1" && r.Method == http.MethodGet:
			fmt.Fprint(w, `{"orderDetailId":1,"description":"KS-A (24ska01) - Datacenter: ynm","domain":"ns1.ynm"}`)
		case strings.HasSuffix(r.URL.Path, "/checkout") && r.Method == http.MethodPost:
			n := checkoutCalls.Add(1)
			fmt.Fprintf(w, `{"orderId":%d,"url":"https://www.ovh.com/order/%d"}`, 998877+n, 998877+n)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	swapEndpoint(t, "ovh-eu", srv.URL)

	st := quickOrderTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "acc-mum", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}

	item := &types.QueueItem{
		ID:                "task-mum-1",
		AccountID:         "acc-mum",
		PlanCode:          "24ska01",
		Datacenter:        "mum",
		Status:            "running",
		UncertainCheckout: true,
		UncertainTime:     types.NowISO(),
	}
	out := PurchaseServer(context.Background(), st, item)
	if !out.Success || checkoutCalls.Load() != 0 {
		t.Fatalf("expected existing order #998877 to be reconciled with 0 new checkouts, got out=%+v checkoutCalls=%d", out, checkoutCalls.Load())
	}
	hist := st.HistorySnapshot()
	if len(hist) != 1 || hist[0].OrderID != "998877" {
		t.Fatalf("expected history to record reconciled order 998877, got %+v", hist)
	}
}

func TestRegression_BUG_OVH_11_TrackCartReleasedOnCheckoutSuccess(t *testing.T) {
	const cartID = "success-released-cart-777"
	defer ovh.ReleaseCart(cartID)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case strings.Contains(r.URL.Path, "/dedicated/server/datacenter/availabilities"):
			fmt.Fprint(w, `[{"fqn":"24ska01","datacenters":[{"datacenter":"gra","availability":"1H-low"}]}]`)
		case r.URL.Path == "/order/cart" && r.Method == http.MethodPost:
			fmt.Fprintf(w, `{"cartId":%q}`, cartID)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodPost:
			fmt.Fprint(w, `{"itemId":1}`)
		case strings.HasSuffix(r.URL.Path, "/requiredConfiguration"):
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/configuration"):
			fmt.Fprint(w, `{"id":1}`)
		case strings.HasSuffix(r.URL.Path, "/checkout") && r.Method == http.MethodPost:
			fmt.Fprint(w, `{"orderId":555001,"url":"https://www.ovh.com/order/555001"}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	swapEndpoint(t, "ovh-eu", srv.URL)

	restore := netfp.SwapSharedTransportForTesting(auditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.String(), srv.URL) {
			return http.DefaultTransport.RoundTrip(r)
		}
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24ska01","configurations":[{"name":"region","values":["europe"]}]}]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("blocked external request: %s", r.URL.String())
	}))
	defer restore()

	st := quickOrderTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "acc-leak", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}
	item := &types.QueueItem{ID: "task-leak", AccountID: "acc-leak", PlanCode: "24ska01", Datacenter: "gra", Status: "running"}

	out := PurchaseServer(context.Background(), st, item)
	if !out.Success {
		t.Fatalf("purchase failed: %+v", out)
	}
	if ovh.CartInUse(cartID) {
		t.Fatalf("expected ovh.CartInUse(%q) == false after checkout success", cartID)
	}
}

func TestRegression_BUG_OVH_12_PrewarmSingleflightAndDatacenterNormalization(t *testing.T) {
	st := quickOrderTestState(t)
	ClearPrewarmPool(st)
	defer ClearPrewarmPool(st)
	resetEcoOptionsCache()

	st.Accounts = []types.OVHAccount{{
		ID: "acc-sf", Name: "EU", Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true,
	}}

	var cartCreateCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/order/cart" && r.Method == http.MethodPost:
			n := cartCreateCalls.Add(1)
			time.Sleep(40 * time.Millisecond)
			fmt.Fprintf(w, `{"cartId":"sf-cart-%d"}`, n)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodPost:
			fmt.Fprint(w, `{"itemId":10}`)
		case strings.Contains(r.URL.Path, "/configuration"):
			fmt.Fprint(w, `{"id":1}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	swapEndpoint(t, "ovh-eu", srv.URL)

	restore := netfp.SwapSharedTransportForTesting(auditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.String(), srv.URL) {
			return http.DefaultTransport.RoundTrip(r)
		}
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24ska01","configurations":[{"name":"region","values":["canada"]}]}]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("blocked external request: %s", r.URL.String())
	}))
	defer restore()

	// Mix display code "mum" and API code "ynm" across 6 concurrent AssemblePrewarmedCart calls!
	dcs := []string{"mum", "ynm", "MUM", "ynm", "mum", "YNM"}
	var wg sync.WaitGroup
	errs := make([]error, len(dcs))
	for i, dc := range dcs {
		wg.Add(1)
		go func(idx int, dcCode string) {
			defer wg.Done()
			_, errs[idx] = AssemblePrewarmedCart(context.Background(), st, "acc-sf", "24ska01", dcCode, nil, "", "P1M")
		}(i, dc)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}
	if cartCreateCalls.Load() != 1 {
		t.Fatalf("expected 6 concurrent prewarm calls (mum/ynm) to create 1 OVH cart via singleflight, created %d", cartCreateCalls.Load())
	}
	if !HasPrewarmedCart("acc-sf", "24ska01", "mum", nil, "", "P1M") || !HasPrewarmedCart("acc-sf", "24ska01", "ynm", nil, "", "P1M") {
		t.Fatal("expected normalized prewarm pool lookup to match both 'mum' and 'ynm'")
	}
}
