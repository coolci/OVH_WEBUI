package catalogwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/netfp"
)

type notificationTransport func(*http.Request) (*http.Response, error)

func (f notificationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Real Watcher -> PurchaseServer -> persisted history + Telegram code, with all
// HTTP requests intercepted locally. This test can never create a real order.
func TestAutomaticPurchaseWritesHistoryAndSendsTelegramOnce(t *testing.T) {
	for _, lost := range []bool{false, true} {
		t.Run(fmt.Sprint("lost-response=", lost), func(t *testing.T) { testAutomaticPurchaseNotification(t, lost) })
	}
}
func testAutomaticPurchaseNotification(t *testing.T, lost bool) {
	w, s := fixture(t)
	s.Accounts[0].AppKey = "fixture"
	s.Accounts[0].AppSecret = "fixture"
	s.Accounts[0].ConsumerKey = "fixture"
	cfg := s.Config.Get()
	cfg.TgToken = "123:fixture"
	cfg.TgChatID = "fixture-chat"
	s.Config.Set(cfg)
	var checkouts, notifications atomic.Int32
	transport := notificationTransport(func(r *http.Request) (*http.Response, error) {
		body := "{}"
		p := r.URL.Path
		switch {
		case r.URL.Host == "api.telegram.org" && strings.HasSuffix(p, "/sendMessage"):
			var payload map[string]interface{}
			if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
				return nil, e
			}
			if payload["chat_id"] != "fixture-chat" || !strings.Contains(fmt.Sprint(payload["text"]), "4242") || !strings.Contains(fmt.Sprint(payload["text"]), "24sk40-v1") {
				t.Errorf("incorrect success notification: %v", payload)
			}
			history, e := s.DB.ListHistory()
			if e != nil || len(history) != 1 || history[0].OrderID != "4242" {
				t.Error("history must be persisted before notification")
			}
			notifications.Add(1)
			body = `{"ok":true,"result":{"message_id":7}}`
		case strings.HasSuffix(p, "/auth/time"):
			body = fmt.Sprint(time.Now().Unix())
		case strings.Contains(p, "/order/catalog/public/eco"):
			body = `{"plans":[{"planCode":"24sk40-v1"}]}`
		case strings.Contains(p, "/dedicated/server/datacenter/availabilities"):
			body = `[{"fqn":"24sk40-v1","datacenters":[{"datacenter":"gra","availability":"1H-low"}]}]`
		case strings.HasSuffix(p, "/order/cart") && r.Method == "POST":
			body = `{"cartId":"notification-cart"}`
		case strings.HasSuffix(p, "/notification-cart/assign"):
		case strings.HasSuffix(p, "/notification-cart/eco") && r.Method == "POST":
			body = `{"itemId":1}`
		case strings.HasSuffix(p, "/item/1/configuration"):
		case strings.HasSuffix(p, "/item/1/requiredConfiguration"):
			body = `[]`
		case strings.HasSuffix(p, "/notification-cart/checkout") && r.Method == "POST":
			deadline, ok := r.Context().Deadline()
			if !ok || time.Until(deadline) > 61*time.Second {
				t.Error("checkout must have bounded SDK timeout")
			}
			var payload map[string]interface{}
			if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
				return nil, e
			}
			if payload["autoPayWithPreferredPaymentMethod"] != false {
				t.Error("autopay enabled")
			}
			checkouts.Add(1)
			if lost {
				var raw string
				if err := s.DB.Get(&raw, "SELECT data FROM catalog_watch WHERE id=1"); err != nil || !strings.Contains(raw, `"cartId":"notification-cart"`) || !strings.Contains(raw, `"submitted":true`) {
					t.Error("checkout journal not persisted before POST")
				}
				return nil, io.ErrUnexpectedEOF
			}
			body = `{"orderId":4242,"url":"https://example.invalid/order"}`
		case strings.HasSuffix(p, "/order/cart/notification-cart") && r.Method == "GET":
			body = `{"readOnly":true}`
		case strings.HasSuffix(p, "/notification-cart/checkout") && r.Method == "GET":
			body = `{"orderId":4242}`
		case strings.HasSuffix(p, "/me/order/4242/status"):
			body = `"notPaid"`
		case strings.HasSuffix(p, "/me/order/4242"):
			body = `{"orderId":4242,"priceWithoutTax":{"value":1,"currencyCode":"EUR"}}`
		default:
			return nil, fmt.Errorf("unexpected request blocked: %s %s", r.Method, p)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	restore := netfp.SwapSharedTransportForTesting(transport)
	defer restore()
	client, e := s.OVH.ClientFor("a")
	if e != nil {
		t.Fatal(e)
	}
	client.Client.Transport = transport
	current := snapshot("baseline")
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) { return current, nil }
	if e = w.Poll(context.Background()); e != nil {
		t.Fatal(e)
	}
	current = snapshot("baseline")
	c := Candidate{PlanCode: "24sk40-v1", Datacenters: []string{"gra"}}
	current[key("", c)] = c
	if e = w.Poll(context.Background()); e != nil {
		t.Fatal(e)
	}
	w.Attempt(context.Background())
	if lost {
		if sole(t, w).Status != "uncertain" {
			t.Fatal("lost response must remain uncertain")
		}
		fetcher := w.fetch
		w = New(s)
		w.fetch = fetcher // reconcile across restart, never checkout a second time
		w.Reconcile(context.Background())
		w.Reconcile(context.Background())
	}
	if sole(t, w).Status != "completed" || sole(t, w).OrderID != "4242" {
		t.Fatalf("purchase failed: %+v", sole(t, w))
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		hs, e := s.DB.ListHistory()
		if e == nil && len(hs) == 1 && hs[0].OrderStatus == "notPaid" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("history status was not persisted")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if e = w.Poll(context.Background()); e != nil {
		t.Fatal(e)
	}
	w.Attempt(context.Background())
	New(s).Attempt(context.Background())
	if checkouts.Load() != 1 || notifications.Load() != 1 {
		t.Fatalf("checkout=%d notification=%d", checkouts.Load(), notifications.Load())
	}
	history := s.OrderService.ListHistory()
	if len(history) != 1 || history[0].OrderID != "4242" || !strings.HasPrefix(history[0].TaskID, "catalog-watch-") {
		t.Fatal("history API service missing watcher order")
	}
}
