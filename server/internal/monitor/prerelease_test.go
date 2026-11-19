package monitor

import (
	"fmt"
	"io"
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

// 上游 issue #12:库存先于销售目录公布的 Eco 型号,购物车验价也通不过。
// 库存在有货/无货之间抖动时,不能每次都发"验价失败"提醒,也不能下单;
// 上架(购物车验价通过)后要正常触发一次补货与自动下单。所有请求都由本地 fixture 应答。
func TestPrereleasePlanFlappingDoesNotAlertUntilCartVerifies(t *testing.T) {
	const plan = "27sysle02-v1"
	option := "softraid-2x960nvme-" + plan
	m, state := setupTestMonitor(t)
	var stock atomic.Value
	stock.Store("1H-low")
	var checkouts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprintf(w, `[{"fqn":%q,"storage":"softraid-2x960nvme","datacenters":[{"datacenter":"gra","availability":%q}]}]`,
				plan+".softraid-2x960nvme", stock.Load().(string))
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			fmt.Fprint(w, `{"cartId":"prerelease-cart"}`)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			// 未开售:OVH 拒绝把这个型号加进购物车
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"message":"Product %s is not available"}`, plan)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			checkouts.Add(1)
			fmt.Fprint(w, `{}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(server.Close)
	swapTestEndpoint(t, "ovh-eu", server.URL)
	account := "prerelease-" + uuid.NewString()
	state.Accounts = []types.OVHAccount{{ID: account, Name: "A", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return nil, fmt.Errorf("test blocked external request: %s", r.URL)
		}
		// 销售目录还没收录这个型号
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"plans":[]}`)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	t.Cleanup(func() { purchase.ClearPrewarmPool(state) })

	m.subscriptions = []*Subscription{{PlanCode: plan, AutoOrder: true, Quantity: 1, AutoOrderAccountID: account,
		AutoOrderAccountIDs: []string{account}, Datacenters: []string{"gra"}, Options: []string{option},
		NotifyAvailable: true, NotifyUnavailable: true}}
	sub := m.subscriptions[0]

	countPriceFailures := func() int {
		n := 0
		for _, h := range sub.snapshot().History {
			if h.ChangeType == "price_check_failed" || h.Status == "price_check_failed" {
				n++
			}
		}
		return n
	}
	for i, value := range []string{"1H-low", "unavailable", "1H-low", "comingSoon", "1H-high", "unavailable", "1H-low"} {
		stock.Store(value)
		m.CheckAvailabilityChange(sub, fmt.Sprintf("flap-%d", i))
	}
	if n := countPriceFailures(); n != 0 {
		t.Fatalf("unlisted plan produced %d price-check-failed alerts while stock flapped", n)
	}
	if len(state.Queue) != 0 || checkouts.Load() != 0 {
		t.Fatalf("unlisted plan was ordered: queue=%d checkouts=%d", len(state.Queue), checkouts.Load())
	}
	if !strings.Contains(sub.snapshot().LastCheckError, "销售目录") {
		t.Fatalf("missing visible waiting-for-catalog diagnostic: %q", sub.snapshot().LastCheckError)
	}
	for key, status := range sub.statusSnapshot() {
		if status != "unavailable" {
			t.Fatalf("baseline must stay unavailable before listing: %s=%s", key, status)
		}
	}

	// 开售:购物车验价通过(生产里由预热就绪车或询价成功表达),应触发一次正常补货下单。
	purchase.PutPrewarmedCart(&purchase.PrewarmedCart{CartID: "ready", AccountID: account, PlanCode: plan, Datacenter: "gra", Options: []string{option}, CreatedAt: time.Now()})
	stock.Store("1H-low")
	m.CheckAvailabilityChange(sub, "listed")
	if len(state.Queue) != 1 {
		t.Fatalf("listed plan must enqueue exactly one restock order, got %d", len(state.Queue))
	}
	m.CheckAvailabilityChange(sub, "listed-sustained")
	if len(state.Queue) != 1 {
		t.Fatalf("sustained stock re-ordered: %d", len(state.Queue))
	}
}
