package monitor

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

func installMockCatalogTransport(t *testing.T, handler func(r *http.Request) (status int, body string, handled bool)) {
	t.Helper()
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		host := r.URL.Hostname()
		if host == "127.0.0.1" || host == "localhost" {
			return oldTransport.RoundTrip(r)
		}
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			if handler != nil {
				if status, body, ok := handler(r); ok {
					return &http.Response{
						StatusCode: status,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    r,
					}, nil
				}
			}
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plans":[]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("test blocked external request: %s", r.URL)
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
}

func TestPlanCooldown_PlanNotFoundAndSubsidiaryIsolation(t *testing.T) {
	const plan = "27skle01-pnf"
	m, state := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(state)

	var cartCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprintf(w, `[
				{"fqn":"%s.ram-32g.ssd","memory":"ram-32g","storage":"ssd","datacenters":[{"datacenter":"gra","availability":"72H"},{"datacenter":"rbx","availability":"72H"}]},
				{"fqn":"%s.ram-64g.nvme","memory":"ram-64g","storage":"nvme","datacenters":[{"datacenter":"gra","availability":"72H"},{"datacenter":"rbx","availability":"72H"}]}
			]`, plan, plan)
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			cartCalls.Add(1)
			fmt.Fprintf(w, `{"cartId":"pnf-cart-%d"}`, cartCalls.Load())
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"message":"Plan code not found"}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(srv.Close)
	swapTestEndpoint(t, "ovh-eu", serverURL(srv))

	// NL 目录暂不收录该机型，让 OVH 建车阶段返回 400 Plan code not found
	installMockCatalogTransport(t, func(r *http.Request) (int, string, bool) {
		if strings.Contains(r.URL.RawQuery, "ovhSubsidiary=FR") {
			return 200, fmt.Sprintf(`{"plans":[{"planCode":%q,"pricings":[{"capacities":["renew"],"mode":"default","price":{"value":50,"currencyCode":"EUR"},"interval":1}],"configurations":[{"name":"region","values":["europe"]}]}]}`, plan), true
		}
		return 200, `{"plans":[]}`, true
	})

	accNL := "pnf-nl-" + uuid.NewString()
	accFR := "pnf-fr-" + uuid.NewString()
	state.Accounts = []types.OVHAccount{
		{ID: accNL, Name: "NL", Endpoint: "ovh-eu", Zone: "NL", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
		{ID: accFR, Name: "FR", Endpoint: "ovh-eu", Zone: "FR", AppKey: "k2", AppSecret: "s2", ConsumerKey: "c2"},
	}

	sub := &Subscription{
		PlanCode:            plan,
		AutoOrderAccountID:  accNL,
		AutoOrderAccountIDs: []string{accNL},
		Datacenters:         []string{"gra", "rbx"},
	}
	m.subscriptions = []*Subscription{sub}

	// Round 1: 2 configs × 2 DCs，目录未收录应单点首探即止，仅建车 1 次即触发 15 分钟冷却
	m.CheckAvailabilityChange(sub, "pnf-round-1")
	firstRoundCalls := cartCalls.Load()
	if firstRoundCalls != 1 {
		t.Fatalf("expected 1 cart call in round 1 after Plan code not found short-circuit, got %d", firstRoundCalls)
	}

	active, entry := m.isPlanInCooldownDetail(accNL, "NL", plan)
	if !active || entry == nil || entry.reason != cooldownPlanNotFound {
		t.Fatalf("expected cooldownPlanNotFound for (%s, NL, %s), got active=%v entry=%+v", accNL, plan, active, entry)
	}
	if rem := time.Until(entry.until); rem < 14*time.Minute || rem > 15*time.Minute+5*time.Second {
		t.Fatalf("expected ~15m first cooldown for Plan code not found, got %s", rem)
	}

	// Round 2: 同一账户同区必须跳过建车，不能再发新请求
	m.CheckAvailabilityChange(sub, "pnf-round-2")
	if cartCalls.Load() != firstRoundCalls {
		t.Fatalf("expected 0 new cart calls during cooldown, had %d now %d", firstRoundCalls, cartCalls.Load())
	}

	// GetPriceInfoText 处于冷却中也不应回退建车询价
	if txt := m.GetPriceInfoText(plan, "gra", map[string]interface{}{"account_id": accNL, "subsidiary": "NL"}); txt != "" {
		t.Fatalf("expected empty price text in cooldown, got %q", txt)
	}
	if cartCalls.Load() != firstRoundCalls {
		t.Fatalf("GetPriceInfoText bypassed cooldown and called /order/cart: before=%d after=%d", firstRoundCalls, cartCalls.Load())
	}

	// 不同账户/不同子公司必须完全隔离
	if m.isPlanInCooldown(accFR, "FR", plan) {
		t.Fatalf("expected (%s, FR, %s) NOT to inherit (%s, NL) cooldown", accFR, plan, accNL)
	}

	// 修改原账户 Zone（NL -> FR）应自动启用新子公司的 key，不受旧 NL 冷却阻挡
	state.Accounts[0].Zone = "FR"
	if m.isPlanInCooldown(accNL, "", plan) {
		t.Fatalf("expected changing account zone NL -> FR to isolate away old NL cooldown")
	}
	state.Accounts[0].Zone = "NL"

	// 验证退避递增：15m -> 30m -> 60m（上限 60m）并可被成功重置
	m.recordPlanVerifyFailure(accNL, "NL", plan, false, "Plan code not found")
	_, entry2 := m.isPlanInCooldownDetail(accNL, "NL", plan)
	if rem := time.Until(entry2.until); rem < 29*time.Minute || rem > 30*time.Minute+5*time.Second {
		t.Fatalf("expected 30m on 2nd failure, got %s", rem)
	}
	m.recordPlanVerifyFailure(accNL, "NL", plan, false, "Plan code not found")
	_, entry3 := m.isPlanInCooldownDetail(accNL, "NL", plan)
	if rem := time.Until(entry3.until); rem < 59*time.Minute || rem > 60*time.Minute+5*time.Second {
		t.Fatalf("expected 60m on 3rd failure, got %s", rem)
	}
	m.recordPlanVerifyFailure(accNL, "NL", plan, false, "Plan code not found")
	_, entry4 := m.isPlanInCooldownDetail(accNL, "NL", plan)
	if rem := time.Until(entry4.until); rem < 59*time.Minute || rem > 60*time.Minute+5*time.Second {
		t.Fatalf("expected 60m cap on 4th failure, got %s", rem)
	}

	m.recordPlanVerifySuccess(accNL, "NL", plan)
	if m.isPlanInCooldown(accNL, "NL", plan) {
		t.Fatal("expected recordPlanVerifySuccess to clear cooldown")
	}
}

func TestPlanCooldown_CatalogMissingShortCircuitAndAutoHeal(t *testing.T) {
	const plan = "27skle02-v1"
	m, state := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(state)

	var cartCalls atomic.Int32
	var ecoShouldPass atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			// 3 configs × 4 DCs = 12 个组合全显示 72H
			fmt.Fprintf(w, `[
				{"fqn":"%s.ram-32g.nvme1","memory":"ram-32g","storage":"nvme1","datacenters":[{"datacenter":"gra","availability":"72H"},{"datacenter":"rbx","availability":"72H"},{"datacenter":"sbg","availability":"72H"},{"datacenter":"bhs","availability":"72H"}]},
				{"fqn":"%s.ram-64g.nvme2","memory":"ram-64g","storage":"nvme2","datacenters":[{"datacenter":"gra","availability":"72H"},{"datacenter":"rbx","availability":"72H"},{"datacenter":"sbg","availability":"72H"},{"datacenter":"bhs","availability":"72H"}]},
				{"fqn":"%s.ram-128g.nvme3","memory":"ram-128g","storage":"nvme3","datacenters":[{"datacenter":"gra","availability":"72H"},{"datacenter":"rbx","availability":"72H"},{"datacenter":"sbg","availability":"72H"},{"datacenter":"bhs","availability":"72H"}]}
			]`, plan, plan, plan)
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			cartCalls.Add(1)
			fmt.Fprintf(w, `{"cartId":"cat-cart-%d"}`, cartCalls.Load())
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			if ecoShouldPass.Load() {
				fmt.Fprint(w, `{"itemId":42}`)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"message":"Product %s is not available"}`, plan)
		case strings.HasSuffix(r.URL.Path, "/summary"):
			fmt.Fprint(w, `{"prices":{"withoutTax":{"value":49.99,"currencyCode":"EUR"},"withTax":{"value":59.99,"currencyCode":"EUR"}}}`)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			fmt.Fprint(w, `{}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(srv.Close)
	swapTestEndpoint(t, "ovh-eu", serverURL(srv))

	installMockCatalogTransport(t, nil) // 返回空目录 -> CatalogMissing = true

	accSK := "cat-sk-" + uuid.NewString()
	state.Accounts = []types.OVHAccount{
		{ID: accSK, Name: "SK", Endpoint: "ovh-eu", Zone: "SK", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
	}

	sub := &Subscription{
		PlanCode:            plan,
		AutoOrder:           true,
		Quantity:            1,
		AutoOrderAccountID:  accSK,
		AutoOrderAccountIDs: []string{accSK},
		Datacenters:         []string{"gra", "rbx", "sbg", "bhs"},
		NotifyAvailable:     true,
		NotifyUnavailable:   true,
	}
	m.subscriptions = []*Subscription{sub}

	// Round 1: 12 个组合只允许探 1 个机房，失败后同轮其余 11 个组合全部熔断跳过
	m.CheckAvailabilityChange(sub, "cat-r1")
	if got := cartCalls.Load(); got != 1 {
		t.Fatalf("expected single-probe-first + intra-round circuit break to allow exactly 1 cart call across 3x4 combinations, got %d", got)
	}
	for _, h := range sub.snapshot().History {
		if h.ChangeType == "price_check_failed" || h.Status == "price_check_failed" {
			t.Fatalf("CatalogMissing cooldown/probe failure must not emit false price_check_failed alerts: %+v", h)
		}
	}
	if len(state.Queue) != 0 {
		t.Fatalf("expected 0 queued orders on failed prerelease verify, got %d", len(state.Queue))
	}

	active, entry := m.isPlanInCooldownDetail(accSK, "SK", plan)
	if !active || entry == nil || entry.reason != cooldownCatalogFail || !entry.wasMissingFromCatalog {
		t.Fatalf("expected cooldownCatalogFail with wasMissingFromCatalog=true, got active=%v entry=%+v", active, entry)
	}

	// Rounds 2 & 3: 保持 0 次新 /order/cart 请求
	m.CheckAvailabilityChange(sub, "cat-r2")
	m.CheckAvailabilityChange(sub, "cat-r3")
	if got := cartCalls.Load(); got != 1 {
		t.Fatalf("expected 0 additional cart calls in cooled-down rounds 2-3, got total %d", got)
	}

	// 模拟公开目录新近收录该机型（CatalogMissing 从 true 变为 false）→ 立即自愈清除冷却
	ecoShouldPass.Store(true)
	m.observePlanCatalogStatus(accSK, "SK", plan, true, false)
	if m.isPlanInCooldown(accSK, "SK", plan) {
		t.Fatal("expected observePlanCatalogStatus(inCatalog=true) to auto-heal missing-catalog cooldown immediately")
	}

	// 验证 CatalogMissing=true 触发 429 时会进入短退避（3m），而 CatalogMissing=false 触发 429 不应进入长冷却
	m.recordPlanVerifyFailure(accSK, "SK", "27-429-missing", true, "被 OVH 限流了(OVH 429)。原文:too many requests")
	active429, entry429 := m.isPlanInCooldownDetail(accSK, "SK", "27-429-missing")
	if !active429 || entry429.reason != cooldownCatalog429 {
		t.Fatalf("expected cooldownCatalog429 for CatalogMissing+429, got active=%v entry=%+v", active429, entry429)
	}
	m.recordPlanVerifyFailure(accSK, "SK", "24-429-listed", false, "被 OVH 限流了(OVH 429)。原文:too many requests")
	if m.isPlanInCooldown(accSK, "SK", "24-429-listed") {
		t.Fatal("listed plan (CatalogMissing=false) must NOT enter long cooldown on 429")
	}
}

func TestPlanCooldown_Consecutive504ThresholdAndReset(t *testing.T) {
	const plan = "24sys-504"
	m, state := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(state)

	var cartCalls atomic.Int32
	var force504 atomic.Bool
	force504.Store(true)
	var stock atomic.Value
	stock.Store(`[{"fqn":"24sys-504.ram-32g.ssd","memory":"ram-32g","storage":"ssd","datacenters":[{"datacenter":"gra","availability":"72H"}]}]`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			fmt.Fprint(w, stock.Load().(string))
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			cartCalls.Add(1)
			fmt.Fprintf(w, `{"cartId":"cart-504-%d"}`, cartCalls.Load())
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			if force504.Load() {
				w.WriteHeader(http.StatusGatewayTimeout)
				fmt.Fprint(w, `{"message":"Gateway Time-out"}`)
				return
			}
			fmt.Fprint(w, `{"itemId":99}`)
		case strings.HasSuffix(r.URL.Path, "/summary"):
			fmt.Fprint(w, `{"prices":{"withoutTax":{"value":29.99,"currencyCode":"EUR"},"withTax":{"value":35.99,"currencyCode":"EUR"}}}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(srv.Close)
	swapTestEndpoint(t, "ovh-eu", serverURL(srv))

	// 目录包含该机型（CatalogMissing = false）
	installMockCatalogTransport(t, func(r *http.Request) (int, string, bool) {
		return 200, fmt.Sprintf(`{"plans":[{"planCode":%q,"pricings":[{"capacities":["renew"],"mode":"default","price":{"value":29.99,"currencyCode":"EUR"},"interval":1}],"configurations":[{"name":"region","values":["europe"]}]}]}`, plan), true
	})

	accLT := "lt-504-" + uuid.NewString()
	state.Accounts = []types.OVHAccount{
		{ID: accLT, Name: "LT", Endpoint: "ovh-eu", Zone: "LT", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true},
	}

	// 1. 纯状态机契约：第 1 次 504 不冷却，非 504 打断后计数清零，连续第 2 次 504 才冷却，并支持递增和成功复位
	const unitPlan = "unit-504-plan"
	m.recordPlanVerifyFailure(accLT, "LT", unitPlan, false, "OVH 服务端出错了(OVH 504)。原文:Gateway Time-out")
	if m.isPlanInCooldown(accLT, "LT", unitPlan) {
		t.Fatal("single 504 must NOT activate long cooldown")
	}
	// 非 504 错误打断连续性
	m.recordPlanVerifyFailure(accLT, "LT", unitPlan, false, "被 OVH 限流了(OVH 429)。原文:too many requests")
	m.recordPlanVerifyFailure(accLT, "LT", unitPlan, false, "OVH 服务端出错了(OVH 504)。原文:Gateway Time-out")
	if m.isPlanInCooldown(accLT, "LT", unitPlan) {
		t.Fatal("non-consecutive 504 (reset by 429) must NOT activate cooldown after only 1 new 504")
	}
	// 连续第 2 次 504 → 进入 10m 冷却
	m.recordPlanVerifyFailure(accLT, "LT", unitPlan, false, "OVH 服务端出错了(OVH 504)。原文:Gateway Time-out")
	active, entry := m.isPlanInCooldownDetail(accLT, "LT", unitPlan)
	if !active || entry.reason != cooldownUpstream504 {
		t.Fatalf("expected cooldownUpstream504 on 2nd consecutive 504, got active=%v entry=%+v", active, entry)
	}
	if rem := time.Until(entry.until); rem < 9*time.Minute || rem > 10*time.Minute+5*time.Second {
		t.Fatalf("expected 10m cooldown on 1st 504 trip, got %s", rem)
	}
	m.recordPlanVerifyFailure(accLT, "LT", unitPlan, false, "OVH 服务端出错了(OVH 504)。原文:Gateway Time-out")
	_, entry2 := m.isPlanInCooldownDetail(accLT, "LT", unitPlan)
	if rem := time.Until(entry2.until); rem < 19*time.Minute || rem > 20*time.Minute+5*time.Second {
		t.Fatalf("expected 20m cooldown escalation on repeated 504, got %s", rem)
	}
	m.recordPlanVerifyFailure(accLT, "LT", unitPlan, false, "OVH 服务端出错了(OVH 504)。原文:Gateway Time-out")
	m.recordPlanVerifyFailure(accLT, "LT", unitPlan, false, "OVH 服务端出错了(OVH 504)。原文:Gateway Time-out")
	_, entry3 := m.isPlanInCooldownDetail(accLT, "LT", unitPlan)
	if rem := time.Until(entry3.until); rem < 29*time.Minute || rem > 30*time.Minute+5*time.Second {
		t.Fatalf("expected 30m cap for consecutive 504 cooldown, got %s", rem)
	}
	m.recordPlanVerifySuccess(accLT, "LT", unitPlan)
	if m.isPlanInCooldown(accLT, "LT", unitPlan) {
		t.Fatal("expected recordPlanVerifySuccess to clear 504 cooldown")
	}

	// 2. 端到端监控：Round 1 gra 504（不冷却） → Round 2 rbx 上线并 504（连续第 2 次 504，进入冷却）
	// → Round 3 sbg 上线必须被跳过、0 新建车且无"价格校验未执行"误报
	sub := &Subscription{
		PlanCode:            plan,
		AutoOrderAccountID:  accLT,
		AutoOrderAccountIDs: []string{accLT},
		Datacenters:         []string{"gra", "rbx", "sbg"},
		NotifyAvailable:     true,
		NotifyUnavailable:   true,
	}
	m.subscriptions = []*Subscription{sub}

	m.CheckAvailabilityChange(sub, "504-r1")
	if cartCalls.Load() != 1 || m.isPlanInCooldown(accLT, "LT", plan) {
		t.Fatalf("after 1st 504: cartCalls=%d (want 1), inCooldown=%v (want false)",
			cartCalls.Load(), m.isPlanInCooldown(accLT, "LT", plan))
	}

	stock.Store(`[{"fqn":"24sys-504.ram-32g.ssd","memory":"ram-32g","storage":"ssd","datacenters":[{"datacenter":"gra","availability":"72H"},{"datacenter":"rbx","availability":"72H"}]}]`)
	m.CheckAvailabilityChange(sub, "504-r2")
	if cartCalls.Load() != 2 {
		t.Fatalf("expected 2nd cart call for rbx in round 2, got %d", cartCalls.Load())
	}
	if !m.isPlanInCooldown(accLT, "LT", plan) {
		t.Fatal("expected 2nd consecutive 504 across monitor rounds to enter cooldownUpstream504")
	}

	// Round 3: 新机房 sbg 也显示有货，但由于仍在 504 冷却期，应跳过建车且不出现"价格校验未执行"
	stock.Store(`[{"fqn":"24sys-504.ram-32g.ssd","memory":"ram-32g","storage":"ssd","datacenters":[{"datacenter":"gra","availability":"72H"},{"datacenter":"rbx","availability":"72H"},{"datacenter":"sbg","availability":"72H"}]}]`)
	m.CheckAvailabilityChange(sub, "504-r3")
	if cartCalls.Load() != 2 {
		t.Fatalf("expected 0 new cart calls for sbg during 504 cooldown, got total %d", cartCalls.Load())
	}
	for _, h := range sub.snapshot().History {
		if h.Datacenter == "sbg" && (h.ChangeType == "price_check_failed" || h.Status == "price_check_failed") {
			t.Fatalf("skipped DC in cooldown must not emit false price_check_failed history: %+v", h)
		}
	}
}

func TestPrewarm_SkipsCatalogMissingCooldownAndFeedsBackFailures(t *testing.T) {
	m, state := setupTestMonitor(t)
	defer purchase.ClearPrewarmPool(state)
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
			fmt.Fprintf(w, `{"cartId":"pw-cart-%d"}`, cartCalls.Load())
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/eco"):
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"message":"Plan code not found"}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(srv.Close)
	swapTestEndpoint(t, "ovh-eu", serverURL(srv))

	const unlistedEco = "27sysle03-v1"
	const pnfEco = "27rise01-v1"

	installMockCatalogTransport(t, func(r *http.Request) (int, string, bool) {
		if strings.Contains(r.URL.RawQuery, "ovhSubsidiary=PL") {
			return 200, fmt.Sprintf(`{"plans":[{"planCode":%q,"configurations":[{"name":"region","values":["europe"]}]}]}`, pnfEco), true
		}
		// IT 目录返回空，模拟目录未收录的 27* 机型
		return 200, `{"plans":[]}`, true
	})

	accIT := "pw-it-" + uuid.NewString()
	accPL := "pw-pl-" + uuid.NewString()
	accCZ := "pw-cz-" + uuid.NewString()
	state.Accounts = []types.OVHAccount{
		{ID: accIT, Name: "IT", Endpoint: "ovh-eu", Zone: "IT", AppKey: "k1", AppSecret: "s1", ConsumerKey: "c1", IsDefault: true},
		{ID: accPL, Name: "PL", Endpoint: "ovh-eu", Zone: "PL", AppKey: "k2", AppSecret: "s2", ConsumerKey: "c2"},
		{ID: accCZ, Name: "CZ", Endpoint: "ovh-eu", Zone: "CZ", AppKey: "k3", AppSecret: "s3", ConsumerKey: "c3"},
	}

	// 1. Eco 未收录机型在 TriggerPrewarmForSubscription 中应直接跳过，3 轮 × 3 机房 = 0 次建车
	subUnlisted := &Subscription{
		PlanCode:            unlistedEco,
		AutoOrder:           true,
		AutoOrderAccountID:  accIT,
		AutoOrderAccountIDs: []string{accIT},
		Datacenters:         []string{"gra", "rbx", "sbg"},
	}
	m.TriggerPrewarmForSubscription(subUnlisted)
	m.TriggerPrewarmForSubscription(subUnlisted)
	m.TriggerPrewarmForSubscription(subUnlisted)
	time.Sleep(80 * time.Millisecond)
	if got := cartCalls.Load(); got != 0 {
		t.Fatalf("expected 0 prewarm cart calls for catalog-missing Eco %s, got %d", unlistedEco, got)
	}

	// 2. 预热装车直接遭遇 400 Plan code not found 后应反向写入共享冷却，
	// 随后的 TriggerPrewarmForSubscription 与 prewarmAlertButtons 均应整机跳过
	m.assemblePrewarmedCartAsync(accPL, pnfEco, "gra", []string{}, "", "")
	if got := cartCalls.Load(); got != 1 {
		t.Fatalf("expected 1 cart call on first failing assemblePrewarmedCartAsync, got %d", got)
	}
	if !m.isPlanInCooldown(accPL, "PL", pnfEco) {
		t.Fatalf("expected prewarm Plan code not found failure to activate cooldown for (%s, PL, %s)", accPL, pnfEco)
	}

	subPNF := &Subscription{
		PlanCode:            pnfEco,
		AutoOrder:           true,
		AutoOrderAccountID:  accPL,
		AutoOrderAccountIDs: []string{accPL},
		Datacenters:         []string{"gra", "rbx", "sbg"},
	}
	m.TriggerPrewarmForSubscription(subPNF)
	n := m.prewarmAlertButtons(pnfEco, []map[string]interface{}{{"dc": "gra"}, {"dc": "rbx"}}, map[string]interface{}{}, accPL)
	time.Sleep(80 * time.Millisecond)
	if n != 0 {
		t.Fatalf("expected prewarmAlertButtons to spawn 0 tasks for cooled-down account, spawned %d", n)
	}
	if got := cartCalls.Load(); got != 1 {
		t.Fatalf("expected 0 new cart calls after prewarm cooldown activation, got total %d", got)
	}

	// 3. 验证跨账号隔离：accPL 被冷却时，accCZ 不受影响仍能触发通知按钮预热
	m.recordPlanVerifyFailure(accPL, "PL", "24sk-split", false, "Plan code not found")
	nSplit := m.prewarmAlertButtons("24sk-split", []map[string]interface{}{{"dc": "gra"}}, map[string]interface{}{}, accCZ)
	time.Sleep(80 * time.Millisecond)
	if nSplit != 1 {
		t.Fatalf("expected non-cooled account %s to still prewarm alert button (n=1), got %d", accCZ, nSplit)
	}
}

func serverURL(srv *httptest.Server) string {
	return srv.URL
}
