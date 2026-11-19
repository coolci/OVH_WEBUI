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

func TestQueueTaskAutoPrewarm(t *testing.T) {
	s := quickOrderTestState(t)
	ClearPrewarmPool(s)
	resetEcoOptionsCache()
	ResetQueuePrewarmCoord()
	defer ResetQueuePrewarmCoord()

	accountID := "acc-q-prewarm"
	s.Accounts = []types.OVHAccount{{
		ID:          accountID,
		Name:        accountID,
		Endpoint:    "ovh-eu",
		Zone:        "PT",
		AppKey:      "ak",
		AppSecret:   "as",
		ConsumerKey: "ck",
		IsDefault:   true,
	}}

	var (
		cartCreated   atomic.Int32
		checkoutCalls atomic.Int32
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprint(w, `[{"fqn":"24sk-queue-plan","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`)
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			cartCreated.Add(1)
			fmt.Fprint(w, `{"cartId":"q-pw-cart-1"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco"):
			fmt.Fprint(w, `{"itemId":777}`)
		case strings.Contains(r.URL.Path, "/configuration"):
			fmt.Fprint(w, `{}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/eco/options"):
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			checkoutCalls.Add(1)
			fmt.Fprint(w, `{"orderId":999111,"url":"https://ovh.example/q-checkout"}`)
		default:
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
				Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24sk-queue-plan","configurations":[{"name":"region","values":["europe"]}]}]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("blocked: %s", r.URL)
	})
	defer func() { http.DefaultTransport = oldTransport }()

	item := &types.QueueItem{
		ID:         "task-q-1",
		AccountID:  accountID,
		PlanCode:   "24sk-queue-plan",
		Datacenter: "gra",
		Options:    []string{},
		Status:     "running",
	}

	// 1. 触发队列任务后台自动预热
	TriggerPrewarmForQueueItem(context.Background(), s, item)

	// 等待装车协程完成
	deadline := time.Now().Add(3 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		if HasPrewarmedCart(accountID, item.PlanCode, item.Datacenter, item.Options, item.Coupon, "") {
			found = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !found {
		t.Fatalf("队列任务未在预热池中生成就绪购物车 (cartCreated=%d)", cartCreated.Load())
	}

	// 2. 模拟购买执行：执行抢购
	beforePurchaseCartCalls := cartCreated.Load()
	outcome := PurchaseServer(context.Background(), s, item)
	if !outcome.Success {
		t.Fatalf("预期购买成功，实际 outcome: %+v", outcome)
	}

	// 3. 验证：抢购阶段装车耗时为 0，因为直接命中并消费了预装车
	if cartCreated.Load() != beforePurchaseCartCalls {
		t.Fatalf("命中预热车时，抢购阶段不应发起新的建车请求，实际建车新增: %d", cartCreated.Load()-beforePurchaseCartCalls)
	}
	if checkoutCalls.Load() != 1 {
		t.Fatalf("应当直接直出结账，实际 checkout 次数: %d", checkoutCalls.Load())
	}
}
