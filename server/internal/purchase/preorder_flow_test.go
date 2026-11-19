package purchase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
)

// 预订完整链路(本地 fixture 应答所有请求,不会真实下单):
// 开售前每一步被 OVH 明确拒绝都只是"等待",不写失败历史、不结账;
// 开售后按库存里的精确配置下单一次,结账前核对购物车,核对不上绝不结账。

const preorderPlan = "27skle03-v1"

type preorderFixture struct {
	mu sync.Mutex
	// 开售开关与各种异常
	launched        bool
	optionsOpen     bool
	storedMemory    string // 购物车 GET item 返回的内存 addon(核对用)
	stockFQN        string
	baseRejections  atomic.Int32
	checkouts       atomic.Int32
	addedOptions    []string
	catalogRequests atomic.Int32
	// apiCalls 打到 OVH 的请求数(不含 /auth/time),用来衡量等待期每轮的开销
	apiCalls atomic.Int32
}

func (f *preorderFixture) set(fn func(*preorderFixture)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func newPreorderFixture(t *testing.T) (*app.State, *preorderFixture) {
	t.Helper()
	s := quickOrderTestState(t)
	ClearPrewarmPool(nil)
	resetEcoOptionsCache()
	resetPreorderGate()
	s.Accounts = []types.OVHAccount{{ID: "pre-acc", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}
	f := &preorderFixture{storedMemory: "ram-64g-ddr4-ecc-2933-" + preorderPlan,
		stockFQN: preorderPlan + ".ram-64g-ddr4-ecc-2933.softraid-2x480ssd"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		f.mu.Lock()
		launched, optionsOpen, stored, fqn := f.launched, f.optionsOpen, f.storedMemory, f.stockFQN
		f.mu.Unlock()
		path := r.URL.Path
		if path != "/auth/time" {
			f.apiCalls.Add(1)
		}
		switch {
		case path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case path == "/me/order":
			io.WriteString(w, `[]`)
		case path == "/dedicated/server/datacenter/availabilities":
			// 同系列另一个型号也在库存里且有货:预订绝不能拿它下单
			fmt.Fprintf(w, `[{"fqn":%q,"datacenters":[{"datacenter":"gra","availability":"1H-low"}]},
				{"fqn":"27skle03-v2.ram-64g-ddr4-ecc-2933.softraid-2x480ssd","datacenters":[{"datacenter":"gra","availability":"1H-high"}]}]`, fqn)
		case path == "/order/cart" && r.Method == http.MethodPost:
			io.WriteString(w, `{"cartId":"pre-cart"}`)
		case strings.HasSuffix(path, "/assign"):
			io.WriteString(w, `null`)
		case strings.HasSuffix(path, "/eco") && r.Method == http.MethodPost:
			if !launched {
				f.baseRejections.Add(1)
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `{"message":"Plan code not available yet"}`)
				return
			}
			io.WriteString(w, `{"itemId":5}`)
		case strings.HasSuffix(path, "/requiredConfiguration"):
			io.WriteString(w, `[]`)
		case strings.HasSuffix(path, "/configuration"):
			io.WriteString(w, `null`)
		case strings.HasSuffix(path, "/eco/options") && r.Method == http.MethodGet:
			memory := `{"planCode":"ram-32g-ddr4-ecc-2933-` + preorderPlan + `","prices":[{"duration":"P1M","pricingMode":"default","capacities":["renew"]}]}`
			if optionsOpen {
				memory += `,{"planCode":"ram-64g-ddr4-ecc-2933-` + preorderPlan + `","prices":[{"duration":"P1M","pricingMode":"default","capacities":["renew"]}]}`
			}
			fmt.Fprintf(w, `[%s,{"planCode":"softraid-2x480ssd-%s","prices":[{"duration":"P1M","pricingMode":"default","capacities":["renew"]}]}]`, memory, preorderPlan)
		case strings.HasSuffix(path, "/eco/options"):
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.addedOptions = append(f.addedOptions, fmt.Sprint(body["planCode"]))
			f.mu.Unlock()
			io.WriteString(w, `null`)
		case path == "/order/cart/pre-cart" && r.Method == http.MethodGet:
			io.WriteString(w, `{"items":[5]}`)
		case path == "/order/cart/pre-cart/item/5":
			fmt.Fprintf(w, `{"itemId":5,"duration":"P1M","options":[6,7],"settings":{"planCode":%q,"quantity":1}}`, preorderPlan)
		case path == "/order/cart/pre-cart/item/6":
			fmt.Fprintf(w, `{"itemId":6,"duration":"P1M","options":[],"settings":{"planCode":%q,"quantity":1}}`, stored)
		case path == "/order/cart/pre-cart/item/7":
			fmt.Fprintf(w, `{"itemId":7,"duration":"P1M","options":[],"settings":{"planCode":"softraid-2x480ssd-%s","quantity":1}}`, preorderPlan)
		case strings.HasSuffix(path, "/checkout"):
			f.checkouts.Add(1)
			io.WriteString(w, `{"orderId":4242,"url":"https://pre.invalid/order"}`)
		case r.Method == http.MethodDelete:
			io.WriteString(w, `null`)
		default:
			io.WriteString(w, `null`)
		}
	}))
	t.Cleanup(srv.Close)
	swapEndpoint(t, "ovh-eu", srv.URL)
	base := http.DefaultTransport
	transport := auditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.String(), srv.URL) {
			return base.RoundTrip(r)
		}
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			// 公开销售目录一直没收录这个型号:预订不能因此卡住
			f.catalogRequests.Add(1)
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"plans":[]}`)), Request: r}, nil
		}
		return nil, fmt.Errorf("external request blocked: %s", r.URL)
	})
	http.DefaultTransport = transport
	restore := netfp.SwapSharedTransportForTesting(transport)
	t.Cleanup(func() { restore(); http.DefaultTransport = base; ClearPrewarmPool(nil); resetEcoOptionsCache() })
	return s, f
}

func preorderTask() *types.QueueItem {
	return &types.QueueItem{ID: "pre-task", AccountID: "pre-acc", PlanCode: preorderPlan, Datacenter: "gra", Status: "running",
		Preorder: true, Options: []string{"CPU-Intel-unpublished", "ram-64g-ddr4-ecc-2933", "softraid-2x480ssd"}}
}

func historyCount(s *app.State) int {
	s.HistoryMu.Lock()
	defer s.HistoryMu.Unlock()
	return len(s.History)
}

func TestPreorderWaitsSilentlyThenOrdersExactConfigurationOnce(t *testing.T) {
	s, f := newPreorderFixture(t)

	// 开售前:基础商品被 OVH 拒(400)→ 等待,不写历史、不结账、不算尝试
	for i := 0; i < 3; i++ {
		out := PurchaseServer(context.Background(), s, preorderTask())
		if !out.WaitingForCatalog || out.Attempted || out.Fatal || out.Success {
			t.Fatalf("round %d before launch must just wait: %+v", i, out)
		}
	}
	if f.baseRejections.Load() != 3 || f.checkouts.Load() != 0 || historyCount(s) != 0 {
		t.Fatalf("before launch: rejections=%d checkouts=%d history=%d", f.baseRejections.Load(), f.checkouts.Load(), historyCount(s))
	}

	// 型号开售但 64G 内存配置还没开放:继续等,不能退而求其次买 32G
	f.set(func(f *preorderFixture) { f.launched = true })
	out := PurchaseServer(context.Background(), s, preorderTask())
	if !out.WaitingForCatalog || f.checkouts.Load() != 0 || historyCount(s) != 0 {
		t.Fatalf("a configuration that is not open yet must keep waiting: %+v checkouts=%d", out, f.checkouts.Load())
	}

	// 配置开放:下单一次,选项是精确映射后的 addon,CPU 不参与
	f.set(func(f *preorderFixture) { f.optionsOpen = true })
	task := preorderTask()
	out = PurchaseServer(context.Background(), s, task)
	if !out.Success || f.checkouts.Load() != 1 {
		t.Fatalf("launched preorder must order once: %+v checkouts=%d", out, f.checkouts.Load())
	}
	f.mu.Lock()
	added := strings.Join(f.addedOptions, ",")
	f.mu.Unlock()
	if !strings.Contains(added, "ram-64g-ddr4-ecc-2933-"+preorderPlan) || !strings.Contains(added, "softraid-2x480ssd-"+preorderPlan) ||
		strings.Contains(strings.ToLower(added), "cpu") || strings.Contains(added, "ram-32g") {
		t.Fatalf("added options must be exactly the stock configuration: %s", added)
	}
	s.HistoryMu.Lock()
	defer s.HistoryMu.Unlock()
	if len(s.History) != 1 || s.History[0].Status != "success" {
		t.Fatalf("one success history entry expected: %+v", s.History)
	}
	if opts := strings.Join(s.History[0].Options, ","); !strings.Contains(opts, "-"+preorderPlan) {
		t.Fatalf("history must record the options actually ordered: %s", opts)
	}
}

func TestPreorderNeverChecksOutAMismatchedCart(t *testing.T) {
	s, f := newPreorderFixture(t)
	f.set(func(f *preorderFixture) {
		f.launched, f.optionsOpen = true, true
		f.storedMemory = "ram-32g-ddr4-ecc-2933-" + preorderPlan // OVH 实际存下的是另一档内存
	})
	out := PurchaseServer(context.Background(), s, preorderTask())
	if out.Success || !out.WaitingForCatalog || f.checkouts.Load() != 0 {
		t.Fatalf("a cart whose stored hardware differs must never be checked out: %+v checkouts=%d", out, f.checkouts.Load())
	}
}

func TestPreorderIgnoresStockOfOtherModels(t *testing.T) {
	s, f := newPreorderFixture(t)
	// 本型号这套配置在库存里消失,只剩同系列 v2 有货
	f.set(func(f *preorderFixture) {
		f.launched, f.optionsOpen = true, true
		f.stockFQN = preorderPlan + ".ram-32g-ddr4-ecc-2933.softraid-2x480ssd"
	})
	out := PurchaseServer(context.Background(), s, preorderTask())
	if out.Success || out.Attempted || f.checkouts.Load() != 0 || historyCount(s) != 0 {
		t.Fatalf("stock of another model or another configuration must not trigger an order: %+v", out)
	}
}

// 队列层(驱动真实的 processQueueLoop):预订等待不计失败次数、不把任务判死;
// 轮询间隔封顶 15 秒,不跟随普通任务的长重试间隔 —— 用"上次检查在 20 秒前、
// 重试间隔 1 小时"的任务验证:预订到点了,普通任务没到点。
func TestPreorderWaitDoesNotBurnRetriesAndPollsFast(t *testing.T) {
	st := queueCompletionState(t)
	last := float64(time.Now().Add(-20 * time.Second).Unix())
	st.Queue = []types.QueueItem{
		{ID: "pre", PlanCode: "p", Status: "running", Preorder: true, RetryInterval: 3600, MaxRetries: 1, LastCheckTime: last},
		{ID: "normal", PlanCode: "p", Status: "running", RetryInterval: 3600, MaxRetries: 1, LastCheckTime: last},
	}
	if err := st.SaveQueue(); err != nil {
		t.Fatal(err)
	}
	var preCalls, normalCalls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		processQueueLoop(ctx, st, func(_ context.Context, _ *app.State, item *types.QueueItem) Outcome {
			if item.ID == "pre" {
				preCalls.Add(1)
				return Outcome{WaitingForCatalog: true, Reason: "not launched"}
			}
			normalCalls.Add(1)
			return Outcome{}
		})
	}()
	deadline := time.Now().Add(5 * time.Second)
	for preCalls.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	if preCalls.Load() < 1 {
		t.Fatal("preorder must be polled within 15s even with a 1h retry interval")
	}
	if normalCalls.Load() != 0 {
		t.Fatalf("an ordinary task must keep its own 1h interval, ran %d times", normalCalls.Load())
	}
	st.QueueMu.Lock()
	defer st.QueueMu.Unlock()
	for _, q := range st.Queue {
		if q.ID == "pre" && (q.FailureCount != 0 || q.Status != "running") {
			t.Fatalf("waiting must not burn retries or stop the task: %+v", q)
		}
	}
}
