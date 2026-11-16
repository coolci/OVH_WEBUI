package monitor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

func TestRegression_BUG_OVH_09_MultiAccountAlertBindsCorrectAccount(t *testing.T) {
	m, st := setupTestMonitor(t)
	t.Cleanup(func() { purchase.ClearPrewarmPool(st) })
	st.TelegramRepo = st.DB
	st.Accounts = []types.OVHAccount{
		{ID: "acc-eu", Name: "EU Primary", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
		{ID: "acc-us", Name: "US Secondary", Endpoint: "ovh-us", Zone: "US", AppKey: "k", AppSecret: "s", ConsumerKey: "c"},
	}

	euSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprint(w, `[]`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer euSrv.Close()

	usSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprint(w, `[{"fqn":"24ska01-custom.ram-32g.softraid-2x512nvme","memory":"ram-32g","storage":"softraid-2x512nvme","datacenters":[{"datacenter":"hil","availability":"1H-low"}]}]`)
		case r.Method == http.MethodPost && r.URL.Path == "/order/cart":
			fmt.Fprint(w, `{"cartId":"us-cart-1"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/eco"):
			fmt.Fprint(w, `{"itemId":1}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/eco/options"):
			fmt.Fprint(w, `[{"planCode":"ram-32g-24ska01-custom","prices":[{"duration":"P1M","pricingMode":"default"}]},{"planCode":"softraid-2x512nvme-24ska01-custom","prices":[{"duration":"P1M","pricingMode":"default"}]}]`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/eco/options"):
			fmt.Fprint(w, `{}`)
		case strings.HasSuffix(r.URL.Path, "/requiredConfiguration"):
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/summary"):
			fmt.Fprint(w, `{"prices":{"withoutTax":{"value":19.99,"currencyCode":"USD"},"withTax":{"value":19.99,"currencyCode":"USD"}}}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer usSrv.Close()

	swapMonitorEndpoint(t, "ovh-eu", euSrv.URL)
	swapMonitorEndpoint(t, "ovh-us", usSrv.URL)

	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			if strings.Contains(r.URL.RawQuery, "ovhSubsidiary=US") {
				body := `{"plans":[{"planCode":"24ska01-custom","configurations":[{"name":"region","values":["united_states"]}],"addonFamilies":[{"name":"memory","addons":["ram-32g-24ska01-custom"]},{"name":"storage","addons":["softraid-2x512nvme-24ska01-custom"]}]}]}`
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"plans":[]}`)), Request: r}, nil
		}
		return oldTransport.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	m.AddSubscription("24ska01-custom", []string{"hil"}, true, false, "KS-A", nil, nil, false, 0, "acc-eu", false, nil)
	if err := m.UpdateSubscriptionFields("24ska01-custom", nil, []string{"hil"}, true, false, false, 0, "acc-eu", false, nil, []string{"acc-eu", "acc-us"}); err != nil {
		t.Fatal(err)
	}

	// 1. Direct buildAvailabilityAlert with account_id / explicit choice.accountID ("acc-us")
	dcs := []map[string]interface{}{
		{"dc": "hil", "detected_time": time.Now().Format(time.RFC3339Nano), "raw_status": "1H-low"},
	}
	cfg := map[string]interface{}{
		"account_id":   "acc-us",
		"display":      "32G + 2x512GB NVMe",
		"cached_price": "US$19.99/月",
		"options":      []string{"ram-32g-24ska01-custom"},
	}
	_, markup := m.buildAvailabilityAlert("24ska01-custom", dcs, cfg, "KS-A", "", "trace-1", "cfg-1")

	rawJSON, err := json.Marshal(markup)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		InlineKeyboard [][]struct {
			Text         string `json:"text"`
			CallbackData string `json:"callback_data"`
		} `json:"inline_keyboard"`
	}
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatal(err)
	}

	var boundAccID, btnUUID string
	for _, row := range parsed.InlineKeyboard {
		for _, btn := range row {
			var cb map[string]string
			if json.Unmarshal([]byte(btn.CallbackData), &cb) == nil && cb["u"] != "" {
				if dbRow, ok, err := st.TelegramRepo.GetTelegramButton(cb["u"]); err == nil && ok {
					btnUUID = cb["u"]
					boundAccID = dbRow.AccountID
					break
				}
			}
		}
		if btnUUID != "" {
			break
		}
	}
	if boundAccID != "acc-us" {
		t.Fatalf("expected button to bind US account 'acc-us', got %q (uuid=%q)", boundAccID, btnUUID)
	}

	// 2. Full CheckAvailabilityChange end-to-end check on multi-account subscription ["acc-eu", "acc-us"]
	m.CheckAvailabilityChange(m.subscriptions[0], "trace-e2e")
	m.cacheLock.Lock()
	var e2eUUIDs []string
	for id := range m.messageUUIDCache {
		if id != btnUUID {
			e2eUUIDs = append(e2eUUIDs, id)
		}
	}
	m.cacheLock.Unlock()
	if len(e2eUUIDs) == 0 {
		t.Fatal("expected CheckAvailabilityChange to generate Telegram order button UUIDs for acc-us restock")
	}
	for _, u := range e2eUUIDs {
		dbRow, ok, err := st.TelegramRepo.GetTelegramButton(u)
		if err != nil || !ok {
			t.Fatalf("missing persisted Telegram button %s: ok=%v err=%v", u, ok, err)
		}
		if dbRow.AccountID != "acc-us" {
			t.Fatalf("expected CheckAvailabilityChange button %s to bind 'acc-us', got %q", u, dbRow.AccountID)
		}
	}
}

func TestRegression_BUG_OVH_15_VerifyPriceAvailableRejectsNegativeZeroNaNInfAndNonNumeric(t *testing.T) {
	m, st := setupTestMonitor(t)
	t.Cleanup(func() { purchase.ClearPrewarmPool(st) })
	st.Accounts = []types.OVHAccount{
		{ID: "acc-p", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
	}

	var returnWithoutTax string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/order/cart" && r.Method == http.MethodPost:
			fmt.Fprint(w, `{"cartId":"price-check-cart"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodPost:
			fmt.Fprint(w, `{"itemId":1}`)
		case strings.HasSuffix(r.URL.Path, "/requiredConfiguration"):
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/configuration"):
			fmt.Fprint(w, `{"id":1}`)
		case strings.HasSuffix(r.URL.Path, "/summary"):
			fmt.Fprintf(w, `{"prices":{"withoutTax":{"value":%s,"currencyCode":"EUR"}}}`, returnWithoutTax)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	swapMonitorEndpoint(t, "ovh-eu", srv.URL)

	priceCfg := map[string]interface{}{
		"account_id": "acc-p",
		"subsidiary": "IE",
		"duration":   "P1M",
		"auto_order": true,
	}

	cases := []struct {
		plan string
		val  string
	}{
		{"24ska01-neg", `-15.5`},
		{"24ska01-zero", `0`},
		{"24ska01-nan", `"NaN"`},
		{"24ska01-inf", `"+Inf"`},
		{"24ska01-ninf", `"-Inf"`},
		{"24ska01-str", `"not-a-number"`},
	}

	for _, tc := range cases {
		returnWithoutTax = tc.val
		ok, reason := m.verifyPriceAvailable(tc.plan, "gra", priceCfg)
		if ok {
			t.Fatalf("expected invalid price %s (%s) to be rejected by verifyPriceAvailable, but got ok=true", tc.val, tc.plan)
		}
		if reason == "" {
			t.Fatalf("expected non-empty failure reason for invalid price %s (%s)", tc.val, tc.plan)
		}
		if purchase.HasPrewarmedCart("acc-p", tc.plan, "gra", nil, "", "P1M") {
			t.Fatalf("invalid price %s (%s) must never enter prewarm_pool", tc.val, tc.plan)
		}
	}
}

func TestRegression_BUG_OVH_04_VerifyPriceAvailableRejectsUnassignedCart(t *testing.T) {
	m, st := setupTestMonitor(t)
	t.Cleanup(func() { purchase.ClearPrewarmPool(st) })
	st.Accounts = []types.OVHAccount{
		{ID: "acc-assign", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/order/cart" && r.Method == http.MethodPost:
			fmt.Fprint(w, `{"cartId":"unassigned-cart"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"message":"assign forbidden"}`)
		case strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodPost:
			fmt.Fprint(w, `{"itemId":1}`)
		case strings.HasSuffix(r.URL.Path, "/requiredConfiguration"):
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/summary"):
			fmt.Fprint(w, `{"prices":{"withoutTax":{"value":19.99,"currencyCode":"EUR"}}}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	swapMonitorEndpoint(t, "ovh-eu", srv.URL)

	ok, reason := m.verifyPriceAvailable("24ska01-unassigned", "gra", map[string]interface{}{
		"account_id": "acc-assign",
		"subsidiary": "IE",
		"duration":   "P1M",
		"auto_order": true,
	})
	if !ok {
		t.Fatalf("expected price quote itself to succeed while discarding unassigned cart, got ok=false (%q)", reason)
	}
	if purchase.HasPrewarmedCart("acc-assign", "24ska01-unassigned", "gra", nil, "", "P1M") {
		t.Fatal("unassigned cart must never enter prewarm_pool")
	}
}

func TestRegression_BUG_FE_CORE_01_DisplayDatacenterCodesNormalizedAndMatched(t *testing.T) {
	m, _ := setupTestMonitor(t)

	m.AddSubscription("24adv01-fe", []string{"mum", "tor", "PAR-A", "ynm", "yyz", "eu-west-par-a"}, true, false, "ADV-1", nil, nil, false, 0, "", false, nil)
	snap := m.FindSubscription("24adv01-fe")
	if snap == nil {
		t.Fatal("subscription not found")
	}
	wantDCs := []string{"mum", "tor", "par-a"}
	if !reflect.DeepEqual(snap.Datacenters, wantDCs) {
		t.Fatalf("expected deduplicated lowercase DCs preserving display form %v, got %v", wantDCs, snap.Datacenters)
	}

	if err := m.SaveToDB(); err != nil {
		t.Fatal(err)
	}
	m.LoadFromDB()
	reloaded := m.FindSubscription("24adv01-fe")
	if reloaded == nil || !reflect.DeepEqual(reloaded.Datacenters, wantDCs) {
		t.Fatalf("expected DCs after reload %v, got %+v", wantDCs, reloaded)
	}

	if !matchesMonitoredDC(reloaded.Datacenters, "ynm") {
		t.Fatal("expected legacy 'mum' monitored DC to match OVH inventory 'ynm'")
	}
	if !matchesMonitoredDC(reloaded.Datacenters, "ca-east-tor-a") {
		t.Fatal("expected legacy 'tor' monitored DC to match OVH inventory 'ca-east-tor-a'")
	}
	if !matchesMonitoredDC(reloaded.Datacenters, "eu-west-par-a") {
		t.Fatal("expected 'par-a' monitored DC to match OVH inventory 'eu-west-par-a'")
	}
	if matchesMonitoredDC([]string{"par-a"}, "eu-west-par-b") {
		t.Fatal("'par-a' must not match different AZ 'eu-west-par-b'")
	}
}

func swapMonitorEndpoint(t *testing.T, name, url string) {
	t.Helper()
	orig := ovhsdk.Endpoints[name]
	ovhsdk.Endpoints[name] = url
	t.Cleanup(func() { ovhsdk.Endpoints[name] = orig })
}
