package vps

import (
	"fmt"
	"io"
	"log/slog"
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
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

// VPS 补货自动下单(重新开启后)。所有请求由本地 fixture 应答,不会真实下单。
// 关键约束:只买月付;成功 / 确定性失败后关闭该订阅的自动下单(一个订阅只抢一次);
// 订阅中途被改或删除就不下单;结果同步落库。

type vpsOrderFixture struct {
	checkouts atomic.Int32
	carts     atomic.Int32
	offers    atomic.Value // GET /order/cart/{id}/vps 的响应
}

func newVPSOrderState(t *testing.T) (*app.State, *vpsOrderFixture) {
	t.Helper()
	f := &vpsOrderFixture{}
	f.offers.Store(`[{"planCode":"vps-auto","prices":[{"duration":"P12M","pricingMode":"upfront12","capacities":["renew"]},{"duration":"P1M","pricingMode":"default","capacities":["renew"]}]}]`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			f.carts.Add(1)
			fmt.Fprint(w, `{"cartId":"auto-cart"}`)
		case r.URL.Path == "/order/cart/auto-cart/assign":
			fmt.Fprint(w, `null`)
		case r.Method == "GET" && r.URL.Path == "/order/cart/auto-cart/vps":
			fmt.Fprint(w, f.offers.Load().(string))
		case r.Method == "POST" && r.URL.Path == "/order/cart/auto-cart/vps":
			fmt.Fprint(w, `{"itemId":3}`)
		case strings.HasSuffix(r.URL.Path, "/requiredConfiguration"):
			fmt.Fprint(w, `[{"label":"vps_datacenter","required":true},{"label":"vps_os","allowedValues":["Debian 12"]}]`)
		case strings.HasSuffix(r.URL.Path, "/configuration"):
			fmt.Fprint(w, `{}`)
		case strings.HasSuffix(r.URL.Path, "/vps/options"):
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			f.checkouts.Add(1)
			fmt.Fprint(w, `{"orderId":555,"url":"https://example.invalid/order/555"}`)
		case r.Method == "DELETE":
			fmt.Fprint(w, `null`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"message":"unexpected %s %s"}`, r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = server.URL
	t.Cleanup(func() { ovhsdk.Endpoints["ovh-eu"] = orig })

	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	lg := logger.New(filepath.Join(dir, "t.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	state := app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
	state.Accounts = []types.OVHAccount{{ID: "acc-vps", Name: "A", Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}
	return state, f
}

func addAutoSub(t *testing.T, state *app.State) types.VPSSubscription {
	t.Helper()
	sub := types.VPSSubscription{ID: "auto-sub", PlanCode: "vps-auto", OvhSubsidiary: "IE", MonitorLinux: true,
		NotifyAvailable: true, LastStatus: map[string]string{}, History: []map[string]interface{}{},
		CreatedAt: types.NowISO(), AutoOrder: true, AutoOrderAccountID: "acc-vps", Quantity: 1}
	if err := state.AddVPSSubscription(sub); err != nil {
		t.Fatal(err)
	}
	return sub
}

var restockDCs = []map[string]interface{}{{"name": "GRA", "code": "eu-west-gra"}, {"name": "SBG", "code": "eu-west-sbg"}}

func persistedSub(t *testing.T, state *app.State, id string) types.VPSSubscription {
	t.Helper()
	subs, err := state.DB.ListVPSSubscriptions()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range subs {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("subscription %s not persisted", id)
	return types.VPSSubscription{}
}

func TestRestockOrdersOnceMonthlyAndThenStops(t *testing.T) {
	state, f := newVPSOrderState(t)
	sub := addAutoSub(t, state)
	autoOrderOnRestock(state, sub, restockDCs)
	if f.checkouts.Load() != 1 {
		t.Fatalf("restock must order exactly once (stop after the first datacenter succeeds), checkouts=%d", f.checkouts.Load())
	}
	history, err := state.DB.ListHistory()
	if err != nil || len(history) != 1 || history[0].Status != "success" || history[0].OrderID != "555" || history[0].Duration != "P1M" {
		t.Fatalf("success must be saved to the database right away: %+v err=%v", history, err)
	}
	if persistedSub(t, state, sub.ID).AutoOrder {
		t.Fatal("auto-order must be switched off (and persisted) after a successful order")
	}
	// 同一订阅再补货:不再下单
	autoOrderOnRestock(state, sub, restockDCs)
	if f.checkouts.Load() != 1 {
		t.Fatalf("a subscription that already bought must not order again, checkouts=%d", f.checkouts.Load())
	}
}

func TestRestockNeverBuysANonMonthlyPeriod(t *testing.T) {
	state, f := newVPSOrderState(t)
	f.offers.Store(`[{"planCode":"vps-auto","prices":[{"duration":"P12M","pricingMode":"upfront12","capacities":["renew"]}]}]`)
	sub := addAutoSub(t, state)
	autoOrderOnRestock(state, sub, restockDCs)
	if f.checkouts.Load() != 0 {
		t.Fatalf("a yearly-only plan must not be bought by auto-order, checkouts=%d", f.checkouts.Load())
	}
	if f.carts.Load() != 1 {
		t.Fatalf("a deterministic failure must stop after the first datacenter, carts=%d", f.carts.Load())
	}
	if persistedSub(t, state, sub.ID).AutoOrder {
		t.Fatal("a deterministic failure must switch auto-order off")
	}
	history, _ := state.DB.ListHistory()
	if len(history) != 1 || history[0].Status != "failed" || !strings.Contains(*history[0].ErrorMessage, "按月") {
		t.Fatalf("failure reason must be recorded: %+v", history)
	}
}

func TestRestockSkipsWhenSubscriptionChanged(t *testing.T) {
	state, f := newVPSOrderState(t)
	sub := addAutoSub(t, state)
	// 用户在检查与下单之间关掉了自动下单
	if _, _, err := state.MutateVPSSubscription(sub.ID, func(s *types.VPSSubscription) bool { s.AutoOrder = false; return true }); err != nil {
		t.Fatal(err)
	}
	autoOrderOnRestock(state, sub, restockDCs)
	// 或者换了下单数量
	sub2 := sub
	if _, _, err := state.MutateVPSSubscription(sub.ID, func(s *types.VPSSubscription) bool { s.AutoOrder, s.Quantity = true, 3; return true }); err != nil {
		t.Fatal(err)
	}
	autoOrderOnRestock(state, sub2, restockDCs)
	if f.carts.Load() != 0 || f.checkouts.Load() != 0 {
		t.Fatalf("a changed subscription must not be ordered from a stale snapshot: carts=%d checkouts=%d", f.carts.Load(), f.checkouts.Load())
	}
}

func TestRestockAcceptsLegacySubscriptionWithoutSubsidiary(t *testing.T) {
	state, f := newVPSOrderState(t)
	sub := types.VPSSubscription{ID: "legacy", PlanCode: "vps-auto", MonitorLinux: true, LastStatus: map[string]string{},
		History: []map[string]interface{}{}, CreatedAt: types.NowISO(), AutoOrder: true, AutoOrderAccountID: "acc-vps", Quantity: 1}
	if err := state.AddVPSSubscription(sub); err != nil {
		t.Fatal(err)
	}
	// 监控循环会给缺子公司的老订阅补上推断值再交给下单
	toOrder := sub
	toOrder.OvhSubsidiary = DefaultSubsidiary(state, sub.AutoOrderAccountID)
	autoOrderOnRestock(state, toOrder, restockDCs[:1])
	if f.checkouts.Load() != 1 {
		t.Fatalf("an old subscription without a stored subsidiary must still be ordered, checkouts=%d", f.checkouts.Load())
	}
}

func TestSaveObservationPersistsBaselineBeforeOrdering(t *testing.T) {
	state, _ := newVPSOrderState(t)
	sub := addAutoSub(t, state)
	sub.LastStatus = map[string]string{"eu-west-gra": "available"}
	sub.History = []map[string]interface{}{{"changeType": "available", "datacenterCode": "eu-west-gra"}}
	if err := saveObservation(state, &sub); err != nil {
		t.Fatal(err)
	}
	got := persistedSub(t, state, sub.ID)
	if got.LastStatus["eu-west-gra"] != "available" || len(got.History) != 1 || !got.AutoOrder {
		t.Fatalf("observation must be persisted without touching order settings: %+v", got)
	}
}
