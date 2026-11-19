package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/handlers"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func persistenceState(t *testing.T) *app.State {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	lg := logger.New(filepath.Join(dir, "logs.json"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return app.NewState(storage.Paths{}, config.New(database), lg, database)
}

func TestFailedConfigSaveKeepsCommittedMemory(t *testing.T) {
	s := persistenceState(t)
	before := s.Config.Get()
	before.TgChatID = "old-audit-chat"
	if err := s.Config.Set(before); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`CREATE TRIGGER audit_reject_config BEFORE UPDATE ON kv BEGIN SELECT RAISE(FAIL, 'audit write rejected'); END`); err != nil {
		t.Fatal(err)
	}
	next := before
	next.TgChatID = "new-audit-chat"
	err := s.Config.Set(next)
	if err == nil {
		t.Fatal("expected injected database write error")
	}
	active := s.Config.Get().TgChatID
	persisted := config.New(s.DB).Get().TgChatID
	if active != before.TgChatID || persisted != before.TgChatID {
		t.Fatalf("unexpected state active=%q persisted=%q", active, persisted)
	}
}

func TestPairingDatabaseFailureReturnsError(t *testing.T) {
	s := persistenceState(t)
	if _, err := s.DB.Exec(`DROP TABLE app_pairing_codes`); err != nil {
		t.Fatal(err)
	}
	code, expiry, err := s.DB.CreatePairingCode()
	if err == nil || code != "" || !expiry.IsZero() {
		t.Fatalf("unexpected code=%q expiry=%v error=%v", code, expiry, err)
	}
}

func TestFailedVPSSubscriptionSaveKeepsAutoOrderDisabled(t *testing.T) {
	s := persistenceState(t)
	sub := types.VPSSubscription{ID: "audit-vps", PlanCode: "audit-plan", OvhSubsidiary: "IE", CreatedAt: types.NowISO()}
	if err := s.AddVPSSubscription(sub); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`CREATE TRIGGER audit_reject_vps BEFORE DELETE ON vps_subscriptions BEGIN SELECT RAISE(FAIL, 'audit write rejected'); END`); err != nil {
		t.Fatal(err)
	}
	_, found, err := s.MutateVPSSubscription(sub.ID, func(v *types.VPSSubscription) bool { v.AutoOrder = true; v.AutoPay = true; return true })
	if err == nil || !found {
		t.Fatal("expected injected database write failure")
	}
	active := s.VPSSubscriptionSnapshot()
	persisted, readErr := s.DB.ListVPSSubscriptions()
	if readErr != nil || len(active) != 1 || len(persisted) != 1 || active[0].AutoOrder || active[0].AutoPay || persisted[0].AutoOrder || persisted[0].AutoPay {
		t.Fatalf("unexpected state readErr=%v", readErr)
	}
}

func TestAccountUpdateRejectsInvalidProxy(t *testing.T) {
	s := persistenceState(t)
	a := types.OVHAccount{ID: "audit-account", Name: "audit", Endpoint: "ovh-eu", Zone: "IE", AppKey: "audit-key", AppSecret: "audit-secret", ConsumerKey: "audit-consumer", IsDefault: true, CreatedAt: types.NowISO()}
	if err := s.DB.UpsertAccount(a); err != nil {
		t.Fatal(err)
	}
	if err := s.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.POST("/accounts", handlers.CreateAccount(s))
	r.PUT("/accounts/:id", handlers.UpdateAccount(s))
	create := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(`{"name":"audit","endpoint":"ovh-eu","zone":"IE","appKey":"a","appSecret":"b","consumerKey":"c","proxyUrl":"invalid://proxy:1234"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(create, req)
	update := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/accounts/audit-account", strings.NewReader(`{"proxyUrl":"invalid://proxy:1234","fingerprint":"unsupported-audit-profile"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(update, req)
	stored, ok, err := s.DB.GetAccount(a.ID)
	if create.Code != 400 || update.Code != 400 || err != nil || !ok || stored.ProxyURL != "" || stored.Fingerprint != "" {
		t.Fatalf("create=%d update=%d storedProxy=%q err=%v", create.Code, update.Code, stored.ProxyURL, err)
	}
}

func TestDeletedLastAccountStaysDeletedAtStartup(t *testing.T) {
	s := persistenceState(t)
	a := types.OVHAccount{ID: "audit-account", Name: "audit", Endpoint: "ovh-eu", Zone: "IE", AppKey: "audit-key", AppSecret: "audit-secret", ConsumerKey: "audit-consumer", IsDefault: true, CreatedAt: types.NowISO()}
	if err := s.DB.UpsertAccount(a); err != nil {
		t.Fatal(err)
	}
	s.LoadAll()
	if err := s.DB.DeleteAccount(a.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}
	count, err := s.DB.CountAccounts()
	if err != nil || count != 0 {
		t.Fatalf("deletion failed: count=%d err=%v", count, err)
	}
	s.LoadAll() // identical legacy migration path to a subsequent process startup
	accounts, err := s.DB.ListAccounts()
	if err != nil || len(accounts) != 0 || s.Config.Get().ConsumerKey != "" || config.New(s.DB).Get().ConsumerKey != "" {
		t.Fatalf("unexpected restored accounts: count=%d err=%v", len(accounts), err)
	}
}

func TestVPSMutatorsKeepCommittedStateOnFailure(t *testing.T) {
	for _, operation := range []string{"add", "remove", "clear", "batch", "interval"} {
		t.Run(operation, func(t *testing.T) {
			s := persistenceState(t)
			if err := s.AddVPSSubscription(types.VPSSubscription{ID: "original", PlanCode: "original", OvhSubsidiary: "IE"}); err != nil {
				t.Fatal(err)
			}
			if err := s.SetVPSCheckInterval(5); err != nil {
				t.Fatal(err)
			}
			before := s.VPSSubscriptionSnapshot()
			if _, err := s.DB.Exec(`CREATE TRIGGER reject_vps BEFORE DELETE ON vps_subscriptions BEGIN SELECT RAISE(FAIL,'fixture'); END`); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB.Exec(`CREATE TRIGGER reject_interval BEFORE UPDATE ON kv BEGIN SELECT RAISE(FAIL,'fixture'); END`); err != nil {
				t.Fatal(err)
			}
			var err error
			switch operation {
			case "add":
				err = s.AddVPSSubscription(types.VPSSubscription{ID: "new", PlanCode: "new"})
			case "remove":
				_, _, err = s.RemoveVPSSubscription("original")
			case "clear":
				_, err = s.ClearVPSSubscriptions()
			case "batch":
				_, err = s.BatchAddVPSSubscriptions([]types.VPSSubscription{{ID: "new", PlanCode: "new"}})
			case "interval":
				err = s.SetVPSCheckInterval(99)
			}
			if err == nil {
				t.Fatal("expected database rejection")
			}
			if !reflect.DeepEqual(before, s.VPSSubscriptionSnapshot()) {
				t.Fatal("failed mutation published")
			}
			_, interval := s.VPSMonitorStatus()
			if interval != 5 {
				t.Fatal("failed interval published")
			}
		})
	}
}
func TestConcurrentVPSAddsKeepAllCommittedSubscriptions(t *testing.T) {
	s := persistenceState(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := s.AddVPSSubscription(types.VPSSubscription{ID: fmt.Sprint(i), PlanCode: fmt.Sprint(i), OvhSubsidiary: "IE"}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	active := s.VPSSubscriptionSnapshot()
	stored, err := s.DB.ListVPSSubscriptions()
	if err != nil || len(active) != 20 || len(stored) != 20 {
		t.Fatalf("active=%d stored=%d err=%v", len(active), len(stored), err)
	}
	ids := map[string]bool{}
	for _, item := range stored {
		ids[item.ID] = true
	}
	for _, item := range active {
		if !ids[item.ID] {
			t.Fatalf("missing persisted id %s", item.ID)
		}
	}
}
func TestLegacyCredentialsMigrateExactlyOnce(t *testing.T) {
	s := persistenceState(t)
	cfg := s.Config.Get()
	cfg.AppKey = "fixture"
	cfg.AppSecret = "fixture"
	cfg.ConsumerKey = "fixture"
	if err := s.Config.Set(cfg); err != nil {
		t.Fatal(err)
	}
	s.LoadAll()
	accounts, err := s.DB.ListAccounts()
	if err != nil || len(accounts) != 1 {
		t.Fatalf("migration count=%d err=%v", len(accounts), err)
	}
	s.LoadAll()
	again, _ := s.DB.ListAccounts()
	if len(again) != 1 || again[0].ID != accounts[0].ID {
		t.Fatal("migration repeated")
	}
}

func TestFailedLegacyMigrationPreservesCredentialSource(t *testing.T) {
	s := persistenceState(t)
	cfg := s.Config.Get()
	cfg.AppKey = "fixture"
	cfg.AppSecret = "fixture"
	cfg.ConsumerKey = "fixture"
	if err := s.Config.Set(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`CREATE TRIGGER reject_account BEFORE INSERT ON ovh_accounts BEGIN SELECT RAISE(FAIL,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	s.LoadAll()
	if s.Config.Get().ConsumerKey != "fixture" || config.New(s.DB).Get().ConsumerKey != "fixture" {
		t.Fatal("failed migration erased the only credential source")
	}
}
