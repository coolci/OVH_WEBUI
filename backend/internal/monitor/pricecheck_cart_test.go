package monitor

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

// 补货时监控先验价、再发通知 / 自动下单。验价以前全串行(十来次往返),
// 验完还把配好的车删掉,下单时再从零装一辆。现在:
//   - 验价内部并发(加购 ‖ 绑定 ‖ 拉选项,配置 ‖ 选项),同一时刻会有多个请求在飞;
//   - 校验通过的车直接进预热池,下单任务按同样的键能取到,且这辆车不会被删。
func TestPriceCheckRunsConcurrentlyAndKeepsCart(t *testing.T) {
	m, st := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(st)
	ovh.ResetAuthTimeCache()
	defer ovh.ResetAuthTimeCache()

	const (
		acc  = "acc-pricecheck"
		plan = "24sk-pc"
		opt  = "ram-32g-ecc-2400-24sk-pc"
	)
	var inFlight, peak, deletes, timeCalls atomic.Int32
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			timeCalls.Add(1)
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		time.Sleep(60 * time.Millisecond) // 模拟到 OVH 的往返
		switch {
		case r.Method == http.MethodDelete:
			deletes.Add(1)
			fmt.Fprint(w, `null`)
		case r.Method == http.MethodPost && r.URL.Path == "/order/cart":
			fmt.Fprint(w, `{"cartId":"cart-pc-1"}`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/eco"):
			fmt.Fprint(w, `{"itemId":42}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/eco/options"):
			fmt.Fprintf(w, `[{"planCode":%q,"prices":[]}]`, opt)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/summary"):
			fmt.Fprint(w, `{"prices":{"withTax":{"value":12,"currencyCode":"EUR"},"withoutTax":{"value":10,"currencyCode":"EUR"},"tax":{"value":2,"currencyCode":"EUR"}}}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	swapTestEndpoint(t, "ovh-eu", srv.URL)

	// 公开目录(定 region 用)不出网
	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/") {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Request: r,
				Body: io.NopCloser(strings.NewReader(`{"plans":[]}`))}, nil
		}
		if strings.HasPrefix(srv.URL, "http://"+r.URL.Host) {
			return origTransport.RoundTrip(r)
		}
		return nil, fmt.Errorf("blocked: %s", r.URL)
	})
	defer func() { http.DefaultTransport = origTransport }()

	st.Accounts = []types.OVHAccount{{ID: acc, Name: acc, Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}

	ok, msg := m.verifyPriceAvailable(plan, "gra", map[string]interface{}{"account_id": acc, "options": []string{opt}})
	if !ok {
		t.Fatalf("验价应通过: %s (请求: %v)", msg, seen)
	}
	if peak.Load() < 2 {
		t.Errorf("验价请求应并发发出, 实际最多同时 %d 个 (请求: %v)", peak.Load(), seen)
	}
	if timeCalls.Load() > 1 {
		t.Errorf("多个隔离 client 只应真正校时 1 次, 实际 %d 次", timeCalls.Load())
	}
	time.Sleep(150 * time.Millisecond) // 给可能的异步删车留时间
	if deletes.Load() != 0 {
		t.Fatalf("验价通过的车不该被删 (删了 %d 次)", deletes.Load())
	}
	c := purchase.TakePrewarmedCart(acc, plan, "gra", []string{opt}, "", "")
	if c == nil || c.CartID != "cart-pc-1" {
		t.Fatalf("下单任务应能从预热池取到验价车, got %+v", c)
	}
}
