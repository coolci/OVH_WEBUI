package purchase

import (
	"context"
	"fmt"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func auditOrderFixture(t *testing.T, plan, optionResponse string) (*app.State, *types.QueueItem, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	s := quickOrderTestState(t)
	ClearPrewarmPool(nil)
	resetEcoOptionsCache()
	s.Accounts = []types.OVHAccount{{ID: "audit-account", Endpoint: "ovh-eu", Zone: "FI", AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}
	checkout, options := &atomic.Int32{}, &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/me/order":
			w.WriteHeader(503)
			io.WriteString(w, `{"message":"reconcile unavailable"}`)
		case strings.HasPrefix(r.URL.Path, "/me/order/"):
			w.WriteHeader(404)
			io.WriteString(w, `{"message":"audit does not expose real orders"}`)
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprintf(w, `[{"fqn":%q,"datacenters":[{"datacenter":"gra","availability":"1H"}]}]`, plan+".ram-64g")
		case r.URL.Path == "/order/cart":
			io.WriteString(w, `{"cartId":"audit-cart"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			io.WriteString(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco"):
			io.WriteString(w, `{"itemId":1}`)
		case strings.HasSuffix(r.URL.Path, "/requiredConfiguration"):
			io.WriteString(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/configuration"):
			io.WriteString(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco/options") && r.Method == http.MethodGet:
			if optionResponse == "503" {
				w.WriteHeader(503)
				io.WriteString(w, `{"message":"temporary option outage"}`)
			} else {
				io.WriteString(w, optionResponse)
			}
		case strings.HasSuffix(r.URL.Path, "/eco/options"):
			options.Add(1)
			io.WriteString(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/coupon"):
			w.WriteHeader(400)
			io.WriteString(w, `{"message":"Coupon is not valid"}`)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			checkout.Add(1)
			io.WriteString(w, `{"orderId":12345,"url":"https://audit.invalid/order"}`)
		case r.Method == http.MethodDelete:
			io.WriteString(w, `null`)
		default:
			t.Errorf("unexpected mocked request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	swapEndpoint(t, "ovh-eu", srv.URL)
	base := http.DefaultTransport
	transport := auditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.String(), srv.URL) {
			return base.RoundTrip(r)
		}
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			body := fmt.Sprintf(`{"plans":[{"planCode":%q,"configurations":[{"name":"region","values":["europe"]}]}]}`, plan)
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		}
		return nil, fmt.Errorf("external request blocked by audit: %s", r.URL)
	})
	http.DefaultTransport = transport
	restore := netfp.SwapSharedTransportForTesting(transport)
	t.Cleanup(func() { restore(); http.DefaultTransport = base; ClearPrewarmPool(nil); resetEcoOptionsCache() })
	return s, &types.QueueItem{ID: "audit-task", AccountID: "audit-account", PlanCode: plan, Datacenter: "gra", Status: "running", Force: true}, checkout, options
}

func TestUncertainReconcileFailureBlocksCheckout(t *testing.T) {
	s, item, checkout, _ := auditOrderFixture(t, "audit-uncertain", `[{"planCode":"ram-64g","prices":[]}]`)
	item.UncertainCheckout = true
	item.UncertainTime = types.NowISO()
	out := PurchaseServer(context.Background(), s, item)
	if out.Success || !out.PauseTask || !item.UncertainCheckout || checkout.Load() != 0 {
		t.Fatalf("outcome=%+v checkout=%d uncertain=%v", out, checkout.Load(), item.UncertainCheckout)
	}
}
func TestPrewarmMissingOptionsBlocksCheckout(t *testing.T) {
	for _, response := range []string{"503", `[]`} {
		t.Run(response, func(t *testing.T) {
			s, item, checkout, options := auditOrderFixture(t, "audit-missing-option", response)
			item.Options = []string{"ram-64g"}
			_, err := AssemblePrewarmedCart(context.Background(), s, item.AccountID, item.PlanCode, item.Datacenter, item.Options, "")
			if err == nil || HasPrewarmedCart(item.AccountID, item.PlanCode, item.Datacenter, item.Options, "") {
				t.Fatal("incomplete prewarm published")
			}
			out := PurchaseServer(context.Background(), s, item)
			if out.Success || checkout.Load() != 0 || options.Load() != 0 {
				t.Fatalf("%+v checkout=%d addonPosts=%d", out, checkout.Load(), options.Load())
			}
		})
	}
}
func TestPrewarmRejectedCouponBlocksAutoPay(t *testing.T) {
	s, item, checkout, _ := auditOrderFixture(t, "audit-coupon", `[{"planCode":"ram-64g","prices":[]}]`)
	item.Coupon = "INVALID"
	item.AutoPay = true
	item.Options = []string{"ram-64g"}
	_, err := AssemblePrewarmedCart(context.Background(), s, item.AccountID, item.PlanCode, item.Datacenter, item.Options, item.Coupon)
	if err == nil || HasPrewarmedCart(item.AccountID, item.PlanCode, item.Datacenter, item.Options, item.Coupon) {
		t.Fatal("rejected coupon published")
	}
	out := PurchaseServer(context.Background(), s, item)
	if out.Success || !out.PauseTask || checkout.Load() != 0 {
		t.Fatalf("%+v checkout=%d", out, checkout.Load())
	}
}
func TestCustomDottedPlanWorkerCheckout(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			s, item, checkout, options := auditOrderFixture(t, "custom-NewModel.LZ", `[{"planCode":"ram-64g","prices":[]}]`)
			if explicit {
				item.Options = []string{"ram-64g"}
			}
			out := PurchaseServer(context.Background(), s, item)
			if !out.Success || checkout.Load() != 1 || options.Load() != 1 {
				t.Fatalf("%+v checkout=%d options=%d history=%+v", out, checkout.Load(), options.Load(), s.History)
			}
		})
	}
}
func TestPrewarmCompleteCartReused(t *testing.T) {
	s, item, checkout, options := auditOrderFixture(t, "custom-NewModel.LZ", `[{"planCode":"ram-64g","prices":[]}]`)
	item.Options = []string{"ram-64g"}
	_, err := AssemblePrewarmedCart(context.Background(), s, item.AccountID, item.PlanCode, item.Datacenter, item.Options, "")
	if err != nil {
		t.Fatal(err)
	}
	out := PurchaseServer(context.Background(), s, item)
	if !out.Success || checkout.Load() != 1 || options.Load() != 1 {
		t.Fatalf("%+v checkout=%d options=%d", out, checkout.Load(), options.Load())
	}
	if HasPrewarmedCart(item.AccountID, item.PlanCode, item.Datacenter, item.Options, "") {
		t.Fatal("cart not consumed")
	}
}
func TestPrewarmWithoutOptionsDoesNotOverrideInventoryHardware(t *testing.T) {
	s, item, checkout, options := auditOrderFixture(t, "custom-NewModel.LZ", `[{"planCode":"ram-64g","prices":[]}]`)
	_, err := AssemblePrewarmedCart(context.Background(), s, item.AccountID, item.PlanCode, item.Datacenter, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	out := PurchaseServer(context.Background(), s, item)
	if !out.Success || checkout.Load() != 1 || options.Load() != 1 {
		t.Fatalf("%+v checkout=%d options=%d", out, checkout.Load(), options.Load())
	}
}
func TestReconcileRequiresWholeModelToken(t *testing.T) {
	for _, tc := range []struct {
		description, plan string
		want              bool
	}{
		{"RISE 24sk202 - gra", "24sk20", false},
		{"RISE 24sk20 - gra", "24sk20", true},
		{"Custom-NewModel.LZ (gra)", "custom-NewModel.LZ", true},
		{"custom-NewModel.LZ2", "custom-NewModel.LZ", false},
	} {
		if got := orderDetailMatchesPlan(map[string]interface{}{"description": tc.description}, tc.plan); got != tc.want {
			t.Errorf("%+v got=%v", tc, got)
		}
	}
}
func TestDottedPlanIsNotHardware(t *testing.T) {
	fqn := "custom.LZ.ram-64g"
	if fqnCoversOptions(fqn, []string{"LZ"}, "custom.LZ") || preorderFQNCoversOptions(fqn, []string{"LZ"}, "custom.LZ") {
		t.Fatal("model suffix treated as hardware")
	}
	if !fqnCoversOptions(fqn, []string{"ram-64g"}, "custom.LZ") {
		t.Fatal("hardware not matched")
	}
}
