package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/types"
	ovhsdk "github.com/ovh/go-ovh/ovh"
)

func TestGetAccountOrders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestState(t)

	mockOVH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("1700000000"))
		case "/me/order":
			_ = json.NewEncoder(w).Encode([]int64{1001, 1002})
		case "/me/order/1001":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"orderId":  1001,
				"date":     "2026-01-01T12:00:00+00:00",
				"password": "order-password-must-not-leak",
				"priceWithTax": map[string]interface{}{
					"value":        19.99,
					"currencyCode": "EUR",
					"text":         "19.99 €",
				},
				"url": "https://www.ovh.com/order/1001",
			})
		case "/me/order/1002":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"orderId": 1002,
				"date":    "2026-01-02T12:00:00+00:00",
				"priceWithTax": map[string]interface{}{
					"value":        29.99,
					"currencyCode": "EUR",
					"text":         "29.99 €",
				},
				"url": "",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockOVH.Close()

	origEndpoint := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = mockOVH.URL
	defer func() {
		ovhsdk.Endpoints["ovh-eu"] = origEndpoint
	}()

	acc := types.OVHAccount{
		ID:          "acc-test-orders",
		Name:        "Test Orders Account",
		AppKey:      "ak",
		AppSecret:   "as",
		ConsumerKey: "ck",
		Endpoint:    "ovh-eu",
		CredState:   "valid",
	}
	if err := st.DB.UpsertAccount(acc); err != nil {
		t.Fatalf("UpsertAccount: %v", err)
	}
	if err := st.ReloadAccounts(); err != nil {
		t.Fatalf("ReloadAccounts: %v", err)
	}

	r := gin.New()
	r.GET("/api/ovh/account/orders", GetAccountOrders(st))

	req := httptest.NewRequest(http.MethodGet, "/api/ovh/account/orders?account=acc-test-orders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status string                   `json:"status"`
		Data   []map[string]interface{} `json:"data"`
		Orders []map[string]interface{} `json:"orders"`
		Total  int                      `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Fatalf("expected status success, got %s", resp.Status)
	}
	if len(resp.Data) != 2 || len(resp.Orders) != 2 {
		t.Fatalf("expected 2 orders in data and orders, got data=%d orders=%d", len(resp.Data), len(resp.Orders))
	}
	// Verify order 1002 had its url populated via managerOrderURL
	found1002 := false
	for _, o := range resp.Data {
		if o["orderId"] == float64(1002) {
			found1002 = true
			u, _ := o["url"].(string)
			if u == "" {
				t.Fatalf("expected order 1002 to have filled url, got empty")
			}
		}
	}
	if !found1002 {
		t.Fatalf("order 1002 not found in response")
	}
	if strings.Contains(w.Body.String(), "order-password-must-not-leak") {
		t.Fatal("billing.Order.password 被透传给了前端")
	}
}

func TestGetAccountOrdersPrefersNewestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestState(t)
	mockOVH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			_, _ = w.Write([]byte("1700000000"))
		case "/me/order":
			// 升序：截断前不排序的话，limit=1 会拉到最老的 11。
			_ = json.NewEncoder(w).Encode([]int64{11, 99})
		case "/me/order/99":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"orderId": 99,
				"date":    "2026-03-01T00:00:00+00:00",
				"url":     "https://www.ovh.com/order/99",
			})
		case "/me/order/11":
			t.Errorf("limit=1 仍去拉了较旧的订单 11")
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockOVH.Close()

	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = mockOVH.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = orig }()

	acc := types.OVHAccount{
		ID: "acc-newest", Name: "Newest", Endpoint: "ovh-eu",
		AppKey: "ak", AppSecret: "as", ConsumerKey: "ck",
	}
	if err := st.DB.UpsertAccount(acc); err != nil {
		t.Fatal(err)
	}
	if err := st.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/api/ovh/account/orders", GetAccountOrders(st))
	req := httptest.NewRequest(http.MethodGet, "/api/ovh/account/orders?account=acc-newest&limit=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"orderId":99`) && !strings.Contains(w.Body.String(), `"orderId": 99`) {
		t.Fatalf("应返回最新订单 99, body=%s", w.Body.String())
	}
}

func TestUnknownAccountDoesNotFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestState(t)
	acc := types.OVHAccount{
		ID: "real-account", Name: "Real", Endpoint: "ovh-eu",
		AppKey: "ak", AppSecret: "as", ConsumerKey: "ck",
	}
	if err := st.DB.UpsertAccount(acc); err != nil {
		t.Fatal(err)
	}
	if err := st.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/x?account=deleted-id", nil)
	if _, err := ovhClientFor(st, c); err == nil {
		t.Fatal("不存在的 account 被静默改打到了别的账户")
	}
	if got, ok := ovhAccountFor(st, c); ok {
		t.Fatalf("ovhAccountFor 回退到了 %s", got.ID)
	}
}
