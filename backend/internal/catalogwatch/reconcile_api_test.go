package catalogwatch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/purchase"
)

func TestExactCartReconciliationDoesNotInferAbsence(t *testing.T) {
	for _, scenario := range []string{"unvalidated", "missing-cart", "rate-limit", "simulation", "wrong-order", "confirmed", "saved-order"} {
		t.Run(scenario, func(t *testing.T) {
			_, s := fixture(t)
			s.Accounts[0].AppKey, s.Accounts[0].AppSecret, s.Accounts[0].ConsumerKey = "test", "test", "test"
			client, err := s.OVH.ClientFor("a")
			if err != nil {
				t.Fatal(err)
			}
			client.Client.Transport = notificationTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" {
					t.Fatalf("reconciliation mutated OVH: %s", r.Method)
				}
				body, code := `{}`, 200
				switch {
				case strings.HasSuffix(r.URL.Path, "/auth/time"):
					body = fmt.Sprint(time.Now().Unix())
				case strings.HasSuffix(r.URL.Path, "/order/cart/exact"):
					body = `{"readOnly":true}`
					if scenario == "unvalidated" {
						body = `{"readOnly":false}`
					}
					if scenario == "missing-cart" {
						code = 404
						body = `{"message":"not found"}`
					}
					if scenario == "rate-limit" {
						code = 429
						body = `{"message":"limited"}`
					}
				case strings.HasSuffix(r.URL.Path, "/exact/checkout"):
					body = `{"orderId":123}`
					if scenario == "simulation" {
						body = `{"orderId":0}`
					}
				case strings.HasSuffix(r.URL.Path, "/me/order/123"):
					body = `{"orderId":123}`
					if scenario == "wrong-order" {
						body = `{"orderId":124}`
					}
				default:
					t.Fatalf("unexpected query %s", r.URL.Path)
				}
				return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			journal := &purchase.CheckoutJournal{Submitted: true, CartID: "exact"}
			if scenario == "saved-order" {
				journal.OrderID = "123"
			}
			result, err := purchase.CheckCheckout(context.Background(), s, "a", journal)
			if result.NotSubmitted {
				t.Fatal("upstream absence must not permit another checkout")
			}
			if scenario == "confirmed" || scenario == "saved-order" {
				if err != nil || result.OrderID != "123" {
					t.Fatalf("%+v %v", result, err)
				}
			} else if err == nil || result.OrderID != "" {
				t.Fatalf("unconfirmed order accepted: %+v", result)
			}
		})
	}
}
