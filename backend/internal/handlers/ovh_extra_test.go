package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/types"
)

func fakeOVHServer(t *testing.T, routes map[string]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		key := r.Method + " " + r.URL.Path
		if r.URL.RawQuery != "" {
			key += "?" + r.URL.RawQuery
		}
		body, ok := routes[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"message":"unexpected %s"}`, key)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	t.Cleanup(func() { ovhsdk.Endpoints["ovh-eu"] = orig })
}

func TestDefaultPaymentCheck(t *testing.T) {
	future := time.Now().AddDate(1, 0, 0).Format(time.RFC3339)
	past := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339)
	for _, tc := range []struct {
		name   string
		routes map[string]string
		want   DefaultPaymentInfo
	}{
		{"no default", map[string]string{"GET /me/payment/method?default=true": `[]`}, DefaultPaymentInfo{HasDefault: false}},
		{"valid card", map[string]string{"GET /me/payment/method?default=true": `[7]`,
			"GET /me/payment/method/7": fmt.Sprintf(`{"paymentMethodId":7,"paymentType":"CREDIT_CARD","label":"**** 1234","status":"VALID","default":true,"expirationDate":%q}`, future)},
			DefaultPaymentInfo{HasDefault: true, Valid: true, Type: "CREDIT_CARD", Label: "**** 1234"}},
		{"expired card", map[string]string{"GET /me/payment/method?default=true": `[8]`,
			"GET /me/payment/method/8": fmt.Sprintf(`{"paymentMethodId":8,"paymentType":"CREDIT_CARD","label":"**** 9","status":"VALID","default":true,"expirationDate":%q}`, past)},
			DefaultPaymentInfo{HasDefault: true, Valid: false}},
		{"blocked paypal", map[string]string{"GET /me/payment/method?default=true": `[9]`,
			"GET /me/payment/method/9": `{"paymentMethodId":9,"paymentType":"PAYPAL","label":"a@b","status":"BLOCKED","default":true}`},
			DefaultPaymentInfo{HasDefault: true, Valid: false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeOVHServer(t, tc.routes)
			st := newTestState(t)
			st.Accounts = []types.OVHAccount{{ID: "acc-" + strings.ReplaceAll(tc.name, " ", "-"), Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}
			r := gin.New()
			r.GET("/p", GetDefaultPayment(st))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/p?account_id=acc-"+strings.ReplaceAll(tc.name, " ", "-"), nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status %d %s", w.Code, w.Body.String())
			}
			var got DefaultPaymentInfo
			_ = json.Unmarshal(w.Body.Bytes(), &got)
			if got.HasDefault != tc.want.HasDefault || got.Valid != tc.want.Valid || (tc.want.Label != "" && got.Label != tc.want.Label) {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
			if !got.Valid && got.Reason == "" {
				t.Fatal("an unusable payment method must explain why")
			}
		})
	}
}

// 查可选购买时长走免登录购物车:请求不带签名头,也不尝试删车(匿名车删不掉)
func TestOrderDurationsUseAnonymousCart(t *testing.T) {
	var signed, deletes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		if r.Header.Get("X-Ovh-Signature") != "" || r.Header.Get("X-Ovh-Consumer") != "" {
			signed++
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/order/cart":
			fmt.Fprint(w, `{"cartId":"anon"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/order/cart/anon/eco":
			fmt.Fprint(w, `[{"planCode":"24sk202","prices":[{"duration":"P1M","pricingMode":"default"},{"duration":"P1Y","pricingMode":"upfront12"},{"duration":"P0D","pricingMode":"default"}]}]`)
		case r.Method == http.MethodDelete:
			deletes++
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"You must login first"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	t.Cleanup(func() { ovhsdk.Endpoints["ovh-eu"] = orig })
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "acc", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}
	got, err := fetchOrderDurations(t.Context(), st, "acc", "IE", "24sk202")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "P1M,P12M" {
		t.Fatalf("periods (P0D install entries dropped, P1Y normalized): %v", got)
	}
	if signed != 0 || deletes != 0 {
		t.Fatalf("anonymous lookup must not sign requests or try to delete: signed=%d deletes=%d", signed, deletes)
	}
}

func TestOrderFollowUpReturnsStepsAndDeliveredService(t *testing.T) {
	fakeOVHServer(t, map[string]string{
		"GET /me/order/4242/followUp": `[{"step":"VALIDATING","status":"DONE","history":[{"date":"2026-11-08T10:00:00+01:00","label":"PAYMENT_RECEIVED","description":"paid"}]},
		                                 {"step":"DELIVERING","status":"DOING","history":[]}]`,
		"GET /me/order/4242/details":   `[1,2]`,
		"GET /me/order/4242/details/1": `{"orderDetailId":1,"domain":"ns3141.ip-1-2-3.eu"}`,
		"GET /me/order/4242/details/2": `{"orderDetailId":2,"domain":"*"}`,
	})
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "acc", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}
	r := gin.New()
	r.GET("/o/:orderId", GetOrderFollowUp(st))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/o/4242?account_id=acc", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Steps    []OrderFollowUpStep `json:"steps"`
		Services []string            `json:"services"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if len(got.Steps) != 2 || got.Steps[1].Step != "DELIVERING" || got.Steps[0].History[0].Label != "PAYMENT_RECEIVED" {
		t.Fatalf("steps: %+v", got.Steps)
	}
	if len(got.Services) != 1 || got.Services[0] != "ns3141.ip-1-2-3.eu" {
		t.Fatalf("delivered service name must come from order details (\"*\" ignored): %v", got.Services)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/o/12a?account_id=acc", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("non-numeric order id must be rejected: %d", w.Code)
	}
}
