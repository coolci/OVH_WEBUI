package purchase

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/types"
)

// 下单后查付款截止时间:OVH 在抢购高峰常回 429,只查一次就放弃会让历史里没有截止时间。
// 重试后必须补上 expirationTime。
func TestBackfillRetriesUntilDeadlineKnown(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case "/me/order/777":
			if calls.Add(1) <= 2 {
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"message":"Too many requests"}`)
				return
			}
			fmt.Fprint(w, `{"orderId":777,"expirationDate":"2026-11-08T10:15:00+01:00","priceWithTax":{"value":12.5,"currencyCode":"EUR"}}`)
		case "/me/order/777/status":
			fmt.Fprint(w, `"notPaid"`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"unexpected"}`)
		}
	}))
	t.Cleanup(srv.Close)
	swapEndpoint(t, "ovh-eu", srv.URL)
	orig := orderDetailRetryDelays
	orderDetailRetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { orderDetailRetryDelays = orig })

	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{ID: "acc", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}
	s.HistoryMu.Lock()
	s.History = []types.PurchaseHistoryEntry{{ID: "h", TaskID: "task", AccountID: "acc", PlanCode: "p", Datacenter: "gra",
		Options: []string{}, Status: "success", OrderID: "777", PurchaseTime: types.NowISO()}}
	s.HistoryMu.Unlock()
	client, err := s.OVH.IsolatedClientFor("acc")
	if err != nil {
		t.Fatal(err)
	}
	backfillOrderDetail(s, client, "task", "777")

	if calls.Load() != 3 {
		t.Fatalf("expected two failed attempts then a success, got %d calls", calls.Load())
	}
	s.HistoryMu.Lock()
	defer s.HistoryMu.Unlock()
	if s.History[0].ExpirationTime != "2026-11-08T10:15:00+01:00" {
		t.Fatalf("payment deadline must be filled after retries: %+v", s.History[0])
	}
}
