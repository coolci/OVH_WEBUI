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

// 订阅选了年付:补货时入队的任务必须带上 P12M;只有月付预热车时不能拿它当"已验价"。
// 全部请求由本地 fixture 应答,不会真实下单。
func TestSubscriptionDurationReachesQueuedOrder(t *testing.T) {
	const plan = "27dur01"
	option := "ram-64g-" + plan
	m, state := setupTestMonitor(t)
	var stock atomic.Value
	stock.Store("unavailable")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprintf(w, `[{"fqn":%q,"memory":"ram-64g","datacenters":[{"datacenter":"gra","availability":%q}]}]`,
				plan+".ram-64g", stock.Load().(string))
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			fmt.Fprint(w, `{"cartId":"dur-monitor-cart"}`)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			// 现场询价一律失败:能下单只可能是因为命中了同周期的预热车
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"message":"not orderable in this fixture"}`)
		default:
			fmt.Fprint(w, `[]`)
		}
	}))
	t.Cleanup(server.Close)
	swapTestEndpoint(t, "ovh-eu", server.URL)
	account := "dur-" + uuid.NewString()
	state.Accounts = []types.OVHAccount{{ID: account, Name: "A", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return nil, fmt.Errorf("test blocked external request: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"plans":[]}`)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	t.Cleanup(func() { purchase.ClearPrewarmPool(state) })

	m.subscriptions = []*Subscription{{PlanCode: plan, AutoOrder: true, Quantity: 1, AutoOrderAccountID: account,
		AutoOrderAccountIDs: []string{account}, Datacenters: []string{"gra"}, Options: []string{option},
		NotifyAvailable: true, Duration: "P12M"}}
	sub := m.subscriptions[0]
	m.CheckAvailabilityChange(sub, "baseline")

	// 只有月付预热车:年付订阅不能把它当成已验价,也不能下单
	purchase.PutPrewarmedCart(&purchase.PrewarmedCart{CartID: "monthly", AccountID: account, PlanCode: plan,
		Datacenter: "gra", Options: []string{option}, CreatedAt: time.Now()})
	stock.Store("1H-low")
	m.CheckAvailabilityChange(sub, "monthly-cart-only")
	if len(state.Queue) != 0 {
		t.Fatalf("a monthly prewarmed cart must not verify a yearly subscription: queue=%+v", state.Queue)
	}

	// 年付预热车就绪 → 一次补货下单,任务带 P12M
	stock.Store("unavailable")
	m.CheckAvailabilityChange(sub, "gone")
	purchase.PutPrewarmedCart(&purchase.PrewarmedCart{CartID: "yearly", AccountID: account, PlanCode: plan,
		Datacenter: "gra", Options: []string{option}, Duration: "P12M", CreatedAt: time.Now()})
	stock.Store("1H-low")
	m.CheckAvailabilityChange(sub, "yearly-cart")
	state.QueueMu.Lock()
	defer state.QueueMu.Unlock()
	if len(state.Queue) != 1 {
		t.Fatalf("yearly subscription must enqueue exactly one order, got %d", len(state.Queue))
	}
	if state.Queue[0].Duration != "P12M" {
		t.Fatalf("queued order lost the subscription period: %q", state.Queue[0].Duration)
	}
}
