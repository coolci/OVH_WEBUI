package db

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ovh-webui/server/internal/secret"
	"github.com/ovh-webui/server/internal/types"
)

// BUG-BE-02: Read-then-write transactions (e.g. DeleteAccount / SetDefaultAccount) running concurrently
// with pooled KV writers and concurrent readers must NEVER fail with <1ms SQLITE_BUSY (517),
// and Close() must release both writeDB and readDB handles cleanly on Windows.
func TestRegression_BE02_ImmediateTxZeroSQLiteBusy(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() {
		if cerr := database.Close(); cerr != nil {
			t.Errorf("Close() returned error: %v", cerr)
		}
	}()

	if err := database.UpsertAccount(types.OVHAccount{ID: "acc-1", Name: "A1", Endpoint: "ovh-eu", Zone: "IE", IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertAccount(types.OVHAccount{ID: "acc-2", Name: "A2", Endpoint: "ovh-us", Zone: "US"}); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 4 concurrent writer goroutines
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				if err := database.SetKV("probe_key", map[string]int{"w": id, "i": i}); err != nil {
					if strings.Contains(strings.ToLower(err.Error()), "locked") || strings.Contains(strings.ToLower(err.Error()), "busy") {
						t.Errorf("SetKV hit SQLITE_BUSY: %v", err)
						return
					}
				}
			}
		}(w)
	}

	// 4 concurrent reader goroutines exercising readDB pool
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := database.ListAccounts(); err != nil {
					t.Errorf("ListAccounts failed under concurrent writes: %v", err)
					return
				}
			}
		}()
	}

	// 80 read-then-write transactions (UpsertAccount + DeleteAccount)
	for i := 0; i < 80; i++ {
		id := "acc-del-loop"
		if err := database.UpsertAccount(types.OVHAccount{ID: id, Name: "Loop", Endpoint: "ovh-us", Zone: "US"}); err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("iteration %d UpsertAccount failed: %v", i, err)
		}
		if err := database.DeleteAccount(id); err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("iteration %d DeleteAccount failed (SQLITE_BUSY regression): %v", i, err)
		}
	}

	close(stop)
	wg.Wait()
}

// BUG-BE-07: UpsertVPSSubscription and UpsertMonitorSubscription must allow clearing LastStatus/History
// when explicit non-nil empty '{}'/'[]' is passed (e.g. subsidiary switch reset), while preserving existing state when nil is passed.
func TestRegression_BE07_UpsertSubscriptionClearsExplicitEmptyAndPreservesNil(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	// 1. VPS Subscription: insert with FR status, then reset with explicit empty map/slice
	vpsSub := types.VPSSubscription{
		ID:            "vps-sub-1",
		PlanCode:      "vps-2025-model1",
		OvhSubsidiary: "FR",
		LastStatus:    map[string]string{"eu-west-gra": "available"},
		History:       []map[string]interface{}{{"dc": "eu-west-gra", "status": "available"}},
		CreatedAt:     "2026-10-01T00:00:00Z",
	}
	if err := database.UpsertVPSSubscription(vpsSub); err != nil {
		t.Fatal(err)
	}

	// Nil LastStatus/History preserves state
	vpsNilEdit := vpsSub
	vpsNilEdit.NotifyUnavailable = true
	vpsNilEdit.LastStatus = nil
	vpsNilEdit.History = nil
	if err := database.UpsertVPSSubscription(vpsNilEdit); err != nil {
		t.Fatal(err)
	}
	gotVPSAfterNil, err := database.ListVPSSubscriptions()
	if err != nil || len(gotVPSAfterNil) != 1 || len(gotVPSAfterNil[0].LastStatus) != 1 {
		t.Fatalf("expected nil edit to preserve LastStatus, got %+v (err=%v)", gotVPSAfterNil, err)
	}

	// Explicit empty map/slice clears state
	vpsSub.OvhSubsidiary = "US"
	vpsSub.LastStatus = map[string]string{}
	vpsSub.History = []map[string]interface{}{}
	if err := database.UpsertVPSSubscription(vpsSub); err != nil {
		t.Fatal(err)
	}
	gotVPS, err := database.ListVPSSubscriptions()
	if err != nil || len(gotVPS) != 1 {
		t.Fatalf("list vps: %v len=%d", err, len(gotVPS))
	}
	if len(gotVPS[0].LastStatus) != 0 || len(gotVPS[0].History) != 0 {
		t.Fatalf("expected explicit empty LastStatus/History to clear stale state, got LastStatus=%v History=%v",
			gotVPS[0].LastStatus, gotVPS[0].History)
	}

	// 2. Monitor Subscription: insert with LastStatus, then reset with explicit empty map
	monSub := types.Subscription{
		PlanCode:   "24ska01",
		LastStatus: map[string]string{"gra": "1H-low"},
		History: []types.SubscriptionHistoryEntry{
			{Datacenter: "gra", Status: "1H-low", Timestamp: "2026-10-01T00:00:00Z"},
		},
		CreatedAt: "2026-10-01T00:00:00Z",
	}
	if err := database.UpsertMonitorSubscription(monSub); err != nil {
		t.Fatal(err)
	}
	monSub.LastStatus = map[string]string{}
	monSub.History = []types.SubscriptionHistoryEntry{}
	if err := database.UpsertMonitorSubscription(monSub); err != nil {
		t.Fatal(err)
	}
	gotMon, err := database.ListMonitorSubscriptions()
	if err != nil || len(gotMon) != 1 {
		t.Fatalf("list monitor: %v len=%d", err, len(gotMon))
	}
	if len(gotMon[0].LastStatus) != 0 || len(gotMon[0].History) != 0 {
		t.Fatalf("expected explicit empty monitor LastStatus/History to be cleared, got LastStatus=%v History=%v",
			gotMon[0].LastStatus, gotMon[0].History)
	}
}

// BUG-BE-08: DeleteAccount must reset auto_order=0 and auto_pay=0 alongside auto_order_account_id=” on vps_subscriptions.
func TestRegression_BE08_DeleteAccountClearsVPSAutoOrderAndAutoPay(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := database.UpsertAccount(types.OVHAccount{ID: "acc-del", Name: "Del", Endpoint: "ovh-eu", Zone: "IE", IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertVPSSubscription(types.VPSSubscription{
		ID:                 "vps-zombie",
		PlanCode:           "vps-2025-model1",
		OvhSubsidiary:      "IE",
		AutoOrder:          true,
		AutoPay:            true,
		Quantity:           1,
		AutoOrderAccountID: "acc-del",
		CreatedAt:          "2026-10-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	if err := database.DeleteAccount("acc-del"); err != nil {
		t.Fatal(err)
	}
	subs, err := database.ListVPSSubscriptions()
	if err != nil || len(subs) != 1 {
		t.Fatalf("list vps: %v len=%d", err, len(subs))
	}
	got := subs[0]
	if got.AutoOrderAccountID != "" || got.AutoOrder || got.AutoPay {
		t.Fatalf("expected AutoOrderAccountID='', AutoOrder=false, AutoPay=false after DeleteAccount, got AutoOrderAccountID=%q AutoOrder=%v AutoPay=%v",
			got.AutoOrderAccountID, got.AutoOrder, got.AutoPay)
	}
}

// BUG-SEC-06 & BUG-SEC-07 (DB layer): EncryptExistingSecrets migrates plaintext proxy_url & notifyWebhookUrl,
// and HasEncryptedSecrets + RawAccount.HasProxyCiphertext detect proxy_url ciphertext.
func TestRegression_SEC06_ProxyURLAndWebhookMigrationAndEncryptedCheck(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(secret.KeyEnv, "regression-secret-key-2026-0123456789abcdef")
	if err := secret.Init(dir, filepath.Join(dir, ".env")); err != nil {
		t.Fatal(err)
	}

	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	plainProxy := "http://proxyuser:SuperSecretPass@10.0.0.1:8080"
	_, err = database.Exec(`INSERT INTO ovh_accounts (id, name, endpoint, zone, app_key, app_secret, consumer_key, iam, is_default, created_at, proxy_url) VALUES (?, ?, ?, ?, ?, ?, ?, 'go-ovh-ie', 1, '2026-01-01T00:00:00Z', ?)`,
		"legacy-acc", "Legacy", "ovh-eu", "IE", secret.Encrypt("ak"), secret.Encrypt("as"), secret.Encrypt("ck"), plainProxy)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO kv (key, value) VALUES ('config', ?)`,
		`{"notifyWebhookUrl":"https://hooks.slack.com/services/T00/B00/SECRET123"}`)
	if err != nil {
		t.Fatal(err)
	}

	migrated, err := database.EncryptExistingSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if migrated != 2 {
		t.Fatalf("expected migrated=2 (1 account proxy_url + 1 kv[config] notifyWebhookUrl), got %d", migrated)
	}

	// Idempotency check
	migrated2, err := database.EncryptExistingSecrets()
	if err != nil || migrated2 != 0 {
		t.Fatalf("expected second EncryptExistingSecrets to migrate 0, got %d (%v)", migrated2, err)
	}

	var storedProxy string
	if err := database.Get(&storedProxy, `SELECT proxy_url FROM ovh_accounts WHERE id = 'legacy-acc'`); err != nil {
		t.Fatal(err)
	}
	if !secret.IsEncrypted(storedProxy) {
		t.Fatalf("expected proxy_url to be encrypted, got %q", storedProxy)
	}
	rawAcc, ok, err := database.GetAccountRaw("legacy-acc")
	if err != nil || !ok || !rawAcc.HasProxyCiphertext() {
		t.Fatalf("expected RawAccount.HasProxyCiphertext()==true, got ok=%v hasProxy=%v err=%v", ok, rawAcc.HasProxyCiphertext(), err)
	}

	var storedCfg string
	if err := database.Get(&storedCfg, `SELECT value FROM kv WHERE key = 'config'`); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(storedCfg, "SECRET123") || !strings.Contains(storedCfg, secret.EncPrefix) {
		t.Fatalf("expected notifyWebhookUrl in kv[config] to be encrypted, got %s", storedCfg)
	}

	// Verify HasEncryptedSecrets when ONLY proxy_url is encrypted
	if _, err := database.Exec(`DELETE FROM ovh_accounts`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`DELETE FROM kv WHERE key = 'config'`); err != nil {
		t.Fatal(err)
	}
	if err := database.UpsertAccount(types.OVHAccount{
		ID:       "proxy-only",
		Name:     "ProxyOnly",
		Endpoint: "ovh-eu",
		Zone:     "IE",
		ProxyURL: plainProxy,
	}); err != nil {
		t.Fatal(err)
	}
	hasEnc, err := database.HasEncryptedSecrets()
	if err != nil || !hasEnc {
		t.Fatalf("expected HasEncryptedSecrets()==true when only proxy_url is encrypted, got %v (%v)", hasEnc, err)
	}
}
