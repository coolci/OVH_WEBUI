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
