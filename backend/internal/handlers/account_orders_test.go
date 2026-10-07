package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"
	"github.com/ovh-webui/server/internal/types"
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
				"orderId": 1001,
				"date":    "2026-01-01T12:00:00+00:00",
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
}
