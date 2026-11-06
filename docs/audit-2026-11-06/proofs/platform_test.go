// HISTORICAL defect reproducer for the audit baseline only; current run-* scripts use formal regression tests.
// Isolated audit evidence. Overlay this file into internal/server; it intentionally
// confirms the observed defects and is not part of the default regression suite.
package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/handlers"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func auditState(t *testing.T) *app.State {
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

func TestAuditFailedConfigSavePublishesMemory(t *testing.T) {
	s := auditState(t)
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
	if active != next.TgChatID || persisted != before.TgChatID {
		t.Fatalf("unexpected state active=%q persisted=%q", active, persisted)
	}
	t.Logf("CONFIRMED: save error=%v; active=%s; persisted=%s", err, active, persisted)
}

func TestAuditPairingDatabaseFailureReturnsSuccess(t *testing.T) {
	s := auditState(t)
	if _, err := s.DB.Exec(`DROP TABLE app_pairing_codes`); err != nil {
		t.Fatal(err)
	}
	code, expiry, err := s.DB.CreatePairingCode()
	if err != nil || code != "" || !expiry.IsZero() {
		t.Fatalf("unexpected code=%q expiry=%v error=%v", code, expiry, err)
	}
	t.Logf("CONFIRMED: missing table gives code=%q, zeroExpiry=%t, error=%v", code, expiry.IsZero(), err)
}

func TestAuditFailedVPSSubscriptionSavePublishesAutoOrder(t *testing.T) {
	s := auditState(t)
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
	if readErr != nil || len(active) != 1 || len(persisted) != 1 || !active[0].AutoOrder || !active[0].AutoPay || persisted[0].AutoOrder || persisted[0].AutoPay {
		t.Fatalf("unexpected state readErr=%v", readErr)
	}
	t.Logf("CONFIRMED: save error=%v; active autoOrder/autoPay=true/true; persisted=false/false", err)
}

func TestAuditAccountUpdatePersistsInvalidProxy(t *testing.T) {
	s := auditState(t)
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
	if create.Code != 400 || update.Code != 422 || err != nil || !ok || stored.ProxyURL != "invalid://proxy:1234" || stored.Fingerprint != "unsupported-audit-profile" {
		t.Fatalf("create=%d update=%d storedProxy=%q err=%v", create.Code, update.Code, stored.ProxyURL, err)
	}
	t.Logf("CONFIRMED: create rejects invalid proxy with %d; update returns %d after persisting invalid proxy and fingerprint", create.Code, update.Code)
}

func TestAuditDeletedLastAccountResurrectsAtStartup(t *testing.T) {
	s := auditState(t)
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
	if err != nil || len(accounts) != 1 || accounts[0].ConsumerKey != a.ConsumerKey || accounts[0].ID == a.ID {
		t.Fatalf("unexpected restored accounts: count=%d err=%v", len(accounts), err)
	}
	t.Log("CONFIRMED: deleted final account count 0 -> startup count 1; original credentials restored under new ID")
}

func TestAuditRunReturnsBeforeHTTPShutdownDrains(t *testing.T) {
	s := auditState(t)
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := reservation.Addr().String()
	_ = reservation.Close()
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("finished"))
	})
	srv := New(Config{Addr: addr, Handler: handler, State: s, EnableAuth: true, Console: slog.New(slog.NewTextHandler(io.Discard, nil))})
	runDone := make(chan error, 1)
	go func() { runDone <- srv.Run() }()
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		client := &http.Client{Timeout: 3 * time.Second}
		for i := 0; i < 100; i++ {
			resp, e := client.Get("http://" + addr + "/audit")
			if e == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("local handler did not start")
	}
	shutdownDone := make(chan struct{})
	srv.shutdownPending.Store(true)
	go func() {
		defer close(shutdownDone)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.httpServer.Shutdown(ctx)
	}()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("Run waited; defect no longer reproduced")
	}
	select {
	case <-shutdownDone:
		close(release)
		t.Fatal("shutdown finished unexpectedly")
	default:
	}
	t.Log("CONFIRMED: Run returned while active request and Shutdown were still waiting; main would exit here")
	close(release)
	<-shutdownDone
	<-requestDone
}
