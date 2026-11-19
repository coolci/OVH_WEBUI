package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/types"
)

func TestOrderMapping_NoServers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	st.Accounts = []types.OVHAccount{
		{
			ID:          "acc-empty",
			Name:        "Empty Account",
			Endpoint:    "ovh-eu",
			AppKey:      "test",
			AppSecret:   "test",
			ConsumerKey: "test",
			IsDefault:   true,
		},
	}

	client, err := st.OVH.ClientFor("acc-empty")
	if err != nil {
		t.Fatal(err)
	}

	orderCalls := int32(0)
	client.Client = &http.Client{
		Transport: queuePreorderTransport(func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			if strings.HasSuffix(path, "/auth/time") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader(strconv.FormatInt(time.Now().Unix(), 10))),
					Request:    req,
				}, nil
			}
			if path == "/1.0/dedicated/server" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`[]`)),
					Request:    req,
				}, nil
			}
			if strings.Contains(path, "/me/order") {
				atomic.AddInt32(&orderCalls, 1)
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
				Request:    req,
			}, nil
		}),
	}

	router := gin.New()
	router.GET("/api/server-control/order-mapping", GetOrderMapping(st))

	req := httptest.NewRequest(http.MethodGet, "/api/server-control/order-mapping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if total, _ := resp["total"].(float64); total != 0 {
		t.Fatalf("expected total=0, got %v", total)
	}
	if atomic.LoadInt32(&orderCalls) > 0 {
		t.Fatalf("无服务器时不应调用任何 /me/order 接口")
	}
}

func TestOrderMapping_EarlyStopping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	accID := "acc-early"
	st.Accounts = []types.OVHAccount{
		{
			ID:          accID,
			Name:        "Early Stop Account",
			Endpoint:    "ovh-eu",
			AppKey:      "test",
			AppSecret:   "test",
			ConsumerKey: "test",
			IsDefault:   true,
		},
	}

	// 保证缓存是干净的
	invalidateOrderMappingCache(accID)

	client, err := st.OVH.ClientFor(accID)
	if err != nil {
		t.Fatal(err)
	}

	var callMu sync.Mutex
	orderStatusCalls := make(map[int64]int)
	client.Client = &http.Client{
		Transport: queuePreorderTransport(func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			if strings.HasSuffix(path, "/auth/time") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader(strconv.FormatInt(time.Now().Unix(), 10))),
					Request:    req,
				}, nil
			}

			// 只有一台服务器
			if path == "/1.0/dedicated/server" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`["ns-early.ip"]`)),
					Request:    req,
				}, nil
			}

			// serviceInfos
			if path == "/1.0/dedicated/server/ns-early.ip/serviceInfos" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"creation":"2026-01-01T00:00:00Z"}`)),
					Request:    req,
				}, nil
			}

			// /me/order 返回 20 个订单: 1 到 20
			if path == "/1.0/me/order" {
				ids := make([]int64, 20)
				for i := 0; i < 20; i++ {
					ids[i] = int64(i + 1)
				}
				b, _ := json.Marshal(ids)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(string(b))),
					Request:    req,
				}, nil
			}

			// /me/order/{id}/status
			if strings.Contains(path, "/status") {
				parts := strings.Split(path, "/")
				oid, _ := strconv.ParseInt(parts[3], 10, 64)
				callMu.Lock()
				orderStatusCalls[oid]++
				callMu.Unlock()
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`"delivered"`)),
					Request:    req,
				}, nil
			}

			// /me/order/{id}
			if strings.HasPrefix(path, "/1.0/me/order/") && !strings.Contains(path, "details") && !strings.Contains(path, "status") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"date":"2026-01-01T00:00:00Z"}`)),
					Request:    req,
				}, nil
			}

			// /me/order/{id}/details (list)
			if strings.HasSuffix(path, "/details") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`[101]`)),
					Request:    req,
				}, nil
			}

			// /me/order/{id}/details/101
			if strings.HasSuffix(path, "/details/101") {
				// 订单 20 就包含了 ns-early.ip
				domain := "other.com"
				desc := "web domain"
				if strings.Contains(path, "/order/20/") {
					domain = "ns-early.ip"
					desc = "dedicated server"
				}
				body := fmt.Sprintf(`{"domain":%q,"description":%q,"cancelled":false,"totalPrice":{"value":10.5,"currencyCode":"EUR"}}`, domain, desc)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
				Request:    req,
			}, nil
		}),
	}

	router := gin.New()
	router.GET("/api/server-control/order-mapping", GetOrderMapping(st))

	req := httptest.NewRequest(http.MethodGet, "/api/server-control/order-mapping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool                              `json:"success"`
		Mapping map[string]map[string]interface{} `json:"mapping"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true")
	}
	t.Logf("Response body: %s", w.Body.String())
	info, exists := resp.Mapping["ns-early.ip"]
	if !exists {
		t.Fatalf("expected mapping for ns-early.ip")
	}
	if orderID, _ := info["orderId"].(float64); int64(orderID) != 20 {
		t.Fatalf("expected orderId=20, got %v", info["orderId"])
	}

	// 关键验证：由于订单 20 在第一批次（20..11）中就已完成 ns-early.ip 的匹配，
	// 早停机制生效，第二批次（10..1）的订单状态绝不应该被查询！
	callMu.Lock()
	defer callMu.Unlock()
	for i := 1; i <= 10; i++ {
		if count := orderStatusCalls[int64(i)]; count > 0 {
			t.Fatalf("早停机制未生效: 订单 %d 不应被查询状态 (count=%d)", i, count)
		}
	}

	// 再次发起请求，应直接命中缓存
	{
		req := httptest.NewRequest(http.MethodGet, "/api/server-control/order-mapping", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var cacheResp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &cacheResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if cached, _ := cacheResp["cached"].(bool); !cached {
			t.Fatalf("第二次请求应命中缓存 cached=true")
		}
	}
}
