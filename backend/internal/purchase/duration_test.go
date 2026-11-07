package purchase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
)

// 购买时长(移植上游 5aa4df0 / 3c177bc):任务选什么周期就按什么周期下单,
// 基础商品与配件同周期,目录里没有该周期时确定性失败,绝不悄悄换成月付。
// 全部请求由本地 fixture 应答,不会真实下单。

type durationFixture struct {
	mu        sync.Mutex
	basePosts []map[string]interface{}
	optPosts  []map[string]interface{}
	ecoGets   atomic.Int32
	checkout  atomic.Int32
}

func (f *durationFixture) bases() []map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]interface{}(nil), f.basePosts...)
}

func (f *durationFixture) opts() []map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]interface{}(nil), f.optPosts...)
}

// ecoOffers: GET /order/cart/{id}/eco 的响应;optionPrices: eco/options 里 ram-64g 的 prices
func newDurationFixture(t *testing.T, plan, ecoOffers, optionPrices string) (*app.State, *durationFixture) {
	t.Helper()
	s := quickOrderTestState(t)
	ClearPrewarmPool(nil)
	resetEcoOptionsCache()
	s.Accounts = []types.OVHAccount{{ID: "dur-account", Endpoint: "ovh-eu", Zone: "FI", AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}
	f := &durationFixture{}
	decode := func(r *http.Request) map[string]interface{} {
		body := map[string]interface{}{}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		return body
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/me/order":
			io.WriteString(w, `[]`)
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprintf(w, `[{"fqn":%q,"datacenters":[{"datacenter":"gra","availability":"1H"}]}]`, plan+".ram-64g")
		case r.URL.Path == "/order/cart" && r.Method == http.MethodPost:
			io.WriteString(w, `{"cartId":"dur-cart"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			io.WriteString(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodGet:
			f.ecoGets.Add(1)
			io.WriteString(w, ecoOffers)
		case strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodPost:
			body := decode(r)
			f.mu.Lock()
			f.basePosts = append(f.basePosts, body)
			f.mu.Unlock()
			io.WriteString(w, `{"itemId":1}`)
		case strings.HasSuffix(r.URL.Path, "/requiredConfiguration"):
			io.WriteString(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/configuration"):
			io.WriteString(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco/options") && r.Method == http.MethodGet:
			fmt.Fprintf(w, `[{"planCode":"ram-64g","prices":%s}]`, optionPrices)
		case strings.HasSuffix(r.URL.Path, "/eco/options"):
			body := decode(r)
			f.mu.Lock()
			f.optPosts = append(f.optPosts, body)
			f.mu.Unlock()
			io.WriteString(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			f.checkout.Add(1)
			io.WriteString(w, `{"orderId":777,"url":"https://dur.invalid/order"}`)
		case r.Method == http.MethodDelete:
			io.WriteString(w, `null`)
		default:
			io.WriteString(w, `null`)
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
		return nil, fmt.Errorf("external request blocked: %s", r.URL)
	})
	http.DefaultTransport = transport
	restore := netfp.SwapSharedTransportForTesting(transport)
	t.Cleanup(func() { restore(); http.DefaultTransport = base; ClearPrewarmPool(nil); resetEcoOptionsCache() })
	return s, f
}

const yearlyAndMonthlyOffers = `[{"planCode":"dur-plan","prices":[
 {"duration":"P1M","pricingMode":"default","capacities":["renew"]},
 {"duration":"P12M","pricingMode":"upfront12","capacities":["renew"]}]}]`

const bothPeriodOptionPrices = `[
 {"duration":"P1M","pricingMode":"default","capacities":["renew"]},
 {"duration":"P12M","pricingMode":"upfront12","capacities":["renew"]}]`

func durationItem(duration string) *types.QueueItem {
	return &types.QueueItem{ID: "dur-task", AccountID: "dur-account", PlanCode: "dur-plan", Datacenter: "gra",
		Options: []string{"ram-64g"}, Status: "running", Force: true, Duration: duration}
}

func TestYearlyTaskOrdersBaseAndAddonsYearly(t *testing.T) {
	s, f := newDurationFixture(t, "dur-plan", yearlyAndMonthlyOffers, bothPeriodOptionPrices)
	out := PurchaseServer(context.Background(), s, durationItem("P12M"))
	if !out.Success || f.checkout.Load() != 1 {
		t.Fatalf("yearly order should succeed: %+v checkout=%d", out, f.checkout.Load())
	}
	bases := f.bases()
	if len(bases) != 1 || bases[0]["duration"] != "P12M" || bases[0]["pricingMode"] != "upfront12" {
		t.Fatalf("base item must be posted as P12M/upfront12 from the cart offer, got %v", bases)
	}
	opts := f.opts()
	if len(opts) != 1 || opts[0]["duration"] != "P12M" || opts[0]["pricingMode"] != "upfront12" {
		t.Fatalf("addon must follow the base period, got %v", opts)
	}
	s.HistoryMu.Lock()
	defer s.HistoryMu.Unlock()
	if len(s.History) == 0 || s.History[len(s.History)-1].Duration != "P12M" {
		t.Fatalf("history must record the purchase period: %+v", s.History)
	}
}

// 月付保持零额外请求的快速路径:不查 GET /eco
func TestMonthlyTaskKeepsFastPath(t *testing.T) {
	s, f := newDurationFixture(t, "dur-plan", yearlyAndMonthlyOffers, bothPeriodOptionPrices)
	out := PurchaseServer(context.Background(), s, durationItem(""))
	if !out.Success {
		t.Fatalf("monthly order should succeed: %+v", out)
	}
	if f.ecoGets.Load() != 0 {
		t.Fatalf("monthly path must not look up offers first, GET /eco called %d times", f.ecoGets.Load())
	}
	if bases := f.bases(); len(bases) != 1 || bases[0]["duration"] != "P1M" || bases[0]["pricingMode"] != "default" {
		t.Fatalf("legacy empty duration must order P1M/default, got %v", bases)
	}
}

func TestUnofferedPeriodFailsWithoutCheckout(t *testing.T) {
	monthlyOnly := `[{"planCode":"dur-plan","prices":[{"duration":"P1M","pricingMode":"default","capacities":["renew"]}]}]`
	s, f := newDurationFixture(t, "dur-plan", monthlyOnly, bothPeriodOptionPrices)
	out := PurchaseServer(context.Background(), s, durationItem("P24M"))
	if out.Success || !out.Fatal || f.checkout.Load() != 0 || len(f.bases()) != 0 {
		t.Fatalf("an unoffered period must fail deterministically before adding anything: %+v checkout=%d bases=%v",
			out, f.checkout.Load(), f.bases())
	}
	if !strings.Contains(out.Reason, "2 年") {
		t.Fatalf("reason should name the period: %q", out.Reason)
	}
}

func TestAddonWithoutSelectedPeriodCancelsOrder(t *testing.T) {
	monthlyOptionOnly := `[{"duration":"P1M","pricingMode":"default","capacities":["renew"]}]`
	s, f := newDurationFixture(t, "dur-plan", yearlyAndMonthlyOffers, monthlyOptionOnly)
	out := PurchaseServer(context.Background(), s, durationItem("P12M"))
	if out.Success || !out.Fatal || f.checkout.Load() != 0 || len(f.opts()) != 0 {
		t.Fatalf("a monthly-only addon must not be attached to a yearly server: %+v checkout=%d opts=%v",
			out, f.checkout.Load(), f.opts())
	}
}

func TestInvalidDurationIsFatal(t *testing.T) {
	s, f := newDurationFixture(t, "dur-plan", yearlyAndMonthlyOffers, bothPeriodOptionPrices)
	out := PurchaseServer(context.Background(), s, durationItem("forever"))
	if out.Success || !out.Fatal || f.checkout.Load() != 0 {
		t.Fatalf("invalid duration must be fatal: %+v", out)
	}
}

// 预热车按时长分键:月付车绝不能被年付任务拿去直接结账
func TestPrewarmedCartIsKeyedByDuration(t *testing.T) {
	ClearPrewarmPool(nil)
	t.Cleanup(func() { ClearPrewarmPool(nil) })
	PutPrewarmedCart(&PrewarmedCart{CartID: "monthly", AccountID: "a", PlanCode: "p", Datacenter: "gra", CreatedAt: time.Now()})
	if HasPrewarmedCart("a", "p", "gra", nil, "", "P12M") {
		t.Fatal("a monthly cart must not satisfy a yearly lookup")
	}
	if TakePrewarmedCart("a", "p", "gra", nil, "", "P12M") != nil {
		t.Fatal("a yearly task took the monthly cart")
	}
	// 空值和 P1M 是同一个周期(旧任务兼容)
	if !HasPrewarmedCart("a", "p", "gra", nil, "", "P1M") || TakePrewarmedCart("a", "p", "gra", nil, "", "") == nil {
		t.Fatal("empty duration must match the monthly cart")
	}
	if prewarmKey("a", "p", "gra", nil, "", "bogus") == prewarmKey("a", "p", "gra", nil, "", "") {
		t.Fatal("an invalid duration must never collide with a valid key")
	}
}

func TestPrewarmAssemblesSelectedPeriod(t *testing.T) {
	s, f := newDurationFixture(t, "dur-plan", yearlyAndMonthlyOffers, bothPeriodOptionPrices)
	cart, err := AssemblePrewarmedCart(context.Background(), s, "dur-account", "dur-plan", "gra", []string{"ram-64g"}, "", "P12M")
	if err != nil {
		t.Fatal(err)
	}
	if cart.Duration != "P12M" || !HasPrewarmedCart("dur-account", "dur-plan", "gra", []string{"ram-64g"}, "", "P12M") {
		t.Fatalf("prewarmed cart must be stored under P12M: %+v", cart)
	}
	if bases := f.bases(); len(bases) != 1 || bases[0]["duration"] != "P12M" {
		t.Fatalf("prewarm base item period wrong: %v", bases)
	}
	if HasPrewarmedCart("dur-account", "dur-plan", "gra", []string{"ram-64g"}, "", "") {
		t.Fatal("yearly prewarmed cart leaked into the monthly key")
	}
}

func TestQuickOrderCarriesDuration(t *testing.T) {
	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{ID: "qo", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}
	PutPrewarmedCart(&PrewarmedCart{CartID: "c", AccountID: "qo", PlanCode: "qo-plan", Datacenter: "gra", Duration: "P12M", CreatedAt: time.Now()})
	t.Cleanup(func() { ClearPrewarmPool(nil) })
	res := EnqueueQuickOrder(s, QuickOrderParams{AccountID: "qo", PlanCode: "qo-plan", Datacenter: "gra",
		Options: []string{"x"}, FromMonitor: true, SkipDuplicateCheck: true, Force: true, Quantity: 1, Duration: "p1y"})
	if !res.Success {
		t.Fatalf("enqueue failed: %+v", res)
	}
	s.QueueMu.Lock()
	defer s.QueueMu.Unlock()
	if len(s.Queue) != 1 || s.Queue[0].Duration != "P12M" {
		t.Fatalf("queued item must carry the normalized period P12M: %+v", s.Queue)
	}
	if bad := EnqueueQuickOrder(s, QuickOrderParams{AccountID: "qo", PlanCode: "qo-plan", Datacenter: "gra", Duration: "weekly"}); bad.Success || bad.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("invalid duration must be rejected with 400: %+v", bad)
	}
}
