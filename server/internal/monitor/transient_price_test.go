package monitor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

// TestRestockTransientPriceFailureStillEnqueuesWithoutCartStorm 验证补货瞬间遇到暂时性验价失败
// （如 429/504/超时）时：
// 1. 不丢失补货自动下单机会（仍照常入队由抢购任务完整核对购物车）；
// 2. 同一轮内不重复发起冗余建车（不触发二次 getPriceWithTimeout 与按钮预热建车风暴）。
func TestRestockTransientPriceFailureStillEnqueuesWithoutCartStorm(t *testing.T) {
	const plan = "24sys-transient"
	m, state := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(state)

	var cartCalls atomic.Int32
	var stock atomic.Value
	stock.Store("unavailable")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprintf(w, `[{"fqn":"%s.ram-32g.ssd","memory":"ram-32g","storage":"ssd","datacenters":[{"datacenter":"gra","availability":%q}]}]`,
				plan, stock.Load().(string))
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			cartCalls.Add(1)
			fmt.Fprintf(w, `{"cartId":"transient-cart-%d"}`, cartCalls.Load())
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"message":"Too Many Requests"}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(srv.Close)
	swapTestEndpoint(t, "ovh-eu", srv.URL)

	installMockCatalogTransport(t, func(r *http.Request) (int, string, bool) {
		return 200, fmt.Sprintf(`{"plans":[{"planCode":%q,"pricings":[{"capacities":["renew"],"mode":"default","price":2999000000,"interval":1}],"configurations":[{"name":"region","values":["europe"]}]}]}`, plan), true
	})

	accID := "transient-" + uuid.NewString()
	state.Accounts = []types.OVHAccount{
		{ID: accID, Name: "FR", Endpoint: "ovh-eu", Zone: "FR", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
	}

	sub := &Subscription{
		PlanCode:            plan,
		AutoOrder:           true,
		Quantity:            1,
		AutoOrderAccountID:  accID,
		AutoOrderAccountIDs: []string{accID},
		Datacenters:         []string{"gra"},
		NotifyAvailable:     true,
		NotifyUnavailable:   true,
	}
	m.subscriptions = []*Subscription{sub}

	// 基线：无货
	m.CheckAvailabilityChange(sub, "t-baseline")
	if len(state.Queue) != 0 {
		t.Fatalf("expected 0 queued orders while unavailable, got %d", len(state.Queue))
	}

	// 补货瞬间验价遇到 429（暂时性失败）：仍需入队下单，且只发起 1 次建车
	stock.Store("1H-low")
	m.CheckAvailabilityChange(sub, "t-restock")
	time.Sleep(40 * time.Millisecond)

	if len(state.Queue) != 1 {
		t.Fatalf("expected restock with transient 429 price check to enqueue 1 order, got %d", len(state.Queue))
	}
	if got := cartCalls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 cart call on transient price failure (no duplicate price/prewarm storm), got %d", got)
	}
}

// TestVerifyPriceWithTransient_TimeoutReturnsTransient 验证超过 priceVerifyWait 上限会按暂时性失败快速放行。
func TestVerifyPriceWithTransient_TimeoutReturnsTransient(t *testing.T) {
	m, state := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(state)

	oldWait := priceVerifyWait
	priceVerifyWait = 60 * time.Millisecond
	t.Cleanup(func() { priceVerifyWait = oldWait })

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			<-release
			fmt.Fprint(w, `{"cartId":"slow-cart"}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	swapTestEndpoint(t, "ovh-eu", srv.URL)

	accID := "slow-" + uuid.NewString()
	state.Accounts = []types.OVHAccount{
		{ID: accID, Name: "FR", Endpoint: "ovh-eu", Zone: "FR", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
	}

	start := time.Now()
	ok, reason, transient := m.verifyPriceWithTransient("24sys-slow", "gra", map[string]interface{}{
		"account_id": accID,
		"subsidiary": "FR",
	})
	elapsed := time.Since(start)
	if ok || !transient || reason == "" {
		t.Fatalf("expected timeout to return (false, non-empty, true), got ok=%v transient=%v reason=%q", ok, transient, reason)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected verifyPriceWithTransient to return near priceVerifyWait, took %s", elapsed)
	}
}
