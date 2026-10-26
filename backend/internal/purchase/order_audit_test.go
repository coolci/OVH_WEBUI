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

	"github.com/ovh-webui/server/internal/types"
)

type auditTransport func(*http.Request) (*http.Response, error)

func (f auditTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAuditCrossAccountQuickOrder(t *testing.T) {
	for _, mode := range []string{"queued", "recent_success"} {
		t.Run(mode, func(t *testing.T) {
			s := quickOrderTestState(t)
			s.Accounts = []types.OVHAccount{{ID: "A", Name: "A"}, {ID: "B", Name: "B"}}
			p := QuickOrderParams{AccountID: "B", PlanCode: "24sk602", Datacenter: "gra", Options: []string{"ram-opt"}, FromMonitor: true}
			if mode == "queued" {
				s.Queue = []types.QueueItem{{ID: "A-task", AccountID: "A", PlanCode: p.PlanCode, Datacenter: p.Datacenter, Options: p.Options, Status: "running"}}
			} else {
				s.History = []types.PurchaseHistoryEntry{{ID: "A-history", AccountID: "A", PlanCode: p.PlanCode, Datacenter: p.Datacenter, Options: p.Options, Status: "success", PurchaseTime: types.NowISO()}}
				p.SkipDuplicateCheck = true
			}
			res := EnqueueQuickOrder(s, p)
			if !res.Success {
				t.Fatalf("account B blocked by account A %s: HTTP %d, %s", mode, res.HTTPStatus, res.Error)
			}
		})
	}
}

func TestAuditPauseBeforeCheckout(t *testing.T) {
	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{ID: "A", Name: "A", Endpoint: "ovh-eu", Zone: "IE", AppKey: "ak", AppSecret: "as", ConsumerKey: "ck"}}
	item := types.QueueItem{ID: "audit-task", AccountID: "A", PlanCode: "audit-plan", Datacenter: "gra", Status: "running", AutoPay: true}
	if err := s.EnqueueItems([]types.QueueItem{item}, false); err != nil {
		t.Fatal(err)
	}
	var checkout atomic.Int64
	blocked := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case "/dedicated/server/datacenter/availabilities":
			fmt.Fprint(w, `[{"fqn":"audit-plan","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`)
		case "/order/cart":
			fmt.Fprint(w, `{"cartId":"audit-cart"}`)
		case "/order/cart/audit-cart/assign":
			fmt.Fprint(w, `null`)
		case "/order/cart/audit-cart/eco":
			fmt.Fprint(w, `{"itemId":1}`)
		case "/order/cart/audit-cart/item/1/configuration":
			once.Do(func() { close(blocked); <-release })
			fmt.Fprint(w, `{}`)
		case "/order/cart/audit-cart/checkout":
			checkout.Add(1)
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["autoPayWithPreferredPaymentMethod"] != true {
				t.Errorf("autoPay missing")
			}
			w.WriteHeader(400)
			fmt.Fprint(w, `{"message":"audit checkout rejected; no real order"}`)
		case "/order/cart/audit-cart":
			fmt.Fprint(w, `null`)
		default:
			t.Errorf("unexpected local mock path: %s", r.URL.Path)
			w.WriteHeader(404)
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	swapEndpoint(t, "ovh-eu", srv.URL)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = auditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"audit-plan","configurations":[{"name":"region","values":["europe"]}]}]}`)), Request: r}, nil
		}
		return nil, fmt.Errorf("audit blocked external request: %s", r.URL)
	})
	defer func() { http.DefaultTransport = oldTransport }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.RegisterTaskCancel(item.ID, cancel)
	defer s.UnregisterTaskCancel(item.ID)
	done := make(chan Outcome, 1)
	go func() { done <- PurchaseServer(ctx, s, &item) }()
	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("mock configuration not reached")
	}
	target, err := s.OrderService.UpdateQueueStatus(item.ID, "paused")
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	t.Logf("pause completed before checkout: target=%s contextErr=%v", target.Status, ctx.Err())
	close(release)
	select {
	case outcome := <-done:
		t.Logf("purchase outcome=%+v", outcome)
	case <-time.After(5 * time.Second):
		t.Fatal("purchase did not finish")
	}
	if checkout.Load() != 0 {
		t.Fatalf("checkout sent %d times AFTER successful pause, with autoPay=true", checkout.Load())
	}
}

func TestAuditFailedEnqueueBeforeCheckout(t *testing.T) {
	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{ID: "A", Name: "A", Endpoint: "ovh-eu", Zone: "IE", AppKey: "ak", AppSecret: "as", ConsumerKey: "ck"}}
	item := types.QueueItem{ID: "audit-task", AccountID: "A", PlanCode: "audit-plan", Datacenter: "gra", Status: "running", AutoPay: true}
	// 破坏数据库表以模拟落库失败
	if _, err := s.DB.Exec("DROP TABLE queue"); err != nil {
		t.Fatal(err)
	}
	err := s.EnqueueItems([]types.QueueItem{item}, false)
	if err == nil {
		t.Fatal("落库失败时入队必须返回错误")
	}
	if len(s.QueueSnapshot()) != 0 {
		t.Fatalf("落库失败的任务绝不允许暴露进运行队列: got %d", len(s.QueueSnapshot()))
	}
}
