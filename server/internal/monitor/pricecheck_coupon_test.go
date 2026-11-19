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

	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// 设了 TG 默认优惠码时:只通知的订阅,验价车套上优惠码再进预热池(按钮下单会带码,这样才对得上);
// 开了自动下单的订阅不套码(自动下单任务不带码)。
func TestPriceCheckCartFollowsTelegramCoupon(t *testing.T) {
	for _, tc := range []struct {
		name       string
		autoOrder  bool
		wantCoupon string
	}{
		{"只通知 → 套默认优惠码", false, "SAVE10"},
		{"自动下单 → 不套码", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, st := setupTestMonitor(t)
			defer purchase.ClearPrewarmPool(st)
			ResetPrewarmCoordinator()
			defer ResetPrewarmCoordinator()
			ovh.ResetAuthTimeCache()
			defer ovh.ResetAuthTimeCache()

			plan := fmt.Sprintf("24sk-coupon-%v", tc.autoOrder) // 每个子用例独立 planCode,避开进程级询价缓存
			const opt = "ram-32g-ecc-2400"
			var couponPosts atomic.Int32
			var couponBody atomic.Value
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path == "/auth/time":
					fmt.Fprint(w, time.Now().Unix())
				case r.Method == http.MethodPost && r.URL.Path == "/order/cart":
					fmt.Fprint(w, `{"cartId":"cart-coupon"}`)
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/coupon"):
					b, _ := io.ReadAll(r.Body)
					couponBody.Store(string(b))
					couponPosts.Add(1)
					fmt.Fprint(w, `{}`)
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/eco"):
					fmt.Fprint(w, `{"itemId":42}`)
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/eco/options"):
					fmt.Fprintf(w, `[{"planCode":%q,"prices":[]}]`, opt)
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/summary"):
					fmt.Fprint(w, `{"prices":{"withoutTax":{"value":10,"currencyCode":"EUR"},"withTax":{"value":12,"currencyCode":"EUR"}}}`)
				default:
					fmt.Fprint(w, `{}`)
				}
			}))
			defer srv.Close()
			swapTestEndpoint(t, "ovh-eu", srv.URL)
			orig := http.DefaultTransport
			http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				if strings.Contains(r.URL.Path, "/order/catalog/public/") {
					return &http.Response{StatusCode: 200, Header: make(http.Header), Request: r, Body: io.NopCloser(strings.NewReader(`{"plans":[]}`))}, nil
				}
				if strings.HasPrefix(srv.URL, "http://"+r.URL.Host) {
					return orig.RoundTrip(r)
				}
				return nil, fmt.Errorf("blocked: %s", r.URL)
			})
			defer func() { http.DefaultTransport = orig }()

			const acc = "acc-coupon"
			st.Accounts = []types.OVHAccount{{ID: acc, Name: acc, Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}
			if err := telegram.SetActiveCoupon(st, "SAVE10"); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = telegram.ClearActiveCoupon(st) }()

			ok, msg := m.verifyPriceAvailable(plan, "gra", map[string]interface{}{
				"account_id": acc, "options": []string{opt}, "auto_order": tc.autoOrder,
			})
			if !ok {
				t.Fatalf("验价应通过: %s", msg)
			}
			deadline := time.Now().Add(2 * time.Second)
			for !purchase.HasPrewarmedCart(acc, plan, "gra", []string{opt}, tc.wantCoupon, "") && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			c := purchase.TakePrewarmedCart(acc, plan, "gra", []string{opt}, tc.wantCoupon, "")
			if c == nil || c.CartID != "cart-coupon" {
				t.Fatalf("验价车应按优惠码 %q 进预热池, got %+v", tc.wantCoupon, c)
			}
			if tc.wantCoupon != "" {
				if couponPosts.Load() != 1 || !strings.Contains(fmt.Sprint(couponBody.Load()), "SAVE10") {
					t.Fatalf("应给验价车套一次优惠码 SAVE10, posts=%d body=%v", couponPosts.Load(), couponBody.Load())
				}
			} else if couponPosts.Load() != 0 {
				t.Fatalf("自动下单的验价车不该套码, posts=%d", couponPosts.Load())
			}
		})
	}
}
