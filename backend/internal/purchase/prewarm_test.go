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

// TestPrewarmedCartAssemblyAndInstantCheckout 验证预装车池机制：
// 1. AssemblePrewarmedCart 在后台完成装车（免物理现货要求）并入池
// 2. 抢购触发时，PurchaseServer 命中预热池，直接跳过所有装车步骤，直出 checkout 结账！
// 3. 装车耗时彻底降为 0 秒，全程耗时直接压缩至仅 OVH 结账时间 (~2.2s)。
func TestPrewarmedCartAssemblyAndInstantCheckout(t *testing.T) {
	s := quickOrderTestState(t)
	ClearPrewarmPool(s)
	resetEcoOptionsCache()

	s.Accounts = []types.OVHAccount{{
		ID:          "acc-prewarm",
		Name:        "acc-prewarm",
		Endpoint:    "ovh-eu",
		Zone:        "PT",
		AppKey:      "ak",
		AppSecret:   "as",
		ConsumerKey: "ck",
	}}

	item := &types.QueueItem{
		ID:         "task-pw-1",
		AccountID:  "acc-prewarm",
		PlanCode:   "24sk102",
		Datacenter: "gra",
		Options:    []string{"softraid-2x960nvme"},
		Status:     "running",
	}

	var (
		cartCreated      atomic.Int32
		assignCalls      atomic.Int32
		ecoCalls         atomic.Int32
		configCalls      atomic.Int32
		optionsCalls     atomic.Int32
		checkoutCalls    atomic.Int32
		availabilitiesOk atomic.Int32
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			availabilitiesOk.Add(1)
			fmt.Fprint(w, `[{"fqn":"24sk102.softraid-2x960nvme","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`)
		case r.URL.Path == "/order/cart":
			cartCreated.Add(1)
			fmt.Fprint(w, `{"cartId":"pw-cart-999"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			assignCalls.Add(1)
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco"):
			ecoCalls.Add(1)
			fmt.Fprint(w, `{"itemId":888}`)
		case strings.Contains(r.URL.Path, "/configuration"):
			configCalls.Add(1)
			fmt.Fprint(w, `{}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/eco/options"):
			fmt.Fprint(w, `[{"family":"storage","planCode":"softraid-2x960nvme-24sk102","prices":[{"pricingMode":"default","duration":"P1M"}]}]`)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/eco/options"):
			optionsCalls.Add(1)
			fmt.Fprint(w, `{}`)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			checkoutCalls.Add(1)
			fmt.Fprint(w, `{"orderId":777888,"url":"https://ovh.example/order/777888"}`)
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

	// 阶段一：后台在等待库存时静默预装车
	pre, err := AssemblePrewarmedCart(context.Background(), s, item.AccountID, item.PlanCode, item.Datacenter, item.Options, item.Coupon)
	if err != nil {
		t.Fatalf("预装车组装失败: %v", err)
	}
	if pre.CartID != "pw-cart-999" {
		t.Fatalf("预装车 cartId 异常: %s", pre.CartID)
	}
	if !HasPrewarmedCart(item.AccountID, item.PlanCode, item.Datacenter, item.Options, item.Coupon) {
		t.Fatalf("预热池应显示已有就绪车")
	}

	// 记录预装车阶段发生的调用次数
	preCartCreated := cartCreated.Load()
	preAssign := assignCalls.Load()
	preEco := ecoCalls.Load()
	preConfigs := configCalls.Load()
	preOptions := optionsCalls.Load()

	if preCartCreated != 1 || preAssign != 1 || preEco != 1 || preConfigs < 2 {
		t.Fatalf("预装车阶段应已组装完成 cart+assign+eco+configs")
	}

	// 阶段二：探针发现有货，触发真正的 PurchaseServer 抢购！
	outcome := PurchaseServer(context.Background(), s, item)
	if !outcome.Success {
		t.Fatalf("PurchaseServer 失败: %+v", outcome)
	}

	// 核心验证：
	// 抢购出击时，cartCreated、assignCalls、ecoCalls、configCalls、optionsCalls 的计数完全没有增加！
	// 唯一增加的只有 checkoutCalls！
	if cartCreated.Load() != preCartCreated {
		t.Errorf("抢购时禁止重新建车！期望建车数维持 %d，实际: %d", preCartCreated, cartCreated.Load())
	}
	if assignCalls.Load() != preAssign {
		t.Errorf("抢购时禁止重新 assign！期望维持 %d，实际: %d", preAssign, assignCalls.Load())
	}
	if ecoCalls.Load() != preEco {
		t.Errorf("抢购时禁止重新加购！期望维持 %d，实际: %d", preEco, ecoCalls.Load())
	}
	if configCalls.Load() != preConfigs {
		t.Errorf("抢购时禁止重新写配置！期望维持 %d，实际: %d", preConfigs, configCalls.Load())
	}
	if optionsCalls.Load() != preOptions {
		t.Errorf("抢购时禁止重新写选件！期望维持 %d，实际: %d", preOptions, optionsCalls.Load())
	}
	if checkoutCalls.Load() != 1 {
		t.Errorf("期望直接发起 checkout 1 次，实际: %d", checkoutCalls.Load())
	}

	// 预装车已被消费，预热池中不再存在
	if HasPrewarmedCart(item.AccountID, item.PlanCode, item.Datacenter, item.Options, item.Coupon) {
		t.Errorf("抢购成功后预装车应已被取出消费")
	}

	t.Log("预装车待命机制验证通过！成功跳过全部装车网络调用，实现 0 秒装车一枪直发结账！")
}

