package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func setupTestMonitor(t *testing.T) (*Monitor, *app.State) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	st := app.NewState(storage.Paths{LogsDir: filepath.Join(dir, "logs"), DataDir: dir}, config.New(database), logger.New(filepath.Join(dir, "logs.json"), nil), database)
	m := New(st)
	return m, st
}

func swapTestEndpoint(t *testing.T, name, rawURL string) {
	t.Helper()
	orig := ovhsdk.Endpoints[name]
	ovhsdk.Endpoints[name] = rawURL
	t.Cleanup(func() { ovhsdk.Endpoints[name] = orig })
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestVerifyPriceAvailable_PrewarmedCartHit(t *testing.T) {
	m, st := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(st)

	accountID := "acc-prewarm-1"
	planCode := "24sk-prewarm"
	dc := "gra"
	options := []string{"ram-32g"}

	// 1. 预热池尚无车时，若无 mock HTTP 会走真实查询失败
	cfgInfo := map[string]interface{}{
		"account_id": accountID,
		"options":    options,
	}
	ok, _ := m.verifyPriceAvailable(planCode, dc, cfgInfo)
	if ok {
		t.Fatal("未注入预热车且无有效 OVH client 时，价格校验不应该通过")
	}

	// 2. 注入预热就绪车
	cart := &purchase.PrewarmedCart{
		CartID:     "cart-prewarmed-999",
		AccountID:  accountID,
		PlanCode:   planCode,
		Datacenter: dc,
		Options:    options,
		CreatedAt:  time.Now(),
	}
	purchase.PutPrewarmedCart(cart)

	// 3. 预热车命中！verifyPriceAvailable 必须 0ms 直接通过，无需网络请求
	ok, errMsg := m.verifyPriceAvailable(planCode, dc, cfgInfo)
	if !ok {
		t.Fatalf("命中预热车时 verifyPriceAvailable 应当直接放行，但返回失败: %s", errMsg)
	}
}

func TestMonitorPrewarm_TriggerAndDedupe(t *testing.T) {
	m, st := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(st)
	ResetPrewarmCoordinator()
	defer ResetPrewarmCoordinator()

	var cartCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			cartCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"cartId":"cart-test-123"}`))
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/assign"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`null`))
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"itemId":1001}`))
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/configuration"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/eco/options"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	accountID := "acc-prewarm-trigger"
	st.Accounts = []types.OVHAccount{
		{
			ID:          accountID,
			Name:        accountID,
			Endpoint:    "ovh-eu",
			Zone:        "PT",
			AppKey:      "key",
			AppSecret:   "sec",
			ConsumerKey: "ck",
		},
	}

	swapTestEndpoint(t, "ovh-eu", srv.URL)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24sk-prewarm-auto","configurations":[{"name":"region","values":["europe"]}]}]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("blocked: %s", r.URL)
	})
	defer func() { http.DefaultTransport = oldTransport }()

	sub := &Subscription{
		PlanCode:           "24sk-prewarm-auto",
		Datacenters:        []string{"gra"},
		AutoOrder:          true,
		AutoOrderAccountID: accountID,
		Options:            []string{},
	}

	// 触发装车
	m.TriggerPrewarmForSubscription(sub)

	// 等待装车协程完成
	deadline := time.Now().Add(3 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		if purchase.HasPrewarmedCart(accountID, "24sk-prewarm-auto", "gra", []string{}, "", "") {
			found = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !found {
		t.Fatalf("预热装车未在预热池中生成就绪购物车 (cartCalls=%d)", cartCalls.Load())
	}

	// 再次触发，由于已有预热车，应该去重不会产生额外装车调用
	prevCalls := cartCalls.Load()
	m.TriggerPrewarmForSubscription(sub)
	time.Sleep(100 * time.Millisecond)
	if cartCalls.Load() != prevCalls {
		t.Errorf("已有预热车时不应该重复装车: prev=%d, curr=%d", prevCalls, cartCalls.Load())
	}
}

func TestMonitorEndToEnd_PrewarmedSniper(t *testing.T) {
	m, st := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(st)

	accountID := "acc-sniper"
	planCode := "24sk-sniper-auto"
	dc := "gra"
	options := []string{"softraid-2x960nvme-24sk-sniper-auto"}

	st.Accounts = []types.OVHAccount{
		{
			ID:          accountID,
			Name:        accountID,
			Endpoint:    "ovh-eu",
			Zone:        "ES",
			AppKey:      "key",
			AppSecret:   "sec",
			ConsumerKey: "ck",
		},
	}

	var (
		checkoutCalls atomic.Int32
		cartCreations atomic.Int32
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			// 探测返回：GRA 有货 (1H)，且匹配硬件选件
			fmt.Fprint(w, `[{"fqn":"24sk-sniper-auto.softraid-2x960nvme","storage":"softraid-2x960nvme","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`)
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			cartCreations.Add(1)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"cartId":"late-cart"}`))
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			checkoutCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"orderId":888999,"url":"https://ovh.example/checkout"}`))
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	swapTestEndpoint(t, "ovh-eu", srv.URL)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24sk-sniper-auto","configurations":[{"name":"region","values":["europe"]}],"addonFamilies":[{"name":"storage","addons":["softraid-2x960nvme-24sk-sniper-auto"]}]}]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("blocked: %s", r.URL)
	})
	defer func() { http.DefaultTransport = oldTransport }()

	// 1. 模拟扫描期间已提前装好预热车并待命
	cart := &purchase.PrewarmedCart{
		CartID:     "pw-cart-live",
		AccountID:  accountID,
		PlanCode:   planCode,
		Datacenter: dc,
		Options:    options,
		CreatedAt:  time.Now(),
	}
	purchase.PutPrewarmedCart(cart)

	// 2. 添加监控订阅（上次状态为 unavailable，开启自动下单）
	m.AddSubscription(planCode, []string{dc}, true, false, "KS-Sniper",
		map[string]string{dc: "unavailable"}, nil, true, 1, accountID, false, options)

	sub := m.subscriptions[0]

	// 3. 执行监控探测扫描：探针命中 GRA 从 unavailable -> 1H (有货！)
	m.CheckAvailabilityChange(sub, "trace-test-sniper")

	// 4. 验证：由于预热车已就绪，verifyPriceAvailable 直通通过，快速下单任务已入队！
	st.QueueMu.Lock()
	qLen := len(st.Queue)
	st.QueueMu.Unlock()
	if qLen != 1 {
		t.Fatalf("探针命中后应成功将抢购任务入队，实际队列长度: %d", qLen)
	}

	// 5. 模拟购买队列处理：执行器执行抢购
	beforePurchaseCartCalls := cartCreations.Load()
	item := st.Queue[0]
	outcome := purchase.PurchaseServer(context.Background(), st, &item)
	if !outcome.Success {
		t.Fatalf("预期抢购成功，实际 outcome: %+v", outcome)
	}

	// 6. 核心指标验证：
	// 抢购阶段装车耗时为 0，因为直接复用了预装车，绝不允许在抢购阶段发起新的建车请求！
	if cartCreations.Load() != beforePurchaseCartCalls {
		t.Fatalf("命中预热车时，抢购阶段不应发起新的建车请求，实际建车新增: %d", cartCreations.Load()-beforePurchaseCartCalls)
	}
	if checkoutCalls.Load() != 1 {
		t.Fatalf("应当直接直出一击发向 checkout，实际 checkout 次数: %d", checkoutCalls.Load())
	}
}
