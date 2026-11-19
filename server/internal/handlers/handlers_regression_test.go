package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

type regressionRoundTripFunc func(*http.Request) (*http.Response, error)

func (f regressionRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// BUG-BE-03: Proxmox 9 + ZFS preset respects defaultDiskGroupId (never overwritten by diskGroups[0]),
// blocks invalid zfs+raid10 and insufficient physical disk counts, and sets /boot ext4 raidLevel safely.
func TestRegression_BUG_BE_03_Proxmox9ZfsDiskGroupAndRaidValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)
	st.Accounts = []types.OVHAccount{
		{ID: "eu", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "ak", AppSecret: "as", ConsumerKey: "ck", IsDefault: true},
	}

	var diskCountForGroup2 atomic.Int32
	diskCountForGroup2.Store(2)

	var reinstallPayloads []map[string]interface{}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case strings.HasSuffix(r.URL.Path, "/specifications/hardware"):
			fmt.Fprintf(w, `{
				"defaultDiskGroupId": 2,
				"diskGroups": [
					{"diskGroupId": 1, "numberOfDisks": 4, "diskSize": {"value": 8000, "unit": "GB"}},
					{"diskGroupId": 2, "numberOfDisks": %d, "diskSize": {"value": 480, "unit": "GB"}}
				]
			}`, diskCountForGroup2.Load())
		case strings.HasSuffix(r.URL.Path, "/reinstall") && r.Method == http.MethodPost:
			var p map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&p)
			mu.Lock()
			reinstallPayloads = append(reinstallPayloads, p)
			mu.Unlock()
			fmt.Fprint(w, `{"taskId": 1001, "status": "init"}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = orig }()

	r := gin.New()
	r.POST("/install/:service_name", InstallOS(st))

	// 1. zfsRaidLevel = 10 -> ZFS does NOT support RAID 10, must reject with 400!
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/install/ns1.ip-1-2-3.eu", bytes.NewBufferString(`{"templateName":"proxmox9_64","useProxmox9Zfs":true,"zfsRaidLevel":10,"zfsVzSize":10240}`))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for zfs+raid10, got %d: %s", w1.Code, w1.Body.String())
	}

	// 2. zfsRaidLevel = 7 on 2-disk server -> RAID 7 requires >= 5 disks, must reject with 400!
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/install/ns2.ip-1-2-3.eu", bytes.NewBufferString(`{"templateName":"proxmox9_64","useProxmox9Zfs":true,"zfsRaidLevel":7,"zfsVzSize":10240}`))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for raid7 on 2-disk group, got %d: %s", w2.Code, w2.Body.String())
	}

	// 3. Valid zfsRaidLevel = 1 on mixed-disk server (defaultDiskGroupId=2 vs diskGroups[0]=1) -> must target diskGroupId=2!
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/install/ns3.ip-1-2-3.eu", bytes.NewBufferString(`{"templateName":"proxmox9_64","useProxmox9Zfs":true,"zfsRaidLevel":1,"zfsVzSize":10240}`))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid zfs+raid1, got %d: %s", w3.Code, w3.Body.String())
	}

	// 4. Valid zfsRaidLevel = 7 on 5-disk group -> /boot ext4 must use raidLevel:1 (never raidLevel:7)!
	diskCountForGroup2.Store(5)
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodPost, "/install/ns4.ip-1-2-3.eu", bytes.NewBufferString(`{"templateName":"proxmox9_64","useProxmox9Zfs":true,"zfsRaidLevel":7,"zfsVzSize":10240}`))
	req4.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid zfs+raid7 on 5 disks, got %d: %s", w4.Code, w4.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(reinstallPayloads) != 2 {
		t.Fatalf("expected exactly 2 valid reinstall payloads, got %d", len(reinstallPayloads))
	}
	b1, _ := json.Marshal(reinstallPayloads[0]["storage"])
	if !strings.Contains(string(b1), `"diskGroupId":2`) || strings.Contains(string(b1), `"diskGroupId":1`) {
		t.Fatalf("expected defaultDiskGroupId=2 preserved (never overwritten by diskGroups[0]=1), got: %s", string(b1))
	}
	b2, _ := json.Marshal(reinstallPayloads[1]["storage"])
	if strings.Contains(string(b2), `"fileSystem":"ext4","mountPoint":"/boot","raidLevel":7`) ||
		!strings.Contains(string(b2), `"fileSystem":"ext4","mountPoint":"/boot","raidLevel":1`) {
		t.Fatalf("expected /boot ext4 to use raidLevel:1 instead of illegal raidLevel:7, got: %s", string(b2))
	}
}

// BUG-SEC-04 & MoveIP Guard: Path/query injection blocked with 400 and 0 upstream calls; MoveIP conflicting body.To rejected.
func TestRegression_BUG_SEC_04_OVHPathQuerySafetyAndMoveIPGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)
	st.Accounts = []types.OVHAccount{
		{ID: "eu", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "ak", AppSecret: "as", ConsumerKey: "ck", IsDefault: true},
	}

	var seenPaths []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenPaths = append(seenPaths, r.Method+" "+r.URL.RequestURI())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = orig }()

	r := gin.New()
	r.GET("/vps-control/:service_name/mitigation", GetVpsMitigation(st))
	r.POST("/vps-control/:service_name/mitigation/:ip", EnableVpsMitigation(st))
	r.GET("/server-control/:service_name/tasks/:task_id/available-timeslots", GetTaskAvailableTimeslots(st))
	r.DELETE("/server-control/:service_name/backup-ftp/access", DeleteBackupFTPAccess(st))
	r.DELETE("/server-control/:service_name/reverse/:ip", DeleteReverseDNS(st))
	r.POST("/server-control/:service_name/ip/move", MoveIP(st))

	cases := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{"vps mitigation injected block", http.MethodPost, "/vps-control/vps-1/mitigation/1.2.3.4?block=1.2.3.4/32%3Fevil%3D1", ""},
		{"vps mitigation bad service_name", http.MethodPost, "/vps-control/vps..evil/mitigation/1.2.3.4", ""},
		{"vps get mitigation query in service_name", http.MethodGet, "/vps-control/vps%3Fevil%3D1/mitigation", ""},
		{"task timeslots non-numeric traversal id", http.MethodGet, "/server-control/ns1/tasks/123..evil/available-timeslots?periodStart=2026-01-01&periodEnd=2026-01-02", ""},
		{"backup ftp access dotdot ipBlock", http.MethodDelete, "/server-control/ns1/backup-ftp/access?ipBlock=..", ""},
		{"reverse dns invalid ip", http.MethodDelete, "/server-control/ns1/reverse/1.2.3.4%3Fevil?ipBlock=../../me", ""},
		{"move ip conflicting body.to", http.MethodPost, "/server-control/ns-source.ip-1-2-3.eu/ip/move", `{"ip":"1.2.3.4","to":"ns-target.ip-1-2-3.eu"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mu.Lock()
			before := len(seenPaths)
			mu.Unlock()

			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = bytes.NewBufferString(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.url, bodyReader)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			mu.Lock()
			after := len(seenPaths)
			mu.Unlock()

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected HTTP 400, got %d: %s", w.Code, w.Body.String())
			}
			if after != before {
				t.Fatalf("expected 0 upstream OVH requests, got %d (%v)", after-before, seenPaths[before:])
			}
		})
	}
}

// BUG-BE-05, BUG-BE-06, BUG-BE-10: Telegram HUD RLock, ActiveAccountID unification, UTF-8 rune truncation & <=64B callback_data.
func TestRegression_BUG_BE_05_06_10_TelegramFixes(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	st.Accounts = []types.OVHAccount{
		{ID: "eu-default", Name: "EU Default", Endpoint: "ovh-eu", Zone: "IE", AppKey: "ak", AppSecret: "as", ConsumerKey: "ck", IsDefault: true},
		{ID: "us-active", Name: "US Active", Endpoint: "ovh-us", Zone: "US", AppKey: "ak", AppSecret: "as", ConsumerKey: "ck", IsDefault: false},
	}

	// 1. BUG-BE-06: Active Telegram account "us-active" takes precedence over global IsDefault "eu-default".
	if err := telegram.SetActiveAccount(st, "us-active"); err != nil {
		t.Fatal(err)
	}
	var queriedEndpoints []string
	var mu sync.Mutex
	srvEU := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queriedEndpoints = append(queriedEndpoints, "EU:"+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srvEU.Close()
	srvUS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queriedEndpoints = append(queriedEndpoints, "US:"+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		fmt.Fprint(w, `[{"fqn":"24ska01-us.ram-32g","datacenters":[{"datacenter":"hil","availability":"1H-low"}]}]`)
	}))
	defer srvUS.Close()
	origEU, origUS := ovhsdk.Endpoints["ovh-eu"], ovhsdk.Endpoints["ovh-us"]
	ovhsdk.Endpoints["ovh-eu"], ovhsdk.Endpoints["ovh-us"] = srvEU.URL, srvUS.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"], ovhsdk.Endpoints["ovh-us"] = origEU, origUS }()

	restoreNet := netfp.SwapSharedTransportForTesting(regressionRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.String(), srvEU.URL) || strings.HasPrefix(r.URL.String(), srvUS.URL) {
			return http.DefaultTransport.RoundTrip(r)
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"plans":[]}`)),
			Request:    r,
		}, nil
	}))
	defer restoreNet()

	_, stockWhenEmptyAcc := validAndInStockDCs(st, "24ska01-us", "")
	mu.Lock()
	gotEndpoints := append([]string{}, queriedEndpoints...)
	mu.Unlock()
	if len(stockWhenEmptyAcc) != 1 || stockWhenEmptyAcc[0] != "hil" {
		t.Fatalf("expected US stock [hil] via active account us-active, got %v (endpoints=%v)", stockWhenEmptyAcc, gotEndpoints)
	}
	for _, ep := range gotEndpoints {
		if strings.HasPrefix(ep, "EU:") {
			t.Fatalf("expected validAndInStockDCs to query US active account only, got %v", gotEndpoints)
		}
	}

	// 2. BUG-BE-10 (a): truncate() slices by []rune so multi-byte UTF-8 Chinese/Emoji is never corrupted.
	gotTrunc := truncate("账户余额不足无法下单", 5)
	if !utf8.ValidString(gotTrunc) || gotTrunc != "账户余额不" {
		t.Fatalf("expected valid UTF-8 '账户余额不', got %q (valid=%v)", gotTrunc, utf8.ValidString(gotTrunc))
	}

	// 3. BUG-BE-10 (b): Long custom/enterprise planCodes are shortened so callback_data is strictly <= 64 bytes and round-trips!
	longPlan1 := "25scale-amd-epyc-9654-custom-enterprise-edition-v1-us-west"
	longPlan2 := "25scale-amd-epyc-9654-custom-enterprise-edition-v2-us-west"
	ref1 := wizardPlanCallbackRef(st, longPlan1)
	ref2 := wizardPlanCallbackRef(st, longPlan2)
	cb1 := "i:D:t:b:" + ref1 + ":us-west-or-1"
	cb2 := "i:D:t:b:" + ref2 + ":us-west-or-1"
	if len(cb1) > 64 || len(cb2) > 64 {
		t.Fatalf("expected callback_data <= 64 bytes, got len(cb1)=%d (%q), len(cb2)=%d (%q)", len(cb1), cb1, len(cb2), cb2)
	}
	if ref1 == ref2 {
		t.Fatalf("expected distinct short refs for colliding long prefixes, got %q", ref1)
	}
	// Simulate restart by clearing in-memory shortToID map -> SQLite resolves both!
	shortMu.Lock()
	shortToID = map[string]string{}
	shortMu.Unlock()
	if resolvePlanCallbackRef(st, ref1) != longPlan1 || resolvePlanCallbackRef(st, ref2) != longPlan2 {
		t.Fatalf("failed to round-trip long planCodes after restart: got %q and %q",
			resolvePlanCallbackRef(st, ref1), resolvePlanCallbackRef(st, ref2))
	}

	// 4. BUG-BE-05: Concurrent hudDashboardMessage + Accounts slice replacement under AccountsMu.
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			msg := hudDashboardMessage(st, mon)
			if !utf8.ValidString(msg) {
				t.Errorf("invalid UTF-8 in hudDashboardMessage")
			}
		}()
		go func(iter int) {
			defer wg.Done()
			st.AccountsMu.Lock()
			st.Accounts = []types.OVHAccount{
				{ID: fmt.Sprintf("eu-%d", iter), Name: "EU", Endpoint: "ovh-eu", Zone: "IE", IsDefault: true},
			}
			st.AccountsMu.Unlock()
		}(i)
	}
	wg.Wait()
}

// BUG-BE-09: EnableVpsMitigation & DisableVpsMitigation return structured 410 Gone without calling retired OVH APIs.
func TestRegression_BUG_BE_09_VpsMitigationReturns410Gone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)
	st.Accounts = []types.OVHAccount{
		{ID: "eu", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "ak", AppSecret: "as", ConsumerKey: "ck", IsDefault: true},
	}

	var hitUpstream atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitUpstream.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = orig }()

	r := gin.New()
	r.POST("/server-control/:service_name/mitigation/:ip", EnableMitigation(st))
	r.DELETE("/server-control/:service_name/mitigation/:ip", DisableMitigation(st))
	r.POST("/vps-control/:service_name/mitigation/:ip", EnableVpsMitigation(st))
	r.DELETE("/vps-control/:service_name/mitigation/:ip", DisableVpsMitigation(st))

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/server-control/ns1.ip-1-2-3.eu/mitigation/1.2.3.4?block=1.2.3.4%2F32"},
		{http.MethodDelete, "/server-control/ns1.ip-1-2-3.eu/mitigation/1.2.3.4?block=1.2.3.4%2F32"},
		{http.MethodPost, "/vps-control/vps-abc123.vps.ovh.net/mitigation/1.2.3.4?block=1.2.3.4%2F32"},
		{http.MethodDelete, "/vps-control/vps-abc123.vps.ovh.net/mitigation/1.2.3.4?block=1.2.3.4%2F32"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != http.StatusGone {
			t.Fatalf("%s %s: expected 410 Gone, got %d: %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "PERMANENT_MITIGATION_REMOVED") {
			t.Fatalf("%s %s: missing PERMANENT_MITIGATION_REMOVED code: %s", tc.method, tc.path, w.Body.String())
		}
	}
	if hitUpstream.Load() != 0 {
		t.Fatalf("expected 0 upstream OVH calls for retired permanent mitigation endpoints, got %d", hitUpstream.Load())
	}
}

// BUG-BE-12 & BUG-BE-13: Immediate credential-fingerprint cache invalidation + distinct 404/400/412 account errors.
func TestRegression_BUG_BE_12_13_CacheInvalidationAndExplicitAccountErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestState(t)
	acc := types.OVHAccount{ID: "acc-cache", Name: "EU1", Endpoint: "ovh-eu", Zone: "IE", AppKey: "ak1", AppSecret: "as1", ConsumerKey: "ck1", IsDefault: true}
	if err := st.DB.UpsertAccount(acc); err != nil {
		t.Fatal(err)
	}
	_ = st.ReloadAccounts()

	var currentLabel atomic.Value
	currentLabel.Store("Visa *1111 (Old Credential)")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case strings.HasPrefix(r.URL.Path, "/me/payment/method") && r.URL.RawQuery == "default=true":
			fmt.Fprint(w, `[42]`)
		case r.URL.Path == "/me/payment/method/42":
			fmt.Fprintf(w, `{"paymentMethodId":42,"paymentType":"CREDIT_CARD","label":%q,"status":"VALID","default":true}`, currentLabel.Load().(string))
		default:
			fmt.Fprint(w, `{"nichandle":"ab12345-ovh","ovhSubsidiary":"IE"}`)
		}
	}))
	defer srv.Close()
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = orig }()

	r := gin.New()
	r.GET("/payment-default", GetDefaultPayment(st))
	r.PUT("/accounts/:id", UpdateAccount(st))
	r.GET("/aliases", ListServerAliases(st))
	r.GET("/tasks/:service_name", GetServerTasks(st))

	// 1. Populate defaultPaymentCache
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/payment-default?account=acc-cache", nil))
	if w1.Code != http.StatusOK || !strings.Contains(w1.Body.String(), "Visa *1111") {
		t.Fatalf("initial payment-default failed: %d %s", w1.Code, w1.Body.String())
	}

	// 2. Rotate account credentials via PUT /accounts/acc-cache -> fingerprint changes, invalidating old cache in 0ms!
	currentLabel.Store("Mastercard *9999 (New Credential)")
	wUpd := httptest.NewRecorder()
	reqUpd := httptest.NewRequest(http.MethodPut, "/accounts/acc-cache", bytes.NewBufferString(`{"appKey":"ak2","appSecret":"as2","consumerKey":"ck2"}`))
	reqUpd.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpd, reqUpd)
	if wUpd.Code != http.StatusOK {
		t.Fatalf("update account failed: %d %s", wUpd.Code, wUpd.Body.String())
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/payment-default?account=acc-cache", nil))
	if w2.Code != http.StatusOK || !strings.Contains(w2.Body.String(), "Mastercard *9999") {
		t.Fatalf("expected fresh Mastercard *9999 after credential rotation, got: %d %s", w2.Code, w2.Body.String())
	}

	// 3. BUG-BE-13 (a): Explicit nonexistent ?account=deleted-id returns 404 ACCOUNT_NOT_FOUND (not 412 NO_OVH_ACCOUNT!)
	wAlias := httptest.NewRecorder()
	r.ServeHTTP(wAlias, httptest.NewRequest(http.MethodGet, "/aliases?account=deleted-id", nil))
	if wAlias.Code != http.StatusNotFound || !strings.Contains(wAlias.Body.String(), "ACCOUNT_NOT_FOUND") {
		t.Fatalf("expected 404 ACCOUNT_NOT_FOUND for nonexistent account, got %d: %s", wAlias.Code, wAlias.Body.String())
	}

	// 4. BUG-BE-13 (b): Malformed ProxyURL returns 400 ACCOUNT_PROXY_INVALID (not 412 NO_OVH_ACCOUNT!)
	accBadProxy := types.OVHAccount{ID: "acc-badproxy", Name: "BadProxy", Endpoint: "ovh-eu", Zone: "IE", AppKey: "ak", AppSecret: "as", ConsumerKey: "ck", ProxyURL: "ftp://unsupported-scheme:8080"}
	_ = st.DB.UpsertAccount(accBadProxy)
	_ = st.ReloadAccounts()
	wTasks := httptest.NewRecorder()
	r.ServeHTTP(wTasks, httptest.NewRequest(http.MethodGet, "/tasks/ns1.ip-1-2-3.eu?account=acc-badproxy", nil))
	if wTasks.Code != http.StatusBadRequest || !strings.Contains(wTasks.Body.String(), "ACCOUNT_PROXY_INVALID") {
		t.Fatalf("expected 400 ACCOUNT_PROXY_INVALID for malformed proxy URL, got %d: %s", wTasks.Code, wTasks.Body.String())
	}

	// 5. BUG-BE-13 (c): Genuinely 0 configured OVH accounts still returns 412 NO_OVH_ACCOUNT!
	emptyState := newTestState(t)
	rEmpty := gin.New()
	rEmpty.GET("/aliases", ListServerAliases(emptyState))
	wEmpty := httptest.NewRecorder()
	rEmpty.ServeHTTP(wEmpty, httptest.NewRequest(http.MethodGet, "/aliases", nil))
	if wEmpty.Code != http.StatusPreconditionFailed || !strings.Contains(wEmpty.Body.String(), "NO_OVH_ACCOUNT") {
		t.Fatalf("expected 412 NO_OVH_ACCOUNT when no accounts exist, got %d: %s", wEmpty.Code, wEmpty.Body.String())
	}
}

// BUG-OVH-12 & BUG-OVH-16: Prewarm singleflight deduplication + DC normalization, and per-bucket stampPlanFirstSeen isolation.
func TestRegression_BUG_OVH_12_16_PrewarmDedupeAndFirstSeenIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, mon := newWatchTestMonitor(t)
	purchase.ClearPrewarmPool(st)
	defer purchase.ClearPrewarmPool(st)

	st.Accounts = []types.OVHAccount{
		{ID: "eu", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "ak", AppSecret: "as", ConsumerKey: "ck", IsDefault: true},
	}

	// REJECTED-01 preservation check: FindSubscription returns deep copy snapshot
	mon.AddSubscription("24ska01", []string{"gra"}, true, false, "KS-A", nil, nil, false, 0, "eu", false, nil)
	snapSub := mon.FindSubscription("24ska01")
	if snapSub == nil {
		t.Fatal("expected subscription snapshot")
	}
	snapSub.LastStatus["gra|default"] = "mutated-by-caller"
	if mon.FindSubscription("24ska01").LastStatus["gra|default"] == "mutated-by-caller" {
		t.Fatal("FindSubscription unexpectedly exposed live pointer instead of s.snapshot()")
	}

	// 1. BUG-OVH-12: 6 concurrent POST /api/purchase/prewarm calls (mixing "GRA" and "gra") coalesce into 1 cart creation!
	var cartPostCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.Method == http.MethodPost && r.URL.Path == "/order/cart":
			n := cartPostCount.Add(1)
			time.Sleep(60 * time.Millisecond)
			fmt.Fprintf(w, `{"cartId":"coalesced-cart-%d"}`, n)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco"):
			fmt.Fprint(w, `{"itemId":1}`)
		case strings.Contains(r.URL.Path, "/configuration"):
			fmt.Fprint(w, `{"id":1}`)
		default:
			fmt.Fprint(w, `[]`)
		}
	}))
	defer srv.Close()
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = orig }()

	restoreNet := netfp.SwapSharedTransportForTesting(regressionRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.String(), srv.URL) {
			return http.DefaultTransport.RoundTrip(r)
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24ska01","configurations":[{"name":"dedicated_region","values":["europe"]}]}]}`)),
			Request:    r,
		}, nil
	}))
	defer restoreNet()

	r := gin.New()
	r.POST("/prewarm", PrewarmOrder(st))

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		dcVal := "gra"
		if i%2 == 1 {
			dcVal = "GRA"
		}
		go func(dc string) {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/prewarm", bytes.NewBufferString(fmt.Sprintf(`{"account_id":"eu","planCode":"24ska01","datacenter":%q}`, dc)))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("unexpected prewarm status %d: %s", w.Code, w.Body.String())
			}
		}(dcVal)
	}
	wg.Wait()

	// Wait for the single background prewarm goroutine to finish
	time.Sleep(180 * time.Millisecond)
	if got := cartPostCount.Load(); got != 1 {
		t.Fatalf("expected singleflight deduplication to create 1 OVH cart, got %d", got)
	}

	// 2. BUG-OVH-16: Per-bucket isolation in stampPlanFirstSeen prevents cross-subsidiary baseline poisoning!
	stFS := newTestState(t)
	mkPlans := func(codes ...string) []types.ServerPlan {
		out := make([]types.ServerPlan, len(codes))
		for i, c := range codes {
			out[i] = types.ServerPlan{PlanCode: c}
		}
		return out
	}
	_ = stampPlanFirstSeen(stFS, "sub:IE", mkPlans("24ska01"))
	gotFR := stampPlanFirstSeen(stFS, "sub:FR", mkPlans("24ska01", "26newlaunch"))
	if gotFR["26newlaunch"] != 0 {
		t.Fatalf("expected sub:FR initial baseline to mark 26newlaunch as 0, got %d", gotFR["26newlaunch"])
	}
	gotIE := stampPlanFirstSeen(stFS, "sub:IE", mkPlans("24ska01", "26newlaunch"))
	if gotIE["26newlaunch"] <= 0 {
		t.Fatalf("expected sub:IE post-baseline refresh to detect 26newlaunch as >0, got %d", gotIE["26newlaunch"])
	}
}

// BUG-CONTRACT-01, 03, 04, 05: 100% bidirectional route & DTO contract alignment between live Gin router and api/openapi.yaml.
func TestOpenAPIRouteAndContractParity(t *testing.T) {
	backendDir := filepath.Clean("../..")
	openapiPath := filepath.Clean("../../../api/openapi.yaml")

	cmd := exec.Command("go", "run", "./cmd/routes")
	cmd.Dir = backendDir
	rawOut, err := cmd.Output()
	if err != nil {
		t.Fatalf("export live backend routes via cmd/routes: %v", err)
	}
	var backendRoutes []struct {
		Method  string `json:"method"`
		Path    string `json:"path"`
		Handler string `json:"handler"`
	}
	if err := json.Unmarshal(bytes.TrimPrefix(rawOut, []byte("\xef\xbb\xbf")), &backendRoutes); err != nil {
		t.Fatalf("unmarshal cmd/routes JSON: %v", err)
	}

	paramColonRe := regexp.MustCompile(`/:[A-Za-z0-9_]+`)
	paramStarRe := regexp.MustCompile(`/\*[A-Za-z0-9_]+`)
	paramBraceRe := regexp.MustCompile(`/\{[^}]+\}`)
	multiSlashRe := regexp.MustCompile(`/+`)

	normalize := func(p string) string {
		clean := strings.TrimSpace(strings.SplitN(p, "?", 2)[0])
		if !strings.HasPrefix(clean, "/") {
			clean = "/" + clean
		}
		clean = multiSlashRe.ReplaceAllString(clean, "/")
		if len(clean) > 1 && strings.HasSuffix(clean, "/") {
			clean = strings.TrimSuffix(clean, "/")
		}
		clean = paramColonRe.ReplaceAllString(clean, "/{*}")
		clean = paramStarRe.ReplaceAllString(clean, "/{**}")
		clean = paramBraceRe.ReplaceAllString(clean, "/{*}")
		return clean
	}

	backendMap := map[string]string{}
	for _, r := range backendRoutes {
		key := strings.ToUpper(r.Method) + " " + normalize(r.Path)
		backendMap[key] = r.Path
	}

	rawOpenAPI, err := os.ReadFile(openapiPath)
	if err != nil {
		t.Fatalf("read %s: %v", openapiPath, err)
	}
	openapiText := strings.ReplaceAll(string(rawOpenAPI), "\r\n", "\n")
	lines := strings.Split(openapiText, "\n")

	httpMethods := map[string]bool{
		"get": true, "post": true, "put": true, "delete": true, "patch": true, "options": true, "head": true,
	}
	pathLineRe := regexp.MustCompile(`^  (/[^:]+):\s*$`)
	methodLineRe := regexp.MustCompile(`^    ([a-z]+):\s*$`)
	topKeyRe := regexp.MustCompile(`^[A-Za-z0-9_-]+:`)

	type oaOp struct {
		method string
		path   string
		line   int
	}
	var oaOps []oaOp
	seenPathKeys := map[string]int{}
	inPaths := false
	curPath := ""

	for i, line := range lines {
		if strings.TrimSpace(line) == "paths:" && !strings.HasPrefix(line, " ") {
			inPaths = true
			continue
		}
		if inPaths && topKeyRe.MatchString(line) {
			inPaths = false
			curPath = ""
			continue
		}
		if !inPaths {
			continue
		}
		if m := pathLineRe.FindStringSubmatch(line); m != nil {
			curPath = strings.TrimSpace(m[1])
			if prevLine, dup := seenPathKeys[curPath]; dup {
				t.Errorf("duplicate OpenAPI path key %q at line %d (first at %d)", curPath, i+1, prevLine)
			}
			seenPathKeys[curPath] = i + 1
			continue
		}
		if curPath != "" {
			if m := methodLineRe.FindStringSubmatch(line); m != nil && httpMethods[m[1]] {
				oaOps = append(oaOps, oaOp{method: strings.ToUpper(m[1]), path: curPath, line: i + 1})
			}
		}
	}

	oaMap := map[string]string{}
	for _, op := range oaOps {
		full := "/api" + op.path
		if op.path == "/health" {
			full = "/health"
		}
		key := op.method + " " + normalize(full)
		oaMap[key] = fmt.Sprintf("%s %s (line %d)", op.method, op.path, op.line)
	}

	for key, raw := range backendMap {
		if _, ok := oaMap[key]; !ok {
			t.Errorf("backend route missing in api/openapi.yaml: %s (%s)", key, raw)
		}
	}
	for key, raw := range oaMap {
		if _, ok := backendMap[key]; !ok {
			t.Errorf("OpenAPI operation missing in backend router (ghost/misaligned route): %s (%s)", key, raw)
		}
	}

	// Verify key BUG-CONTRACT-03/04/05 schema alignments exist in openapi.yaml
	requiredSnippets := []string{
		"operationId: VerifyAuth",
		"valid:\n                    type: boolean",
		"retryInterval:\n                  type: integer",
		"bootId:\n                  type: integer",
		"hasPerformedBackup:\n                  type: boolean",
		"componentType:\n                  type: string",
		"serialNumber:\n                  type: string",
		"deprecated: true",
		"'410':",
	}
	for _, snip := range requiredSnippets {
		if !strings.Contains(openapiText, snip) {
			t.Errorf("expected aligned OpenAPI snippet %q in api/openapi.yaml", snip)
		}
	}
}
