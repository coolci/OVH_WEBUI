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
			nowISO := time.Now().UTC().Format(time.RFC3339)
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
					t.Fatalf("must never call simulation endpoint GET %s", r.URL.Path)
				case strings.HasSuffix(r.URL.Path, "/me/order"):
					if scenario == "simulation" || scenario == "missing-cart" {
						body = `[]`
					} else {
						body = `[123]`
					}
				case strings.HasSuffix(r.URL.Path, "/me/order/123"):
					body = fmt.Sprintf(`{"orderId":123,"date":%q,"url":"https://www.ovh.com/order/123"}`, nowISO)
					if scenario == "saved-order" {
						body = `{"orderId":123}`
					}
				case strings.HasSuffix(r.URL.Path, "/me/order/123/details"):
					body = `[1]`
				case strings.HasSuffix(r.URL.Path, "/me/order/123/details/1"):
					body = `{"orderDetailId":1,"description":"Server (24sk40-v1) - gra"}`
					if scenario == "wrong-order" {
						body = `{"orderDetailId":1,"description":"Other server (24other99) - rbx"}`
					}
				default:
					t.Fatalf("unexpected query %s", r.URL.Path)
				}
				return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			journal := &purchase.CheckoutJournal{
				Submitted:  true,
				CartID:     "exact",
				PlanCode:   "24sk40-v1",
				Datacenter: "gra",
				StartedAt:  time.Now().UTC().Format(time.RFC3339Nano),
			}
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
