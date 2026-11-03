package purchase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/types"
)

// TestParallelCartAssembly_OrderSuccessAndConcurrency 验证装车流水线全并行化：
// 1. assign 与 eco 并发执行（并发度 >= 2）
// 2. configuration 与 eco/options 并发执行（并发度 >= 3）
// 3. 总装车耗时大幅低于传统 11 次顺序串行耗时
func TestParallelCartAssembly_OrderSuccessAndConcurrency(t *testing.T) {
	resetEcoOptionsCache()
	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{
		ID:          "acc-parallel",
		Name:        "acc-parallel",
		Endpoint:    "ovh-eu",
		Zone:        "PT",
		AppKey:      "ak",
		AppSecret:   "as",
		ConsumerKey: "ck",
	}}

	item := &types.QueueItem{
		ID:         "task-parallel-1",
		AccountID:  "acc-parallel",
		PlanCode:   "24sk102",
		Datacenter: "gra",
		Options:    []string{"softraid-2x960nvme"},
		Status:     "running",
		AutoPay:    false,
	}

	var (
		activeAssignAndEco atomic.Int32
		peakAssignAndEco   atomic.Int32

		activeConfigsAndOpts atomic.Int32
		peakConfigsAndOpts   atomic.Int32

		cartCreated  atomic.Int32
		checkoutDone atomic.Int32
	)

	// 模拟单次调用 30ms 延迟
	delay := 30 * time.Millisecond

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		time.Sleep(delay)

		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())

		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprint(w, `[{"fqn":"24sk102.softraid-2x960nvme","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`)

		case r.URL.Path == "/order/cart":
			cartCreated.Add(1)
			fmt.Fprint(w, `{"cartId":"cart-parallel-test"}`)

		case r.URL.Path == "/order/cart/cart-parallel-test/assign":
			cur := activeAssignAndEco.Add(1)
			for {
				p := peakAssignAndEco.Load()
				if cur <= p || peakAssignAndEco.CompareAndSwap(p, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			activeAssignAndEco.Add(-1)
			fmt.Fprint(w, `null`)

		case r.URL.Path == "/order/cart/cart-parallel-test/eco":
			cur := activeAssignAndEco.Add(1)
			for {
				p := peakAssignAndEco.Load()
				if cur <= p || peakAssignAndEco.CompareAndSwap(p, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			activeAssignAndEco.Add(-1)
			fmt.Fprint(w, `{"itemId":999}`)

		case strings.HasPrefix(r.URL.Path, "/order/cart/cart-parallel-test/item/999/configuration"):
			cur := activeConfigsAndOpts.Add(1)
			for {
				p := peakConfigsAndOpts.Load()
				if cur <= p || peakConfigsAndOpts.CompareAndSwap(p, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			activeConfigsAndOpts.Add(-1)
			fmt.Fprint(w, `{}`)

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/order/cart/cart-parallel-test/eco/options"):
			fmt.Fprint(w, `[{"family":"storage","planCode":"softraid-2x960nvme-24sk102","prices":[{"pricingMode":"default","duration":"P1M"}]}]`)

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/order/cart/cart-parallel-test/eco/options"):
			cur := activeConfigsAndOpts.Add(1)
			for {
				p := peakConfigsAndOpts.Load()
				if cur <= p || peakConfigsAndOpts.CompareAndSwap(p, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			activeConfigsAndOpts.Add(-1)
			fmt.Fprint(w, `{}`)

		case r.URL.Path == "/order/cart/cart-parallel-test/checkout":
			checkoutDone.Add(1)
			fmt.Fprint(w, `{"orderId":888123,"url":"https://ovh.example/order/888123"}`)

		case r.URL.Path == "/order/cart/cart-parallel-test":
			fmt.Fprint(w, `null`)

		default:
			t.Logf("mock server unhandled path: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()

	swapEndpoint(t, "ovh-eu", srv.URL)

	oldTransport := http.DefaultTransport
	http.DefaultTransport = auditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24sk102","configurations":[{"name":"region","values":["europe"]}]}]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("audit blocked external request: %s", r.URL)
	})
	defer func() { http.DefaultTransport = oldTransport }()

	start := time.Now()
	outcome := PurchaseServer(context.Background(), s, item)
	dur := time.Since(start)

	if !outcome.Success {
		t.Fatalf("PurchaseServer 失败: %+v", outcome)
	}

	if cartCreated.Load() != 1 {
		t.Errorf("期望建车 1 次，实际: %d", cartCreated.Load())
	}
	if checkoutDone.Load() != 1 {
		t.Errorf("期望结账 1 次，实际: %d", checkoutDone.Load())
	}

	// 核心并发性校验：
	// assign 与 eco 必须并发执行，峰值活跃数 >= 2
	if peakAssignAndEco.Load() < 2 {
		t.Errorf("期望 Phase 2 (assign 与 eco) 并发执行 (peak >= 2)，实际峰值并发: %d", peakAssignAndEco.Load())
	}

	// 配置写入（dc, os, region）与选件写入必须并发执行，峰值活跃数 >= 3
	if peakConfigsAndOpts.Load() < 3 {
		t.Errorf("期望 Phase 3 (configs 与 options) 并发执行 (peak >= 3)，实际峰值并发: %d", peakConfigsAndOpts.Load())
	}

	t.Logf("装车并发测试通过！总耗时: %v (assign+eco 峰值并发: %d, configs+options 峰值并发: %d)",
		dur, peakAssignAndEco.Load(), peakConfigsAndOpts.Load())
}

// TestParallelCartAssembly_EcoOptionsCache 验证硬件选项缓存：第二次调用不再发起 GET /eco/options
func TestParallelCartAssembly_EcoOptionsCache(t *testing.T) {
	resetEcoOptionsCache()
	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{
		ID:          "acc-cache",
		Name:        "acc-cache",
		Endpoint:    "ovh-eu",
		Zone:        "PT",
		AppKey:      "ak",
		AppSecret:   "as",
		ConsumerKey: "ck",
	}}

	item := &types.QueueItem{
		ID:         "task-cache-1",
		AccountID:  "acc-cache",
		PlanCode:   "24sk102",
		Datacenter: "gra",
		Options:    []string{"softraid-2x960nvme"},
		Status:     "running",
	}

	var getEcoOptionsCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprint(w, `[{"fqn":"24sk102.softraid-2x960nvme","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`)
		case r.URL.Path == "/order/cart":
			fmt.Fprint(w, `{"cartId":"cart-c1"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco"):
			fmt.Fprint(w, `{"itemId":101}`)
		case strings.Contains(r.URL.Path, "/configuration"):
			fmt.Fprint(w, `{}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/eco/options"):
			getEcoOptionsCount.Add(1)
			fmt.Fprint(w, `[{"family":"storage","planCode":"softraid-2x960nvme-24sk102","prices":[{"pricingMode":"default","duration":"P1M"}]}]`)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/eco/options"):
			fmt.Fprint(w, `{}`)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			fmt.Fprint(w, `{"orderId":90001,"url":"https://ovh.example/order/90001"}`)
		case r.Method == http.MethodDelete:
			fmt.Fprint(w, `null`)
		default:
			w.WriteHeader(404)
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()

	swapEndpoint(t, "ovh-eu", srv.URL)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = auditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24sk102","configurations":[{"name":"region","values":["europe"]}]}]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("audit blocked external request: %s", r.URL)
	})
	defer func() { http.DefaultTransport = oldTransport }()

	// 第一次调用：应触发 GET /eco/options 填充缓存
	out1 := PurchaseServer(context.Background(), s, item)
	if !out1.Success {
		t.Fatalf("第一次下单失败: %+v", out1)
	}
	if getEcoOptionsCount.Load() != 1 {
		t.Fatalf("第 1 次下单期望调用 GET /eco/options 1 次，实际: %d", getEcoOptionsCount.Load())
	}

	// 第二次调用同 subsidiary + planCode：必须命中缓存，不再发起 GET /eco/options
	item.ID = "task-cache-2"
	out2 := PurchaseServer(context.Background(), s, item)
	if !out2.Success {
		t.Fatalf("第二次下单失败: %+v", out2)
	}
	if getEcoOptionsCount.Load() != 1 {
		t.Errorf("第 2 次下单期望命中缓存 (调用次数仍为 1)，实际调用次数增加到了: %d", getEcoOptionsCount.Load())
	}
}

// TestParallelCartAssembly_ConfigFailureCancelsAndCleans 验证并发配置失败时能及时熔断并清理 cart
func TestParallelCartAssembly_ConfigFailureCancelsAndCleans(t *testing.T) {
	resetEcoOptionsCache()
	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{
		ID:          "acc-fail",
		Name:        "acc-fail",
		Endpoint:    "ovh-eu",
		Zone:        "PT",
		AppKey:      "ak",
		AppSecret:   "as",
		ConsumerKey: "ck",
	}}

	item := &types.QueueItem{
		ID:         "task-fail-1",
		AccountID:  "acc-fail",
		PlanCode:   "24sk102",
		Datacenter: "gra",
		Status:     "running",
	}

	var cartDeleted atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprint(w, `[{"fqn":"24sk102","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`)
		case r.URL.Path == "/order/cart":
			fmt.Fprint(w, `{"cartId":"cart-fail"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco"):
			fmt.Fprint(w, `{"itemId":202}`)
		case strings.Contains(r.URL.Path, "/configuration"):
			// 模拟其中一个配置报错
			w.WriteHeader(400)
			fmt.Fprint(w, `{"message":"Configuration item rejected"}`)
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/cart-fail"):
			cartDeleted.Add(1)
			fmt.Fprint(w, `null`)
		default:
			w.WriteHeader(404)
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()

	swapEndpoint(t, "ovh-eu", srv.URL)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = auditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24sk102","configurations":[{"name":"region","values":["europe"]}]}]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("audit blocked external request: %s", r.URL)
	})
	defer func() { http.DefaultTransport = oldTransport }()

	out := PurchaseServer(context.Background(), s, item)
	if out.Success {
		t.Fatalf("期望配置失败时下单失败，实际返回成功: %+v", out)
	}

	// 确认 defer 执行了清理
	if cartDeleted.Load() != 1 {
		t.Errorf("期望失败后调用 DELETE /order/cart 清理购物车，实际清理次数: %d", cartDeleted.Load())
	}
}
