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

func TestDeliveredOrderStillRefreshesAfterRetraction(t *testing.T) {
	var statusCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case "/me/order/1001/status":
			statusCalls.Add(1)
			fmt.Fprint(w, `"cancelled"`)
		default:
			t.Errorf("unexpected offline request: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	swapEndpoint(t, "ovh-eu", srv.URL)
	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{ID: "acc", Endpoint: "ovh-eu", Zone: "FI", AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}
	s.History = []types.PurchaseHistoryEntry{{ID: "retraction-1001", AccountID: "acc", Status: "success", PlanCode: "【模拟】plan", OrderID: "1001", OrderStatus: "delivered", ExpirationTime: "2099-01-01T00:00:00Z", PurchaseTime: types.NowISO()}}
	if n := RefreshOrderStatuses(s, true); n != 0 || statusCalls.Load() != 0 {
		t.Fatal("ordinary delivered orders should remain terminal")
	}
	if err := s.DB.ChangeOrderCancellation("acc", 1001, "retract"); err != nil {
		t.Fatal(err)
	}
	if n := RefreshOrderStatuses(s, true); n != 1 || statusCalls.Load() != 1 {
		t.Fatalf("retracted delivered order skipped: updated=%d calls=%d", n, statusCalls.Load())
	}
	if got := s.HistorySnapshot()[0].OrderStatus; got != "cancelled" {
		t.Fatalf("status=%s", got)
	}
}

// 下单时没补到付款截止时间的订单,状态刷新时再补。截止时间完全取 OVH 的
// billing.Order.expirationDate:促销单 15 分钟、普通单 15 天,本地不做任何假设。
func TestStatusRefreshFillsMissingDeadlineFromOVH(t *testing.T) {
	short := time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339)
	long := time.Now().Add(15 * 24 * time.Hour).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case "/me/order/1/status", "/me/order/2/status":
			fmt.Fprint(w, `"notPaid"`)
		case "/me/order/1":
			fmt.Fprintf(w, `{"orderId":1,"expirationDate":%q}`, short)
		case "/me/order/2":
			fmt.Fprintf(w, `{"orderId":2,"expirationDate":%q,"retractionDate":%q}`, long, long)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"unexpected"}`)
		}
	}))
	t.Cleanup(srv.Close)
	swapEndpoint(t, "ovh-eu", srv.URL)

	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{ID: "acc", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}
	now := types.NowISO()
	s.HistoryMu.Lock()
	s.History = []types.PurchaseHistoryEntry{
		{ID: "flash", AccountID: "acc", TaskID: "t1", PlanCode: "p", Datacenter: "gra", Options: []string{}, Status: "success", OrderID: "1", PurchaseTime: now},
		{ID: "normal", AccountID: "acc", TaskID: "t2", PlanCode: "p", Datacenter: "gra", Options: []string{}, Status: "success", OrderID: "2", PurchaseTime: now},
	}
	s.HistoryMu.Unlock()

	if n := RefreshOrderStatuses(s, true); n == 0 {
		t.Fatal("refresh must report updates")
	}
	s.HistoryMu.Lock()
	defer s.HistoryMu.Unlock()
	got := map[string]types.PurchaseHistoryEntry{}
	for _, h := range s.History {
		got[h.ID] = h
	}
	if got["flash"].ExpirationTime != short {
		t.Fatalf("15-minute deadline must come straight from OVH: %q", got["flash"].ExpirationTime)
	}
	if got["normal"].ExpirationTime != long || got["normal"].RetractionTime != long {
		t.Fatalf("15-day deadline must come straight from OVH: %+v", got["normal"])
	}
	if got["flash"].OrderStatus != "notPaid" {
		t.Fatalf("status refresh must still run: %q", got["flash"].OrderStatus)
	}
}
