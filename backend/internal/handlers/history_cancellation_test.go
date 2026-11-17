package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/types"
)

// 完全离线的订单 API，禁止访问真实 OVH 或触发真实资金操作。
func cancellationFixture(t *testing.T, region, status, expiry, retractUntil string, responseCode int) (*app.State, *gin.Engine, *atomic.Int32, *atomic.Value) {
	t.Helper()
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "cancel", Endpoint: "ovh-" + strings.ToLower(region), Zone: "IE", AppKey: "test", AppSecret: "test", ConsumerKey: "test", IsDefault: true}}
	st.History = []types.PurchaseHistoryEntry{{ID: "history-1001", AccountID: "cancel", OrderID: "1001", PlanCode: "【模拟】plan", Status: "success", OrderStatus: status}}
	client, err := st.OVH.ClientFor("cancel")
	if err != nil {
		t.Fatal(err)
	}
	calls, payload := &atomic.Int32{}, &atomic.Value{}
	client.Client = &http.Client{Transport: queuePreorderTransport(func(r *http.Request) (*http.Response, error) {
		body, code := "", 200
		switch {
		case strings.HasSuffix(r.URL.Path, "/auth/time"):
			body = jsonNumber(time.Now().Unix())
		case strings.HasSuffix(r.URL.Path, "/order/1001/status") && r.Method == "GET":
			b, _ := json.Marshal(status)
			body = string(b)
		case strings.HasSuffix(r.URL.Path, "/order/1001") && r.Method == "GET":
			b, _ := json.Marshal(map[string]interface{}{"orderId": 1001, "expirationDate": expiry, "retractionDate": retractUntil, "priceWithTax": map[string]interface{}{"value": 39.98, "currencyCode": "EUR"}})
			body = string(b)
		case strings.HasSuffix(r.URL.Path, "/order/1001/retraction") && r.Method == "POST":
			calls.Add(1)
			b, _ := io.ReadAll(r.Body)
			payload.Store(string(b))
			code = responseCode
			body = `{"message":"fixture response"}`
		default:
			t.Errorf("unexpected offline request: %s %s", r.Method, r.URL.Path)
			code = 500
			body = `{}`
		}
		return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	r := gin.New()
	r.GET("/history", GetPurchaseHistory(st))
	r.GET("/history/:id/cancellation", GetHistoryCancellation(st))
	r.POST("/history/:id/cancellation", CancelHistoryOrder(st))
	r.POST("/orders/:order_id/pay", PayAccountOrder(st))
	return st, r, calls, payload
}

const cancellationPath = "/history/history-1001/cancellation?account=cancel"
const retractConfirmation = `{"confirm":true,"mode":"retract","amount":39.98,"currency":"EUR","reason":"unused","comment":"取消【模拟】订单"}`

func TestHistoryAbandonAndRestore(t *testing.T) {
	st, r, calls, _ := cancellationFixture(t, "EU", "notPaid", "2099-01-01T00:00:00Z", "", 200)
	for _, mode := range []string{"abandon", "restore"} {
		w := payRequest(r, "POST", cancellationPath, `{"confirm":true,"mode":"`+mode+`"}`)
		if w.Code != 200 || calls.Load() != 0 {
			t.Fatalf("local %s: %d %s calls=%d", mode, w.Code, w.Body, calls.Load())
		}
		action, err := st.DB.GetOrderCancellation("cancel", 1001)
		if err != nil || (mode == "abandon") != (action.AbandonedAt != "") {
			t.Fatalf("persisted %s: %+v %v", mode, action, err)
		}
		if mode == "abandon" {
			w = payRequest(r, "GET", "/history", "")
			if !strings.Contains(w.Body.String(), `"abandonedAt"`) {
				t.Fatalf("history missing marker: %s", w.Body)
			}
			w = payRequest(r, "POST", "/orders/1001/pay?account=cancel", validPayment)
			if w.Code < 400 {
				t.Fatalf("abandoned order paid: %d %s", w.Code, w.Body)
			}
			claimed, err := st.ClaimOrderPayment("cancel", 1001)
			if err != nil || claimed {
				t.Fatalf("payment claim allowed: %v %v", claimed, err)
			}
		}
	}
	claimed, err := st.ClaimOrderPayment("cancel", 1001)
	if err != nil || !claimed {
		t.Fatalf("restored order blocked: %v %v", claimed, err)
	}
	// 已登记付款后，放弃必须被拒绝。
	w := payRequest(r, "POST", cancellationPath, `{"confirm":true,"mode":"abandon"}`)
	if w.Code != 409 {
		t.Fatalf("payment/abandon conflict: %d %s", w.Code, w.Body)
	}
}

func TestHistoryCancellationRejectsUnsafeRequests(t *testing.T) {
	for _, tc := range []struct{ name, region, status, expiry, deadline, body, path string }{
		{"unconfirmed", "EU", "delivering", "", "2099-01-01T00:00:00Z", strings.Replace(retractConfirmation, `"confirm":true`, `"confirm":false`, 1), cancellationPath},
		{"wrong account", "EU", "delivering", "", "2099-01-01T00:00:00Z", retractConfirmation, strings.Replace(cancellationPath, "account=cancel", "account=other", 1)},
		{"missing account", "EU", "delivering", "", "2099-01-01T00:00:00Z", retractConfirmation, "/history/history-1001/cancellation"},
		{"US", "US", "delivering", "", "2099-01-01T00:00:00Z", retractConfirmation, cancellationPath},
		{"no right", "EU", "delivered", "", "", retractConfirmation, cancellationPath},
		{"expired right", "EU", "delivering", "", "2000-01-01T00:00:00Z", retractConfirmation, cancellationPath},
		{"cancelled", "EU", "cancelled", "", "2099-01-01T00:00:00Z", retractConfirmation, cancellationPath},
		{"unknown", "EU", "unknown", "", "2099-01-01T00:00:00Z", retractConfirmation, cancellationPath},
		{"amount changed", "EU", "delivering", "", "2099-01-01T00:00:00Z", strings.Replace(retractConfirmation, "39.98", "40.00", 1), cancellationPath},
		{"currency changed", "EU", "delivering", "", "2099-01-01T00:00:00Z", strings.Replace(retractConfirmation, "EUR", "USD", 1), cancellationPath},
		{"invalid reason", "EU", "delivering", "", "2099-01-01T00:00:00Z", strings.Replace(retractConfirmation, "unused", "bogus", 1), cancellationPath},
		{"unpaid cannot retract", "EU", "notPaid", "2099-01-01T00:00:00Z", "2099-01-01T00:00:00Z", retractConfirmation, cancellationPath},
		{"expired unpaid", "EU", "notPaid", "2000-01-01T00:00:00Z", "", `{"confirm":true,"mode":"abandon"}`, cancellationPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, r, calls, _ := cancellationFixture(t, tc.region, tc.status, tc.expiry, tc.deadline, 200)
			w := payRequest(r, "POST", tc.path, tc.body)
			if w.Code < 400 || calls.Load() != 0 {
				t.Fatalf("unsafe request: %d %s calls=%d", w.Code, w.Body, calls.Load())
			}
		})
	}
}

func TestHistoryRetractionSingleSubmission(t *testing.T) {
	st, r, calls, payload := cancellationFixture(t, "CA", "delivering", "", "2099-01-01T00:00:00Z", 200)
	w := payRequest(r, "GET", cancellationPath, "")
	if w.Code != 200 || calls.Load() != 0 {
		t.Fatalf("preview writes: %d %s", w.Code, w.Body)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := payRequest(r, "POST", cancellationPath, retractConfirmation)
			if w.Code != 200 && w.Code != 409 {
				t.Errorf("concurrent request: %d %s", w.Code, w.Body)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("retraction sent %d times", calls.Load())
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(payload.Load().(string)), &body); err != nil || body["reason"] != "unused" || body["comment"] != "取消【模拟】订单" {
		t.Fatalf("payload: %v %v", body, err)
	}
	action, err := st.DB.GetOrderCancellation("cancel", 1001)
	if err != nil || action.RetractionState != "submitted" {
		t.Fatalf("journal: %+v %v", action, err)
	}
}

func TestHistoryRetractionAmbiguousNeverRetries(t *testing.T) {
	for _, code := range []int{408, 409, 429, 503} {
		t.Run(jsonNumber(int64(code)), func(t *testing.T) {
			st, r, calls, _ := cancellationFixture(t, "EU", "delivering", "", "2099-01-01T00:00:00Z", code)
			for i := 0; i < 2; i++ {
				w := payRequest(r, "POST", cancellationPath, retractConfirmation)
				if w.Code < 400 {
					t.Fatalf("unexpected success %d", w.Code)
				}
			}
			action, err := st.DB.GetOrderCancellation("cancel", 1001)
			if calls.Load() != 1 || err != nil || action.RetractionState != "uncertain" {
				t.Fatalf("unsafe retry: calls=%d %+v %v", calls.Load(), action, err)
			}
		})
	}
}

func TestHistoryRetractionDefiniteRejectionCanReconfirm(t *testing.T) {
	_, r, calls, _ := cancellationFixture(t, "EU", "delivering", "", "2099-01-01T00:00:00Z", 400)
	for i := 0; i < 2; i++ {
		w := payRequest(r, "POST", cancellationPath, retractConfirmation)
		if w.Code != 502 {
			t.Fatalf("rejection %d %s", w.Code, w.Body)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("fresh confirmation blocked: %d", calls.Load())
	}
}

func TestHistoryUnfinishedRetractionBlocksAfterReload(t *testing.T) {
	st, r, calls, _ := cancellationFixture(t, "EU", "delivering", "", "2099-01-01T00:00:00Z", 200)
	if err := st.DB.ChangeOrderCancellation("cancel", 1001, "retract"); err != nil {
		t.Fatal(err)
	}
	// 新 handler 只读数据库登记，不依赖上一请求的内存状态。
	r.POST("/reloaded/:id/cancellation", CancelHistoryOrder(st))
	w := payRequest(r, "POST", "/reloaded/history-1001/cancellation?account=cancel", retractConfirmation)
	if w.Code != 409 || calls.Load() != 0 {
		t.Fatalf("sending retried %d %s", w.Code, w.Body)
	}
	var action db.OrderCancellation
	if ok, err := st.GetKV(db.OrderCancellationKey("cancel", 1001), &action); err != nil || !ok || action.RetractionState != "sending" {
		t.Fatalf("journal lost %+v %v", action, err)
	}
}
