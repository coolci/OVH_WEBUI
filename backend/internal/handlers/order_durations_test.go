package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/types"
)

func TestGetOrderDurationsCachesAndValidates(t *testing.T) {
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "acc", Endpoint: "ovh-eu", Zone: "FI"}}
	var calls atomic.Int32
	fail := false
	orig := orderDurationsFetcher
	orderDurationsFetcher = func(_ context.Context, _ *app.State, accountID, subsidiary, plan string) ([]string, error) {
		calls.Add(1)
		if fail {
			return nil, errors.New("ovh down")
		}
		if accountID != "acc" || subsidiary == "" {
			t.Errorf("unexpected fetch args: %s %s", accountID, subsidiary)
		}
		return []string{"P1M", "P12M"}, nil
	}
	orderDurationsCache.Lock()
	orderDurationsCache.m = map[string]durationsEntry{}
	orderDurationsCache.Unlock()
	t.Cleanup(func() { orderDurationsFetcher = orig })

	r := gin.New()
	r.GET("/order/durations", GetOrderDurations(st))
	do := func(q string) (*httptest.ResponseRecorder, map[string]interface{}) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/order/durations?"+q, nil))
		body := map[string]interface{}{}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w, body
	}
	for _, q := range []string{"planCode=p", "account_id=missing&planCode=p", "account_id=acc"} {
		if w, _ := do(q); w.Code != http.StatusBadRequest {
			t.Fatalf("%q: want 400, got %d", q, w.Code)
		}
	}
	w, body := do("account_id=acc&planCode=p")
	if w.Code != http.StatusOK || body["default"] != "P1M" {
		t.Fatalf("got %d %v", w.Code, body)
	}
	if ds, _ := body["durations"].([]interface{}); len(ds) != 2 || ds[1] != "P12M" {
		t.Fatalf("durations: %v", body["durations"])
	}
	do("account_id=acc&planCode=p")
	if calls.Load() != 1 {
		t.Fatalf("second lookup must hit the cache (each miss creates an OVH cart), fetched %d times", calls.Load())
	}
	fail = true
	if w, _ := do("account_id=acc&planCode=other"); w.Code != http.StatusBadGateway {
		t.Fatalf("fetch failure must be 502, got %d", w.Code)
	}
}

func TestQueueHandlersValidateDuration(t *testing.T) {
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "acc", Endpoint: "ovh-eu", Zone: "FI"}}
	r := gin.New()
	r.POST("/queue", AddQueueItem(st))
	r.PUT("/queue/:id", UpdateQueueItem(st))
	post := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	if w := post(http.MethodPost, "/queue", `{"account_id":"acc","planCode":"p","datacenter":"gra","duration":"P0M"}`); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "购买时长") {
		t.Fatalf("invalid duration on add: %d %s", w.Code, w.Body.String())
	}
	if err := st.EnqueueItems([]types.QueueItem{{ID: "t1", AccountID: "acc", PlanCode: "p", Datacenter: "gra", Status: "paused"}}, false); err != nil {
		t.Fatal(err)
	}
	if w := post(http.MethodPut, "/queue/t1", `{"datacenter":"gra","retryInterval":30,"duration":"x"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid duration on edit: %d %s", w.Code, w.Body.String())
	}
	if w := post(http.MethodPut, "/queue/t1", `{"datacenter":"gra","retryInterval":30,"duration":"P1Y"}`); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body.String())
	}
	st.QueueMu.Lock()
	got := st.Queue[0].Duration
	st.QueueMu.Unlock()
	if got != "P12M" {
		t.Fatalf("edited duration: %s", got)
	}
}

// 网页添加/编辑订阅时设的购买时长要存进订阅、存进库;TG 等不传时长的入口不能把它冲掉。
func TestSubscriptionDurationRoundTrip(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	// 全部网络请求都在本地应答;添加订阅会启动监控循环,不能放它出网
	originalTransport := http.DefaultTransport
	http.DefaultTransport = queuePreorderTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "127.0.0.1" {
			return originalTransport.RoundTrip(r)
		}
		body := `[]`
		if strings.Contains(r.URL.Path, "/catalog/") {
			body = `{"plans":[]}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	defer netfp.SwapSharedTransportForTesting(http.DefaultTransport)()
	defer func() { http.DefaultTransport = originalTransport }()
	notify.AllowLocalWebhookForTesting = true
	defer func() { notify.AllowLocalWebhookForTesting = false }()
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"code":0}`) }))
	defer webhook.Close()
	st.Config.Set(types.Config{NotifyWebhookURL: webhook.URL})
	defer mon.Stop()
	r := gin.New()
	r.POST("/monitor/subscriptions", AddSubscription(st, mon))
	r.PUT("/monitor/subscriptions/:planCode", UpdateSubscription(st, mon))
	send := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	if w := send(http.MethodPost, "/monitor/subscriptions", `{"planCode":"dur-model","duration":"bad"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid duration: %d %s", w.Code, w.Body.String())
	}
	if w := send(http.MethodPost, "/monitor/subscriptions", `{"planCode":"dur-model","duration":"P12M"}`); w.Code != http.StatusOK {
		t.Fatalf("add: %d %s", w.Code, w.Body.String())
	}
	if cfg := mon.SubscriptionConfig("dur-model"); cfg.Duration != "P12M" {
		t.Fatalf("added duration: %q", cfg.Duration)
	}
	// TG /watch 走 AddSubscription 但不带时长:就地更新时保持原值
	mon.AddSubscription("dur-model", nil, true, false, "", nil, nil, false, 0, "", false, nil)
	if cfg := mon.SubscriptionConfig("dur-model"); cfg.Duration != "P12M" {
		t.Fatalf("a caller without duration reset it to %q", cfg.Duration)
	}
	if w := send(http.MethodPut, "/monitor/subscriptions/dur-model", `{"duration":"P24M"}`); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body.String())
	}
	reloaded := monitor.New(st)
	reloaded.LoadFromDB()
	if cfg := reloaded.SubscriptionConfig("dur-model"); cfg.Duration != "P24M" {
		t.Fatalf("persisted duration after reload: %q", cfg.Duration)
	}
}
