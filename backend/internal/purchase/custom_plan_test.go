package purchase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
)

func TestCustomQuickOrderUsesLiveValidationWithoutCatalogMatch(t *testing.T) {
	for _, accepted := range []bool{true, false} {
		t.Run(fmt.Sprintf("accepted=%v", accepted), func(t *testing.T) {
			st := quickOrderTestState(t)
			st.Accounts = []types.OVHAccount{{ID: "custom-quote", Endpoint: "ovh-eu", Zone: "FI", AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}
			const planCode = "custom-NewModel.LZ"
			var quoted atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/auth/time":
					fmt.Fprint(w, time.Now().Unix())
				case "/order/cart":
					io.WriteString(w, `{"cartId":"custom-cart"}`)
				case "/order/cart/custom-cart/eco":
					if r.Method == "GET" {
						io.WriteString(w, `[]`)
						return
					}
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					if payload["planCode"] != planCode {
						t.Errorf("custom code changed: %+v", payload)
					}
					quoted.Add(1)
					if !accepted {
						w.WriteHeader(400)
						io.WriteString(w, `{"message":"Plan code not found"}`)
						return
					}
					io.WriteString(w, `{"itemId":1}`)
				case "/order/cart/custom-cart/item/1/configuration", "/order/cart/custom-cart/assign", "/order/cart/custom-cart":
					io.WriteString(w, `null`)
				case "/order/cart/custom-cart/summary":
					io.WriteString(w, `{"prices":{"withoutTax":{"value":12,"currencyCode":"EUR"},"withTax":{"value":15,"currencyCode":"EUR"}}}`)
				default:
					t.Errorf("unexpected mock request (checkout forbidden): %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer srv.Close()
			swapEndpoint(t, "ovh-eu", srv.URL)
			originalTransport := http.DefaultTransport
			http.DefaultTransport = auditTransport(func(r *http.Request) (*http.Response, error) {
				if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"plans":[]}`)), Request: r}, nil
				}
				return nil, fmt.Errorf("external request blocked by custom-plan test: %s", r.URL)
			})
			defer netfp.SwapSharedTransportForTesting(http.DefaultTransport)()
			defer func() { http.DefaultTransport = originalTransport }()
			result := EnqueueQuickOrder(st, QuickOrderParams{AccountID: "custom-quote", PlanCode: " " + planCode + " ", Datacenter: " gra ", Force: true})
			if result.Success != accepted || quoted.Load() != 1 {
				t.Fatalf("live validation result=%+v, quoted=%d", result, quoted.Load())
			}
			queue := st.QueueSnapshot()
			if accepted {
				if len(queue) != 1 || queue[0].PlanCode != planCode || !queue[0].Force || queue[0].AutoPay {
					t.Fatalf("custom task changed: %+v", queue)
				}
			} else if len(queue) != 0 {
				t.Fatal("OVH rejected the plan but a task was still queued")
			}
		})
	}
}
