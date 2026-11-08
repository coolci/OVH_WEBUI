package purchase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
)

// 没指定配件的队列任务:预热按库存 FQN 推断配件装车(和下单时同一口径),
// 补货时下单直接命中这辆预热车,不再从零装车。
func TestNoOptionTaskPrewarmIsHitAtCheckout(t *testing.T) {
	s := quickOrderTestState(t)
	ClearPrewarmPool(nil)
	resetEcoOptionsCache()
	resetFQNSnapshots()
	ResetQueuePrewarmCoord()
	t.Cleanup(func() { resetFQNSnapshots(); ResetQueuePrewarmCoord() })
	s.Accounts = []types.OVHAccount{{ID: "inf-acc", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}

	const plan = "inf-plan"
	var inStock atomic.Bool
	var carts, checkouts atomic.Int32
	var checkedOutCart atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			gra := "unavailable"
			if inStock.Load() {
				gra = "1H"
			}
			fmt.Fprintf(w, `[{"fqn":"%s.ram-64g-ecc.softraid-2x480ssd","datacenters":[{"datacenter":"gra","availability":%q},{"datacenter":"rbx","availability":"unavailable"}]},`+
				`{"fqn":"%s.ram-32g-ecc.softraid-2x2000sa","datacenters":[{"datacenter":"rbx","availability":"unavailable"}]}]`, plan, gra, plan)
		case r.URL.Path == "/order/cart" && r.Method == http.MethodPost:
			n := carts.Add(1)
			fmt.Fprintf(w, `{"cartId":"cart-%d"}`, n)
		case strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodPost:
			io.WriteString(w, `{"itemId":1}`)
		case strings.HasSuffix(r.URL.Path, "/eco/options") && r.Method == http.MethodGet:
			io.WriteString(w, `[{"planCode":"ram-64g-ecc-inf","prices":[]},{"planCode":"softraid-2x480ssd-inf","prices":[]},`+
				`{"planCode":"ram-32g-ecc-inf","prices":[]},{"planCode":"softraid-2x2000sa-inf","prices":[]}]`)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			checkouts.Add(1)
			checkedOutCart.Store(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/order/cart/"), "/checkout"))
			io.WriteString(w, `{"orderId":555,"url":"https://inf.invalid/order"}`)
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

	item := &types.QueueItem{ID: "inf-task", AccountID: "inf-acc", PlanCode: plan, Datacenter: "gra", Status: "running", Force: true}
	inferred := []string{"ram-64g-ecc", "softraid-2x480ssd"}

	// 第一次:FQN 列表还没缓存,后台去拉,本轮不预热;之后再触发就按推断出的配件装车
	deadline := time.Now().Add(3 * time.Second)
	for !HasPrewarmedCart("inf-acc", plan, "gra", inferred, "", "") && time.Now().Before(deadline) {
		TriggerPrewarmForQueueItem(context.Background(), s, item)
		time.Sleep(30 * time.Millisecond)
	}
	if !HasPrewarmedCart("inf-acc", plan, "gra", inferred, "", "") {
		t.Fatalf("没指定配件的任务应按 FQN 推断出的配件 %v 预热", inferred)
	}
	if carts.Load() != 1 {
		t.Fatalf("gra 只登记了一种组合,应只热 1 辆车, got %d", carts.Load())
	}

	inStock.Store(true)
	out := PurchaseServer(context.Background(), s, item)
	if !out.Success {
		t.Fatalf("补货后应下单成功: %+v", out)
	}
	if carts.Load() != 1 || checkouts.Load() != 1 || checkedOutCart.Load() != "cart-1" {
		t.Fatalf("下单应直接用预热车 cart-1 结账,不再新建车: carts=%d checkouts=%d cart=%v",
			carts.Load(), checkouts.Load(), checkedOutCart.Load())
	}
}
