package monitor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// 上架通知必须为每颗一键下单按钮热好车,且车的键要和点按钮后入队的任务一致
// (机房来自 availableDCs 的 "dc" 字段、默认账户、当前 TG 优惠码、月付)。
// 以前读的是不存在的 "datacenter" 字段,通知照发、车一辆都没热;
// 也没带优惠码,设了默认优惠码时就算热了也命中不了。
func TestAvailabilityAlertPrewarmsCartsForButtons(t *testing.T) {
	m, st := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(st)
	ResetPrewarmCoordinator()
	defer ResetPrewarmCoordinator()

	var carts, coupons atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			n := carts.Add(1)
			fmt.Fprintf(w, `{"cartId":"cart-notify-%d"}`, n)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/coupon"):
			coupons.Add(1)
			fmt.Fprint(w, `{}`)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			fmt.Fprint(w, `{"itemId":1001}`)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/eco/options"):
			fmt.Fprint(w, `[]`)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	swapTestEndpoint(t, "ovh-eu", srv.URL)

	const acc = "acc-notify"
	st.Accounts = []types.OVHAccount{{ID: acc, Name: acc, Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}
	if err := telegram.SetActiveCoupon(st, "SAVE10"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = telegram.ClearActiveCoupon(st) }()

	// 与 check.go 真实传入的形状一致:机房在 "dc" 字段
	dcs := []map[string]interface{}{{"dc": "gra", "status": "1H"}, {"dc": "rbx", "status": "1H"}}
	if n := m.prewarmAlertButtons("24sk-notify", dcs, nil); n != 2 {
		t.Fatalf("应为 2 颗按钮各热一辆车,实际触发 %d", n)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if purchase.HasPrewarmedCart(acc, "24sk-notify", "gra", nil, "SAVE10", "") &&
			purchase.HasPrewarmedCart(acc, "24sk-notify", "rbx", nil, "SAVE10", "") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, dc := range []string{"gra", "rbx"} {
		if !purchase.HasPrewarmedCart(acc, "24sk-notify", dc, nil, "SAVE10", "") {
			t.Fatalf("%s 没有按 (默认账户, 优惠码 SAVE10, 月付) 热好的车 (建车 %d 次)", dc, carts.Load())
		}
	}
	if coupons.Load() != 2 {
		t.Errorf("每辆预热车都应套用优惠码,实际 %d 次", coupons.Load())
	}

	// 点按钮入队的任务按同样的键取车,能取到
	if c := purchase.TakePrewarmedCart(acc, "24sk-notify", "GRA", []string{}, "SAVE10", "P1M"); c == nil {
		t.Fatal("按钮任务的键取不到预热车")
	}
}
