package purchase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

// All catalog, signed API and checkout requests are handled locally.
func TestPurchaseCouponBeforeCheckout(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, tc := range []struct {
		name, coupon                          string
		status                                int
		preorder, cancel, reconciled, network bool
		checkout, warning                     bool
		catalogWatch                          bool
	}{
		{name: "blank", coupon: "  ", checkout: true},
		{name: "accepted", coupon: " Save10 ", status: 200, checkout: true},
		{name: "preorder", coupon: "Save10", status: 200, preorder: true, checkout: true},
		{name: "invalid", coupon: "Save10", status: 400, checkout: true, warning: true},
		{name: "forbidden", coupon: "Save10", status: 403, checkout: true, warning: true},
		{name: "reconciled after rejection", coupon: "Save10", status: 400, checkout: true, warning: true, reconciled: true},
		{name: "rate limited", coupon: "Save10", status: 429},
		{name: "server error", coupon: "Save10", status: 503},
		{name: "unrecognized transport error", coupon: "Save10", network: true},
		{name: "cancelled", coupon: "Save10", cancel: true},
		{name: "concurrent catalog checkout uncertainty", coupon: "Save10", status: 200, reconciled: true, catalogWatch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			database, err := db.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			lg := logger.New(filepath.Join(dir, "test.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
			st := app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
			st.Accounts = []types.OVHAccount{{ID: "eu", Endpoint: "ovh-eu", Zone: "PT", AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}
			cfg := st.Config.Get()
			cfg.NotifyWebhookURL = "https://coupon-notify.invalid/test"
			st.Config.Set(cfg)
			if tc.reconciled {
				st.History = []types.PurchaseHistoryEntry{{ID: "previous", TaskID: "coupon-task", Status: "failed"}}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var mu sync.Mutex
			var steps []string
			couponCalls, checkoutCalls := 0, 0
			var messages []string
			recordStep := func(step string) {
				mu.Lock()
				steps = append(steps, step)
				mu.Unlock()
			}
			transport := preorderTransport(func(r *http.Request) (*http.Response, error) {
				body, status := "{}", http.StatusOK
				path := r.URL.Path
				switch {
				case r.URL.Host == "coupon-notify.invalid":
					var payload map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Errorf("decode webhook payload: %v", err)
						return nil, err
					}
					message, _ := payload["message"].(string)
					mu.Lock()
					messages = append(messages, message)
					mu.Unlock()
				case strings.HasSuffix(path, "/me/order"):
					body = `[{"orderId":123,"url":"https://example.invalid/order"}]`
				case strings.HasSuffix(path, "/auth/time"):
					body = fmt.Sprint(time.Now().Unix())
				case strings.Contains(path, "/order/catalog/public/eco"):
					body = `{"plans":[{"planCode":"24sk202"}]}`
				case strings.Contains(path, "/dedicated/server/datacenter/availabilities"):
					body = `[{"fqn":"24sk202","datacenters":[{"datacenter":"gra","availability":"1H-low"}]}]`
				case strings.HasSuffix(path, "/order/cart") && r.Method == "POST":
					recordStep("cart")
					body = `{"cartId":"coupon-cart"}`
				case strings.HasSuffix(path, "/coupon-cart/assign"):
					recordStep("assign")
				case strings.HasSuffix(path, "/coupon-cart/eco") && r.Method == "POST":
					recordStep("item")
					body = `{"itemId":1}`
				case strings.HasSuffix(path, "/item/1/configuration"):
					recordStep("config")
				case strings.HasSuffix(path, "/coupon-cart/coupon") && r.Method == "POST":
					mu.Lock()
					couponCalls++
					steps = append(steps, "coupon")
					mu.Unlock()
					var payload map[string]string
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload["coupon"] != "Save10" {
						t.Errorf("coupon must be trimmed without changing case: %v, %v", payload, err)
					}
					if tc.cancel {
						cancel()
						return nil, context.Canceled
					}
					if tc.network {
						return nil, errors.New("transport closed")
					}
					status = tc.status
					body = `["Save10"]`
					if status != 200 {
						body = `{"message":"coupon fixture rejection"}`
					}
				case strings.HasSuffix(path, "/coupon-cart/checkout") && r.Method == "POST":
					mu.Lock()
					checkoutCalls++
					steps = append(steps, "checkout")
					prev := ""
					for i := len(steps) - 2; i >= 0; i-- {
						if steps[i] != "verify" {
							prev = steps[i]
							break
						}
					}
					stepsSnapshot := append([]string(nil), steps...)
					mu.Unlock()
					if tc.coupon != "  " && prev != "coupon" {
						t.Errorf("coupon must precede checkout after configuration: %v", stepsSnapshot)
					}
					var payload map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Errorf("decode checkout payload: %v", err)
						return nil, err
					}
					if payload["autoPayWithPreferredPaymentMethod"] != false {
						t.Error("coupon changed auto-pay default")
					}
					// No real order; omit ID so the fixture also starts no detail-fetch goroutine.
					body = `{}`
					if tc.reconciled {
						status = http.StatusServiceUnavailable
						body = `{"message":"checkout timeout"}`
					}
				case strings.HasSuffix(path, "/order/cart/coupon-cart") && r.Method == "GET":
					// 预订结账前核对购物车:主机一项,月付
					recordStep("verify")
					body = `{"items":[1]}`
				case strings.HasSuffix(path, "/order/cart/coupon-cart/item/1") && r.Method == "GET":
					body = `{"itemId":1,"duration":"P1M","options":[],"settings":{"planCode":"24sk202","quantity":1}}`
				case strings.HasSuffix(path, "/order/cart/coupon-cart") && r.Method == "DELETE":
					recordStep("cleanup")
				default:
					return nil, fmt.Errorf("unexpected fixture request: %s %s", r.Method, r.URL)
				}
				return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			http.DefaultTransport = transport
			restoreShared := netfp.SwapSharedTransportForTesting(transport)
			defer restoreShared()
			defer CancelAndWaitBackfills()
			client, err := st.OVH.ClientFor("eu")
			if err != nil {
				t.Fatal(err)
			}
			client.Client = &http.Client{Transport: transport}
			item := &types.QueueItem{ID: "coupon-task", AccountID: "eu", PlanCode: "24sk202", Datacenter: "gra", Coupon: tc.coupon, Preorder: tc.preorder}
			if tc.catalogWatch {
				item.ID = "catalog-watch-scoped-job-waw"
			}
			outcome := PurchaseServer(ctx, st, item)
			CancelAndWaitBackfills()
			mu.Lock()
			gotCouponCalls := couponCalls
			gotCheckoutCalls := checkoutCalls
			gotSteps := append([]string(nil), steps...)
			gotMessages := append([]string(nil), messages...)
			mu.Unlock()
			if tc.catalogWatch {
				if !outcome.PauseTask || outcome.Success || gotCheckoutCalls != 1 {
					t.Fatalf("uncertain concurrent checkout not paused: %+v", outcome)
				}
				for _, h := range st.HistorySnapshot() {
					if h.Status == "success" {
						t.Fatal("falsely claimed another worker's order")
					}
				}
				return
			}
			wantCoupon := 1
			if strings.TrimSpace(tc.coupon) == "" {
				wantCoupon = 0
			}
			wantCheckout := 0
			if tc.checkout {
				wantCheckout = 1
			}
			if gotCouponCalls != wantCoupon || gotCheckoutCalls != wantCheckout || outcome.Success != tc.checkout || outcome.PauseTask || outcome.Attempted || outcome.Cancelled != tc.cancel {
				t.Fatalf("unexpected flow: outcome=%+v coupon=%d checkout=%d steps=%v", outcome, gotCouponCalls, gotCheckoutCalls, gotSteps)
			}
			if tc.checkout {
				history, err := st.DB.ListHistory()
				if err != nil || len(history) != 1 || history[0].Status != "success" || (history[0].CouponWarning != "") != tc.warning {
					t.Fatalf("coupon warning not persisted with successful order: %+v %v", history, err)
				}
				if len(gotMessages) != 1 || strings.Contains(gotMessages[0], "未使用优惠码") != tc.warning {
					t.Fatalf("notification missing or misleading: %v", gotMessages)
				}
				if tc.warning && !strings.Contains(history[0].CouponWarning, "未使用优惠码") {
					t.Fatal(history[0].CouponWarning)
				}
			}
			if !tc.checkout && (len(gotSteps) == 0 || gotSteps[len(gotSteps)-1] != "cleanup") {
				t.Fatalf("failed cart was not cleaned: %v", gotSteps)
			}
		})
	}
}

func TestPurchaseCoupon_RejectedWithLegacyAutoPayContinuesUnpaid(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })

	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	lg := logger.New(filepath.Join(dir, "test.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	st := app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
	st.Accounts = []types.OVHAccount{{ID: "eu", Endpoint: "ovh-eu", Zone: "PT", AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}

	ctx := context.Background()
	var checkoutCalled atomic.Bool
	var couponCalled atomic.Bool

	transport := preorderTransport(func(r *http.Request) (*http.Response, error) {
		body, status := "{}", http.StatusOK
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/auth/time"):
			body = fmt.Sprint(time.Now().Unix())
		case strings.Contains(path, "/order/catalog/public/eco"):
			body = `{"plans":[{"planCode":"24sk202"}]}`
		case strings.Contains(path, "/dedicated/server/datacenter/availabilities"):
			body = `[{"fqn":"24sk202","datacenters":[{"datacenter":"gra","availability":"1H-low"}]}]`
		case strings.HasSuffix(path, "/order/cart") && r.Method == "POST":
			body = `{"cartId":"coupon-cart"}`
		case strings.HasSuffix(path, "/coupon-cart/assign"):
		case strings.HasSuffix(path, "/coupon-cart/eco") && r.Method == "POST":
			body = `{"itemId":1}`
		case strings.HasSuffix(path, "/item/1/configuration"):
		case strings.HasSuffix(path, "/coupon-cart/coupon") && r.Method == "POST":
			couponCalled.Store(true)
			status = http.StatusBadRequest
			body = `{"message":"coupon is invalid or expired"}`
		case strings.HasSuffix(path, "/coupon-cart/checkout") && r.Method == "POST":
			checkoutCalled.Store(true)
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload["autoPayWithPreferredPaymentMethod"] != false {
				t.Errorf("legacy autoPay enabled checkout payment: %+v %v", payload, err)
			}
			body = `{"orderId":1001,"url":"https://fixture.invalid/order"}`
		case strings.HasSuffix(path, "/me/order/1001"):
			body = `{"orderId":1001,"expirationDate":"2099-01-01T00:00:00Z"}`
		case strings.HasSuffix(path, "/me/order/1001/status"):
			body = `"notPaid"`
		case strings.HasSuffix(path, "/order/cart/coupon-cart") && r.Method == "DELETE":
		default:
			return nil, fmt.Errorf("unexpected fixture request: %s %s", r.Method, r.URL)
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	http.DefaultTransport = transport
	restoreShared := netfp.SwapSharedTransportForTesting(transport)
	defer restoreShared()
	defer CancelAndWaitBackfills()
	client, err := st.OVH.ClientFor("eu")
	if err != nil {
		t.Fatal(err)
	}
	client.Client = &http.Client{Transport: transport}

	item := &types.QueueItem{
		ID:         "autopay-coupon-task",
		AccountID:  "eu",
		PlanCode:   "24sk202",
		Datacenter: "gra",
		Coupon:     "EXPIREDCODE",
		AutoPay:    true,
	}

	outcome := PurchaseServer(ctx, st, item)
	CancelAndWaitBackfills()

	if !couponCalled.Load() {
		t.Fatal("expected coupon to be called")
	}
	if !checkoutCalled.Load() || item.AutoPay {
		t.Fatal("legacy autoPay must be ignored and checkout must stay unpaid")
	}
	if outcome.PauseTask || !outcome.Success {
		t.Fatalf("expected unpaid success with coupon warning, got %+v", outcome)
	}
	if outcome.Attempted {
		t.Fatalf("expected outcome.Attempted=false, got %+v", outcome)
	}
}
