package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

func TestOrderAccountsCompatibilityAndSnapshots(t *testing.T) {
	if got := NormalizeOrderAccounts("legacy", nil); !reflect.DeepEqual(got, []string{"legacy"}) {
		t.Fatalf("legacy account lost: %v", got)
	}
	if got := NormalizeOrderAccounts("legacy", []string{}); len(got) != 0 {
		t.Fatalf("explicit clear ignored: %v", got)
	}
	if got := NormalizeOrderAccounts("", nil); len(got) != 0 {
		t.Fatalf("empty legacy subscription acquired accounts: %v", got)
	}
	m, _ := setupTestMonitor(t)
	m.AddSubscription("plan", nil, false, false, "", map[string]string{"gra|config": "available"}, nil, false, 0, "a", false, nil)
	if err := m.UpdateSubscriptionFields("plan", nil, nil, false, false, false, 0, "a", false, nil, []string{"a", " b ", "a", ""}); err != nil {
		t.Fatal(err)
	}
	snap := m.Snapshot()[0]
	if !reflect.DeepEqual(snap.AutoOrderAccountIDs, []string{"a", "b"}) || snap.AutoOrderAccountID != "a" {
		t.Fatalf("accounts not canonicalized: %+v", snap)
	}
	if snap.LastStatus[accountStatusPrefix("a")+"gra|config"] != "available" {
		t.Fatal("legacy inventory baseline lost during multi-account migration")
	}
	snap.AutoOrderAccountIDs[0] = "mutated"
	cfg := m.SubscriptionConfig("plan")
	if cfg.AutoOrderAccountIDs[0] != "a" {
		t.Fatal("snapshot shares account list with live subscription")
	}
	cfg.AutoOrderAccountIDs[0] = "mutated"
	if m.Snapshot()[0].AutoOrderAccountIDs[0] != "a" {
		t.Fatal("config snapshot shares account list")
	}
	if err := m.SaveToDB(); err != nil {
		t.Fatal(err)
	}
	m.LoadFromDB()
	if !reflect.DeepEqual(m.Snapshot()[0].AutoOrderAccountIDs, []string{"a", "b"}) {
		t.Fatal("multi-account targets lost on reload")
	}
	// A legacy client changing this subscription back to one account must keep
	// that account's scoped baseline, even well after the 120-second dedupe.
	if err := m.UpdateSubscriptionFields("plan", nil, nil, false, false, false, 0, "a", false, nil); err != nil {
		t.Fatal(err)
	}
	got := m.Snapshot()[0]
	if !reflect.DeepEqual(got.AutoOrderAccountIDs, []string{"a"}) || got.LastStatus[accountStatusPrefix("a")+"gra|config"] != "available" {
		t.Fatal("legacy edit discarded account inventory baseline")
	}
}

// Every request in these tests is served locally. The public catalog transport
// rejects any unexpected destination, and no real purchase endpoint is used.
type multiFixtureStats struct{ checkouts, carts, options atomic.Int32 }

func multiAccountFixture(t *testing.T, plan, zone string, custom bool) (*Monitor, *app.State, []string, *multiFixtureStats) {
	t.Helper()
	m, state := setupTestMonitor(t)
	options := []string{"softraid-2x960nvme-" + plan}
	stats := &multiFixtureStats{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprintf(w, `[{"fqn":%q,"storage":"softraid-2x960nvme","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`, plan+".softraid-2x960nvme")
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			stats.carts.Add(1)
			fmt.Fprint(w, `{"cartId":"test-live-cart"}`)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload["planCode"] != plan {
				t.Errorf("unexpected base product: %v %v", payload, err)
			}
			fmt.Fprint(w, `{"itemId":1}`)
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/eco/options"):
			fmt.Fprintf(w, `[{"planCode":%q,"prices":[{"duration":"P1M","pricingMode":"default"}]}]`, options[0])
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco/options"):
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload["planCode"] != options[0] {
				t.Errorf("incorrect or duplicated hardware option: %v %v", payload, err)
			}
			stats.options.Add(1)
			fmt.Fprint(w, `{}`)
		case strings.HasSuffix(r.URL.Path, "/summary"):
			fmt.Fprint(w, `{"prices":{"withoutTax":{"value":12,"currencyCode":"EUR"},"withTax":{"value":15,"currencyCode":"EUR"}}}`)
		case strings.HasSuffix(r.URL.Path, "/requiredConfiguration"):
			fmt.Fprint(w, `[]`)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/checkout"):
			stats.checkouts.Add(1)
			fmt.Fprint(w, `{"orderId":987654,"url":"https://ovh.example/test-order"}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(server.Close)
	swapTestEndpoint(t, "ovh-eu", server.URL)
	// Price and option caches are process-wide. Isolate repeated -count runs
	// without clearing production caches or weakening their behavior.
	accountSuffix := uuid.NewString()
	state.Accounts = []types.OVHAccount{
		{ID: "multi-a-" + accountSuffix, Name: "Account A", Endpoint: "ovh-eu", Zone: zone, AppKey: "key", AppSecret: "secret", ConsumerKey: "a", IsDefault: true},
		{ID: "multi-b-" + accountSuffix, Name: "Account B", Endpoint: "ovh-eu", Zone: zone, AppKey: "key", AppSecret: "secret", ConsumerKey: "b"},
	}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return nil, fmt.Errorf("test blocked external request: %s", r.URL)
		}
		body := fmt.Sprintf(`{"plans":[{"planCode":%q,"configurations":[{"name":"region","values":["europe"]}],"addonFamilies":[{"name":"storage","addons":[%q]}]}]}`, plan, options[0])
		if custom {
			body = `{"plans":[]}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	t.Cleanup(func() { purchase.ClearPrewarmPool(state) })
	return m, state, options, stats
}

func readyMultiCart(accountID, plan string, options []string) {
	purchase.PutPrewarmedCart(&purchase.PrewarmedCart{CartID: "test-cart-" + accountID, AccountID: accountID, PlanCode: plan, Datacenter: "gra", Options: options, CreatedAt: time.Now()})
}

func TestMultiAccountMonitorOrdersQuantityAndSurvivesRestart(t *testing.T) {
	plan := "multi-monitor-plan"
	m, state, options, checkoutCalls := multiAccountFixture(t, plan, "DE", false)
	ids := []string{state.Accounts[0].ID, state.Accounts[1].ID}
	for _, id := range ids {
		readyMultiCart(id, plan, options)
	}
	m.AddSubscription(plan, []string{"gra"}, false, false, "", nil, nil, true, 2, "", false, options, append(ids, ids[0]))
	m.CheckAvailabilityChange(m.subscriptions[0], "multi-first")
	counts := map[string]int{}
	for _, item := range state.Queue {
		counts[item.AccountID]++
		if item.AutoPay {
			t.Fatal("multi-account selection enabled auto-payment")
		}
		readyMultiCart(item.AccountID, plan, options)
		outcome := purchase.PurchaseServer(context.Background(), state, &item)
		if !outcome.Success {
			t.Fatalf("mock purchase failed: %+v", outcome)
		}
	}
	if counts[ids[0]] != 2 || counts[ids[1]] != 2 || len(state.Queue) != 4 || checkoutCalls.checkouts.Load() != 4 {
		t.Fatalf("expected two purchases per account: counts=%v queue=%d checkout=%d", counts, len(state.Queue), checkoutCalls.checkouts.Load())
	}
	if err := m.SaveToDB(); err != nil {
		t.Fatal(err)
	}
	// Remove completed tasks so queue dedupe cannot conceal a broken baseline.
	state.Queue = nil
	if err := state.DB.ReplaceQueue(nil); err != nil {
		t.Fatal(err)
	}
	restored := New(state)
	restored.LoadFromDB()
	restored.CheckAvailabilityChange(restored.subscriptions[0], "multi-after-restart")
	if len(state.Queue) != 0 {
		t.Fatalf("sustained stock re-ordered after restart: %d", len(state.Queue))
	}
	if len(restored.Snapshot()[0].AutoOrderAccountIDs) != 2 || len(restored.Snapshot()[0].LastStatus) != 2 {
		t.Fatal("restart lost account targets or isolated stock baselines")
	}
}

func TestMultiAccountFailureDoesNotSuppressHealthyAccount(t *testing.T) {
	plan := "multi-failure-plan"
	m, state, options, _ := multiAccountFixture(t, plan, "IT", false)
	a, b := state.Accounts[0].ID, state.Accounts[1].ID
	state.Accounts[1].ConsumerKey = ""
	readyMultiCart(a, plan, options)
	// Construct without launching background prewarm requests for invalid creds.
	m.subscriptions = []*Subscription{{PlanCode: plan, AutoOrder: true, Quantity: 1, AutoOrderAccountID: a, AutoOrderAccountIDs: []string{a, b}, Datacenters: []string{"gra"}, Options: options}}
	m.CheckAvailabilityChange(m.subscriptions[0], "one-account-fails")
	if len(state.Queue) != 1 || state.Queue[0].AccountID != a {
		t.Fatalf("healthy account was suppressed: %+v", state.Queue)
	}
	if !strings.Contains(m.Snapshot()[0].LastCheckError, b) {
		t.Fatal("failed account missing from diagnostics")
	}
	state.Accounts[1].ConsumerKey = "b"
	state.OVH.Invalidate(b)
	readyMultiCart(b, plan, options)
	m.CheckAvailabilityChange(m.subscriptions[0], "failed-account-recovers")
	if len(state.Queue) != 2 || state.Queue[0].AccountID == state.Queue[1].AccountID {
		t.Fatalf("recovered account did not order independently: %+v", state.Queue)
	}
}

func TestCustomPlanMonitorMatchesExplicitInventoryOptions(t *testing.T) {
	plan := "custom-not-in-catalog"
	m, state, options, stats := multiAccountFixture(t, plan, "BE", true)
	account := state.Accounts[0].ID
	// No prewarmed cart: exercise real price/cart/addon validation against the mock.
	m.subscriptions = []*Subscription{{PlanCode: plan, AutoOrder: true, Quantity: 1, AutoOrderAccountID: account, AutoOrderAccountIDs: []string{account}, Datacenters: []string{"gra"}, Options: options}}
	m.CheckAvailabilityChange(m.subscriptions[0], "custom-inventory")
	if len(state.Queue) != 1 || !reflect.DeepEqual(state.Queue[0].Options, options) {
		t.Fatalf("custom plan lost explicit options or failed to enqueue: %+v", state.Queue)
	}
	if !state.Queue[0].Force {
		t.Fatal("custom plan queue lost permission to bypass stale catalog")
	}
	if stats.carts.Load() != 1 || stats.options.Load() != 1 || stats.checkouts.Load() != 0 {
		t.Fatalf("monitor did not validate a fully configured live cart: carts=%d options=%d checkouts=%d", stats.carts.Load(), stats.options.Load(), stats.checkouts.Load())
	}
	item := state.Queue[0]
	// 验价通过的那辆车已留作预热车:下单直接拿它结账,不再建第二辆、也不重复加选项
	outcome := purchase.PurchaseServer(context.Background(), state, &item)
	if !outcome.Success || stats.carts.Load() != 1 || stats.options.Load() != 1 || stats.checkouts.Load() != 1 {
		t.Fatalf("custom plan checkout failed or hardware duplicated: outcome=%+v carts=%d options=%d checkouts=%d", outcome, stats.carts.Load(), stats.options.Load(), stats.checkouts.Load())
	}
	wrong := []string{"softraid-2x960nvme-extra-" + plan}
	if configMatchesFilter(wrong, catalog.CustomInventoryOptions(plan, []string{"softraid-2x960nvme"}, wrong)) {
		t.Fatal("partial hardware prefix matched a different configuration")
	}
}

func TestMultiAccountPauseAndAccountRemoval(t *testing.T) {
	m, state := setupTestMonitor(t)
	m.AddSubscription("plan", []string{"gra"}, false, false, "", nil, nil, false, 1, "", false, []string{"ram"}, []string{"a", "b"})
	state.Queue = []types.QueueItem{
		{ID: "a", AccountID: "a", PlanCode: "plan", Datacenter: "gra", Options: []string{"ram"}, Status: "running"},
		{ID: "b", AccountID: "b", PlanCode: "plan", Datacenter: "gra", Options: []string{"ram"}, Status: "running"},
		{ID: "c", AccountID: "c", PlanCode: "plan", Datacenter: "gra", Options: []string{"ram"}, Status: "running"},
	}
	if count, err := m.SetPaused("plan", nil, true); err != nil || count != 2 {
		t.Fatalf("pause did not cover both selected accounts: count=%d err=%v", count, err)
	}
	if state.Queue[2].Status != "running" {
		t.Fatal("pause affected an unrelated account")
	}
	if got := m.ClearAccountRefs("a"); got != 1 {
		t.Fatalf("account reference was not removed: %d", got)
	}
	if ids := m.Snapshot()[0].AutoOrderAccountIDs; !reflect.DeepEqual(ids, []string{"b"}) {
		t.Fatalf("removing one account discarded other targets: %v", ids)
	}
}

func TestFinalEnqueueGuardRejectsEditedAndRemovedSubscriptions(t *testing.T) {
	m, _ := setupTestMonitor(t)
	m.AddSubscription("plan", nil, false, false, "", nil, nil, false, 1, "", false, nil, []string{"a", "b"})
	sub := m.subscriptions[0]
	cfg := sub.checkConfig()
	commits := 0
	commit := func() error { commits++; return nil }
	if err := m.guardCheckEnqueue(sub, cfg, commit); err != nil || commits != 1 {
		t.Fatalf("current subscription cannot enqueue: %v", err)
	}
	if err := m.UpdateSubscriptionFields("plan", nil, nil, false, false, false, 1, "", false, nil, []string{"b"}); err != nil {
		t.Fatal(err)
	}
	if err := m.guardCheckEnqueue(sub, cfg, commit); err == nil || commits != 1 {
		t.Fatal("stale account selection reached the final queue commit")
	}
	cfg = sub.checkConfig()
	m.RemoveSubscription("plan")
	if err := m.guardCheckEnqueue(sub, cfg, commit); err == nil || commits != 1 {
		t.Fatal("deleted subscription reached the final queue commit")
	}
}
