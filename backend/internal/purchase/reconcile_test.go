package purchase

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"
)

func swapEndpoint(t *testing.T, name, rawURL string) {
	t.Helper()
	orig := ovhsdk.Endpoints[name]
	ovhsdk.Endpoints[name] = rawURL
	t.Cleanup(func() { ovhsdk.Endpoints[name] = orig })
}

func TestReconcileRecentOrderOfficialShape(t *testing.T) {
	plan := "24sk202"
	now := time.Now().UTC().Truncate(time.Second)
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.RawQuery != "" {
			queries = append(queries, r.URL.RawQuery)
		}
		switch r.URL.Path {
		case "/auth/time":
			_, _ = w.Write([]byte("1700000000"))
		case "/me/order":
			if r.URL.Query().Get("dateFrom") != "" || r.URL.Query().Get("planCode") != "" {
				t.Errorf("非法查询参数: %s", r.URL.RawQuery)
			}
			if r.URL.Query().Get("date.from") == "" {
				t.Errorf("EU 对账必须带 date.from，实际 %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode([]int64{100, 200})
		case "/me/order/200":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"orderId": 200,
				"date":    now.Format(time.RFC3339),
				"url":     "https://www.ovh.com/cgi-bin/order/displayOrder.cgi?orderId=200&password=secret",
			})
		case "/me/order/200/details":
			_ = json.NewEncoder(w).Encode([]int64{7})
		case "/me/order/200/details/7":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"description": "RISE-1 | " + plan,
				"domain":      "ns123.ip-1-2-3.eu",
			})
		case "/me/order/100":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"orderId": 100,
				"date":    now.Add(-48 * time.Hour).Format(time.RFC3339),
				"url":     "https://www.ovh.com/old",
			})
		case "/me/order/100/details":
			_ = json.NewEncoder(w).Encode([]int64{1})
		case "/me/order/100/details/1":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"description": "old " + plan,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	swapEndpoint(t, "ovh-eu", srv.URL)

	client, err := ovhsdk.NewClient("ovh-eu", "ak", "as", "ck")
	if err != nil {
		t.Fatal(err)
	}
	id, link, err := ReconcileRecentOrder(client, "ovh-eu", plan, now.Add(-30*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if id != "200" {
		t.Fatalf("orderID = %q, 期望 200（应忽略 48 小时前的同型号订单）", id)
	}
	if !strings.Contains(link, "orderId=200") {
		t.Fatalf("应保留订单自带付款链接, got %q", link)
	}
	for _, q := range queries {
		if strings.Contains(q, "dateFrom=") || strings.Contains(q, "planCode=") {
			t.Fatalf("查询串仍是旧参数: %s", q)
		}
	}
}

func TestReconcileRecentOrderUSHasNoQuery(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			_, _ = w.Write([]byte("1700000000"))
		case "/me/order":
			if r.URL.RawQuery != "" {
				t.Errorf("US /me/order 不接受查询参数, got %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode([]int64{9})
		case "/me/order/9":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"orderId": 9,
				"date":    now.Format(time.RFC3339),
			})
		case "/me/order/9/details":
			_ = json.NewEncoder(w).Encode([]int64{1})
		case "/me/order/9/details/1":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"description": "VPS vps2025-starter",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	swapEndpoint(t, "ovh-us", srv.URL)

	client, err := ovhsdk.NewClient("ovh-us", "ak", "as", "ck")
	if err != nil {
		t.Fatal(err)
	}
	id, link, err := ReconcileRecentOrder(client, "ovh-us", "vps2025-starter", now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if id != "9" {
		t.Fatalf("orderID = %q", id)
	}
	if !strings.Contains(link, "manager.us.ovhcloud.com") || !strings.Contains(link, "orderId=9") {
		t.Fatalf("url 缺失时应回退美区控制台深链, got %q", link)
	}
}

func TestReconcileRecentOrderNoMatch(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			_, _ = w.Write([]byte("1700000000"))
		case "/me/order":
			_ = json.NewEncoder(w).Encode([]int64{3})
		case "/me/order/3":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"orderId": 3,
				"date":    now.Format(time.RFC3339),
			})
		case "/me/order/3/details":
			_ = json.NewEncoder(w).Encode([]int64{1})
		case "/me/order/3/details/1":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"description": "some other product",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	swapEndpoint(t, "ovh-eu", srv.URL)
	client, err := ovhsdk.NewClient("ovh-eu", "ak", "as", "ck")
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := ReconcileRecentOrder(client, "ovh-eu", "24sk202", now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		t.Fatalf("不同型号不应被当成这轮的订单, got %q", id)
	}
}
