package secverify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/auth"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/handlers"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/router"
	"github.com/ovh-webui/server/internal/secret"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
	"github.com/ovh-webui/server/internal/updater"
)

func newTestState(t *testing.T) (*app.State, *db.DB) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	lg := logger.New(filepath.Join(dir, "t.log"),
		slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))
	st := app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
	t.Cleanup(func() {
		lg.Flush()
		_ = database.Close()
	})
	return st, database
}

// BUG-SEC-01: Verify server/.env.example removes weak API_SECRET_KEY=123456 & binds 127.0.0.1,
// init-first-run scripts anchor ^API_SECRET_KEY=.* with post-write verification, and ValidateStartupAPIKey rejects weak keys.
func TestSEC01_EnvExampleClearedInitScriptsAnchoredAndWeakKeyBlocked(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	envExampleBytes, err := os.ReadFile(filepath.Join(repoRoot, "server", ".env.example"))
	if err != nil {
		t.Fatalf("read server/.env.example: %v", err)
	}
	envExample := string(envExampleBytes)
	if strings.Contains(envExample, "API_SECRET_KEY=123456") {
		t.Fatalf("server/.env.example still contains weak default API_SECRET_KEY=123456")
	}
	if !regexp.MustCompile(`(?m)^API_SECRET_KEY=\s*$`).MatchString(envExample) {
		t.Fatalf("expected server/.env.example to declare empty API_SECRET_KEY=, got:\n%s", envExample)
	}
	if !regexp.MustCompile(`(?m)^LISTEN_HOST=127\.0\.0\.1\s*$`).MatchString(envExample) {
		t.Fatalf("expected server/.env.example to set LISTEN_HOST=127.0.0.1, got:\n%s", envExample)
	}

	initShBytes, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "init-first-run.sh"))
	if err != nil {
		t.Fatalf("read scripts/init-first-run.sh: %v", err)
	}
	initSh := string(initShBytes)
	if strings.Contains(initSh, "s/API_SECRET_KEY=change-me-to-a-long-random-string/") {
		t.Fatalf("scripts/init-first-run.sh still relies on mismatched hardcoded placeholder")
	}
	if !strings.Contains(initSh, "^API_SECRET_KEY=.*") || !strings.Contains(initSh, `grep -Fxq "API_SECRET_KEY=$key"`) {
		t.Fatalf("scripts/init-first-run.sh missing anchored ^API_SECRET_KEY=.* replacement or post-write verification")
	}

	initPs1Bytes, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "init-first-run.ps1"))
	if err != nil {
		t.Fatalf("read scripts/init-first-run.ps1: %v", err)
	}
	initPs1 := string(initPs1Bytes)
	if !strings.Contains(initPs1, "Assert-EnvApiKeyWritten") || !strings.Contains(initPs1, "Test-WeakApiSecretKey") {
		t.Fatalf("scripts/init-first-run.ps1 missing post-write strength verification")
	}

	// Startup gate must block weak/short keys and allow strong 64-hex generated keys
	for _, weak := range []string{"", "   ", "123456", "change-me-to-a-long-random-string", "ovh-webui-dev-key", "password", "too-short-123"} {
		if err := auth.ValidateStartupAPIKey(weak); err == nil {
			t.Fatalf("expected ValidateStartupAPIKey(%q) to fail, but it succeeded", weak)
		}
	}
	genKey, generated, err := auth.ResolveAPIKey(t.TempDir(), "")
	if err != nil || !generated {
		t.Fatalf("ResolveAPIKey failed: %v (generated=%v)", err, generated)
	}
	if err := auth.ValidateStartupAPIKey(genKey); err != nil {
		t.Fatalf("expected generated 64-hex key to pass ValidateStartupAPIKey, got: %v", err)
	}
}

// BUG-SEC-02 & REJECTED-02:
//  1. Keep Linux stdlib executable_procfs.go TrimSuffix(" (deleted)") proof.
//  2. On Windows, compile a real running binary that invokes updater.RollbackIfStale on ITSELF while mapped & running,
//     proving the two-step rename (exe -> exe.failed, backup -> exe) succeeds without Access is denied!
func TestSEC02_WindowsRollbackRunningExeSucceedsAndLinuxStdlibCheck(t *testing.T) {
	procfsPath := filepath.Join(runtime.GOROOT(), "src", "os", "executable_procfs.go")
	procfsBytes, err := os.ReadFile(procfsPath)
	if err != nil {
		t.Fatalf("read %s: %v", procfsPath, err)
	}
	if !strings.Contains(string(procfsBytes), `TrimSuffix(path, " (deleted)")`) {
		t.Fatalf("expected Go stdlib executable_procfs.go to strip ' (deleted)'")
	}

	if runtime.GOOS == "windows" {
		tmp := t.TempDir()
		helperSrc := filepath.Join(tmp, "sleeper.go")
		helperExe := filepath.Join(tmp, "ovh-server.exe")
		backupExe := helperExe + ".old"
		failedExe := helperExe + ".failed"
		pendingFile := helperExe + ".pending"

		src := "package main\nimport (\"fmt\"; \"time\")\nfunc main() { fmt.Println(\"READY\"); time.Sleep(10 * time.Second) }\n"
		if err := os.WriteFile(helperSrc, []byte(src), 0o600); err != nil {
			t.Fatalf("write sleeper.go: %v", err)
		}
		buildCmd := exec.Command("go", "build", "-o", helperExe, helperSrc)
		if out, err := buildCmd.CombinedOutput(); err != nil {
			t.Fatalf("build sleeper: %v (%s)", err, string(out))
		}
		wantBackupBytes := []byte("HEALTHY-PREVIOUS-VERSION-BINARY")
		if err := os.WriteFile(backupExe, wantBackupBytes, 0o755); err != nil {
			t.Fatalf("write .old backup: %v", err)
		}
		if err := os.WriteFile(pendingFile, []byte("1"), 0o644); err != nil {
			t.Fatalf("write .pending marker: %v", err)
		}

		cmd := exec.Command(helperExe)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("stdout pipe: %v", err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatalf("start helperExe: %v", err)
		}
		killed := false
		defer func() {
			if !killed {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		}()

		buf := make([]byte, 16)
		_, _ = stdout.Read(buf)

		restoreSelf := updater.SetSelfPathForTesting(func() (string, error) {
			return helperExe, nil
		})
		defer restoreSelf()

		st, _ := newTestState(t)
		// Execute RollbackIfStale while helperExe is actively running & mapped by Windows kernel!
		if !updater.RollbackIfStale(st) {
			t.Fatalf("RollbackIfStale failed while target .exe was actively running on Windows")
		}

		gotExeBytes, err := os.ReadFile(helperExe)
		if err != nil {
			t.Fatalf("read rolled-back exe: %v", err)
		}
		if !bytes.Equal(gotExeBytes, wantBackupBytes) {
			t.Fatalf("expected running exe to be replaced by .old backup (%d bytes), got %d bytes", len(wantBackupBytes), len(gotExeBytes))
		}
		if _, err := os.Stat(pendingFile); !os.IsNotExist(err) {
			t.Fatalf("expected .pending marker to be removed after successful rollback")
		}

		// Now terminate the old mapped process (simulating restart exit) and verify CleanupStale removes .failed!
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		killed = true
		updater.CleanupStale()
		if _, err := os.Stat(failedExe); !os.IsNotExist(err) {
			t.Fatalf("expected CleanupStale to remove %s after old process exited", failedExe)
		}
	}
}

// BUG-SEC-03: Verify TRUSTED_PROXIES in docker-compose*.yml AND valid X-API-Key / Bearer device token
// immediately pass even when shared source IP is locked out by 10 unauthenticated brute-force failures.
func TestSEC03_ValidCredentialsBypassSharedIPLockoutAndComposeTrustedProxies(t *testing.T) {
	auth.ResetAuthFailures()
	t.Cleanup(auth.ResetAuthFailures)

	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	for _, composeName := range []string{"docker-compose.yml", "deploy/docker-compose.https.yml"} {
		b, err := os.ReadFile(filepath.Join(repoRoot, composeName))
		if err != nil {
			t.Fatalf("read %s: %v", composeName, err)
		}
		if !strings.Contains(string(b), "TRUSTED_PROXIES:") {
			t.Fatalf("%s missing TRUSTED_PROXIES configuration", composeName)
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(auth.Middleware(auth.Config{
		APIKey:         "correct-admin-secret-key",
		Enabled:        true,
		WhitelistPaths: auth.DefaultWhitelist(),
		DeviceTokenValid: func(token string) (int64, bool) {
			if token == "app_valid_device_token" {
				return 42, true
			}
			return 0, false
		},
	}))
	r.GET("/api/settings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	proxyPeer := "172.18.0.5:54321"
	for i := 0; i < auth.MaxAuthFailures; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		req.RemoteAddr = proxyPeer
		req.Header.Set("X-API-Key", "wrong-guess")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, w.Code)
		}
	}

	// Attacker's 11th wrong guess is rate-limited with 429
	badReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	badReq.RemoteAddr = proxyPeer
	badReq.Header.Set("X-API-Key", "wrong-guess")
	badW := httptest.NewRecorder()
	r.ServeHTTP(badW, badReq)
	if badW.Code != http.StatusTooManyRequests {
		t.Fatalf("expected attacker wrong guess to get 429, got %d", badW.Code)
	}

	// Legitimate admin with valid X-API-Key behind the SAME proxyPeer MUST succeed (200 OK)!
	adminReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	adminReq.RemoteAddr = proxyPeer
	adminReq.Header.Set("X-API-Key", "correct-admin-secret-key")
	adminW := httptest.NewRecorder()
	r.ServeHTTP(adminW, adminReq)
	if adminW.Code != http.StatusOK {
		t.Fatalf("valid admin X-API-Key must NOT be locked out by shared IP failures: got %d (%s)", adminW.Code, adminW.Body.String())
	}

	// Legitimate mobile app with valid Bearer token behind the SAME proxyPeer MUST also succeed (200 OK)!
	appReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	appReq.RemoteAddr = proxyPeer
	appReq.Header.Set("Authorization", "Bearer app_valid_device_token")
	appW := httptest.NewRecorder()
	r.ServeHTTP(appW, appReq)
	if appW.Code != http.StatusOK {
		t.Fatalf("valid Bearer device token must NOT be locked out by shared IP failures: got %d (%s)", appW.Code, appW.Body.String())
	}

	// Attacker sending wrong guess right after admin succeeded STILL gets 429!
	badReq2 := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	badReq2.RemoteAddr = proxyPeer
	badReq2.Header.Set("X-API-Key", "wrong-guess-again")
	badW2 := httptest.NewRecorder()
	r.ServeHTTP(badW2, badReq2)
	if badW2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected attacker wrong guess to remain locked out with 429, got %d", badW2.Code)
	}
}

// BUG-SEC-05: Verify atomic per-IP limiter stops concurrent TOCTOU at max=5,
// global DoS is prevented so legitimate pairing succeeds, and deviceName/logs strip CRLF injection.
func TestSEC05_PairEndpointAtomicLimiterNoGlobalDoSAndLogSanitization(t *testing.T) {
	handlers.ResetPairLimiters()
	t.Cleanup(handlers.ResetPairLimiters)

	gin.SetMode(gin.TestMode)
	st, _ := newTestState(t)

	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.POST("/api/app/pair", handlers.RedeemPairingCode(st))

	const concurrency = 20
	var passedLimiter atomic.Int32
	var rateLimited atomic.Int32
	var startGate sync.WaitGroup
	startGate.Add(1)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			startGate.Wait()
			body := fmt.Sprintf(`{"code":"234567%02d","deviceName":"test"}`, (idx%8)+22)
			req := httptest.NewRequest(http.MethodPost, "/api/app/pair", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "192.0.2.77:40000"
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			switch w.Code {
			case http.StatusBadRequest:
				passedLimiter.Add(1)
			case http.StatusTooManyRequests:
				rateLimited.Add(1)
			}
		}(i)
	}
	startGate.Done()
	wg.Wait()

	if got := passedLimiter.Load(); got > 5 {
		t.Fatalf("TOCTOU regression: %d/20 concurrent requests from single IP bypassed max=5 limiter", got)
	}
	if rateLimited.Load() < 15 {
		t.Fatalf("expected at least 15/20 concurrent requests to receive 429, got %d", rateLimited.Load())
	}

	// Spray 35 invalid requests from attacker IPs; legitimate admin from another subnet must still redeem validCode!
	validCode, _, err := st.DeviceRepo.CreatePairingCode()
	if err != nil {
		t.Fatalf("CreatePairingCode: %v", err)
	}
	for i := 0; i < 35; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/app/pair", strings.NewReader(`{"code":"ABCDEFGH","deviceName":"bot"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:50000", (i%200)+10)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}

	legitBody := fmt.Sprintf(`{"code":%q,"deviceName":"Admin\r\n[INFO] ForgedLine\u0000Phone"}`, validCode)
	legitReq := httptest.NewRequest(http.MethodPost, "/api/app/pair", strings.NewReader(legitBody))
	legitReq.Header.Set("Content-Type", "application/json")
	legitReq.RemoteAddr = "203.0.113.250:51234"
	legitW := httptest.NewRecorder()
	r.ServeHTTP(legitW, legitReq)
	if legitW.Code != http.StatusOK {
		t.Fatalf("expected valid pairing code to succeed with 200 OK, got %d (%s)", legitW.Code, legitW.Body.String())
	}

	devs, err := st.DeviceRepo.ListDevices()
	if err != nil || len(devs) != 1 {
		t.Fatalf("ListDevices: %v len=%d", err, len(devs))
	}
	if strings.ContainsAny(devs[0].Name, "\r\n\x00") {
		t.Fatalf("device name retained control characters (log injection risk): %q", devs[0].Name)
	}
}

// BUG-SEC-06 & BUG-SEC-07:
// 1. EncryptExistingSecrets encrypts ovh_accounts.proxy_url and UpdateAccount returns 409 on OVH_DB_KEY mismatch instead of wiping proxy_url!
// 2. NotifyWebhookURL is encrypted at rest in kv['config'], masked in GET /api/settings, and preserved when saving masked URL in POST /api/settings!
func TestSEC06_SEC07_ProxyURLMigrationMismatchGuardAndEncryptedMaskedWebhook(t *testing.T) {
	gin.SetMode(gin.TestMode)
	keyDirA := t.TempDir()
	envA := filepath.Join(keyDirA, ".env")
	t.Setenv("OVH_DB_KEY", "1111111111111111111111111111111111111111111111111111111111111111")
	if err := secret.Init(keyDirA, envA); err != nil {
		t.Fatalf("secret.Init A: %v", err)
	}

	st, database := newTestState(t)

	rawProxy := "socks5://proxyuser:proxypass@10.20.30.40:1080"
	_, err := database.Exec(`INSERT INTO ovh_accounts (id, name, zone, iam, endpoint, app_key, app_secret, consumer_key, proxy_url, is_default, created_at)
		VALUES ('acc1', 'LegacyAcc', 'IE', 'go-ovh-ie', 'ovh-eu', 'plain_ak', 'plain_as', 'plain_ck', ?, 1, ?)`, rawProxy, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("insert legacy account: %v", err)
	}
	if migrated, err := database.EncryptExistingSecrets(); err != nil || migrated != 1 {
		t.Fatalf("EncryptExistingSecrets: migrated=%d err=%v", migrated, err)
	}
	var dbAppKey, dbProxyAfterMigrate string
	if err := database.QueryRow(`SELECT app_key, proxy_url FROM ovh_accounts WHERE id = 'acc1'`).Scan(&dbAppKey, &dbProxyAfterMigrate); err != nil {
		t.Fatalf("query after migrate: %v", err)
	}
	if !secret.IsEncrypted(dbAppKey) || !secret.IsEncrypted(dbProxyAfterMigrate) {
		t.Fatalf("expected both app_key and proxy_url to be encrypted, got app_key=%q proxy_url=%q", dbAppKey, dbProxyAfterMigrate)
	}
	if has, err := database.HasEncryptedSecrets(); err != nil || !has {
		t.Fatalf("expected HasEncryptedSecrets=true, got %v (%v)", has, err)
	}

	// Switch to mismatched Key B: UpdateAccount without explicit proxyUrl MUST return 409 Conflict and NOT wipe proxy_url!
	t.Setenv("OVH_DB_KEY", "2222222222222222222222222222222222222222222222222222222222222222")
	if err := secret.Init(t.TempDir(), filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Fatalf("secret.Init B: %v", err)
	}

	r := gin.New()
	r.PUT("/api/accounts/:id", handlers.UpdateAccount(st))
	r.GET("/api/settings", handlers.GetSettings(st))
	r.POST("/api/settings", handlers.SaveSettings(st))

	putBody := `{"name":"EncryptedAcc","zone":"IE","endpoint":"ovh-eu","appKey":"new_ak","appSecret":"new_as","consumerKey":"new_ck"}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/accounts/acc1", bytes.NewBufferString(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	r.ServeHTTP(putW, putReq)
	if putW.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when proxy_url ciphertext cannot be decrypted and proxyUrl is omitted, got %d (%s)", putW.Code, putW.Body.String())
	}

	var dbProxyAfterConflict string
	if err := database.QueryRow(`SELECT proxy_url FROM ovh_accounts WHERE id = 'acc1'`).Scan(&dbProxyAfterConflict); err != nil {
		t.Fatalf("query after conflict: %v", err)
	}
	if dbProxyAfterConflict != dbProxyAfterMigrate {
		t.Fatalf("proxy_url ciphertext was altered despite 409 rejection: got %q, want %q", dbProxyAfterConflict, dbProxyAfterMigrate)
	}

	// SEC-07: NotifyWebhookURL encrypted at rest + masked on GET + preserved on POST
	secretWebhook := "https://oapi.dingtalk.com/robot/send?access_token=SUPER_SECRET_WEBHOOK_TOKEN"
	if err := st.Config.Set(types.Config{
		TgToken:          "123456:ABCDEF_SECRET_TG",
		NotifyWebhookURL: secretWebhook,
	}); err != nil {
		t.Fatalf("Config.Set: %v", err)
	}
	var rawConfigJSON string
	if err := database.Get(&rawConfigJSON, `SELECT value FROM kv WHERE key = 'config'`); err != nil {
		t.Fatalf("select kv[config]: %v", err)
	}
	if strings.Contains(rawConfigJSON, "SUPER_SECRET_WEBHOOK_TOKEN") || !strings.Contains(rawConfigJSON, secret.EncPrefix) {
		t.Fatalf("expected NotifyWebhookURL to be encrypted in kv[config], got %s", rawConfigJSON)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if strings.Contains(getW.Body.String(), "SUPER_SECRET_WEBHOOK_TOKEN") {
		t.Fatalf("GET /api/settings leaked plaintext webhook token: %s", getW.Body.String())
	}
	var gotSettings types.Config
	if err := json.Unmarshal(getW.Body.Bytes(), &gotSettings); err != nil {
		t.Fatalf("decode GET /api/settings: %v", err)
	}
	if gotSettings.NotifyWebhookURL != "https://oapi.dingtalk.com/••••••••" {
		t.Fatalf("unexpected masked NotifyWebhookURL: %q", gotSettings.NotifyWebhookURL)
	}

	// Submit masked webhook back to SaveSettings -> must succeed (200) and keep original secretWebhook!
	saveBytes, _ := json.Marshal(gotSettings)
	saveReq := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(saveBytes))
	saveReq.Header.Set("Content-Type", "application/json")
	saveW := httptest.NewRecorder()
	r.ServeHTTP(saveW, saveReq)
	if saveW.Code != http.StatusOK {
		t.Fatalf("SaveSettings with masked NotifyWebhookURL failed: %d (%s)", saveW.Code, saveW.Body.String())
	}
	if st.Config.Get().NotifyWebhookURL != secretWebhook {
		t.Fatalf("SaveSettings did not preserve NotifyWebhookURL: got %q, want %q", st.Config.Get().NotifyWebhookURL, secretWebhook)
	}
}

// BUG-SEC-08: Verify /api/version/check-update requires auth, caches for 15m with singleflight,
// caps body at 1 MiB, and does not reflect upstream status/body on non-200.
func TestSEC08_CheckUpdateAuthSingleflightCacheAndUpstreamConvergence(t *testing.T) {
	handlers.ResetUpdateCheckCache()
	t.Cleanup(handlers.ResetUpdateCheckCache)

	if _, inWhitelist := auth.DefaultWhitelist()["/api/version/check-update"]; inWhitelist {
		t.Fatalf("/api/version/check-update must NOT be in DefaultWhitelist()")
	}

	gin.SetMode(gin.TestMode)
	st, _ := newTestState(t)

	var upstreamHits atomic.Int32
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		time.Sleep(30 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","name":"Release 1.2.3","html_url":"https://github.com/gokele/ovh/releases/tag/v1.2.3"}`))
	}))
	defer mockUpstream.Close()
	t.Setenv("OVH_UPDATE_API", mockUpstream.URL)

	r := gin.New()
	r.Use(auth.Middleware(auth.Config{
		APIKey:         "admin-secret",
		Enabled:        true,
		WhitelistPaths: auth.DefaultWhitelist(),
	}))
	r.GET("/api/version/check-update", handlers.CheckUpdate(st))

	// 1. Unauthenticated request is rejected with 401
	unauthW := httptest.NewRecorder()
	r.ServeHTTP(unauthW, httptest.NewRequest(http.MethodGet, "/api/version/check-update", nil))
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated check-update, got %d", unauthW.Code)
	}

	// 2. 10 concurrent authenticated requests coalesce into 1 upstream call and populate 15m cache
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/version/check-update", nil)
			req.Header.Set("X-API-Key", "admin-secret")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	if hits := upstreamHits.Load(); hits != 1 {
		t.Fatalf("expected singleflight + cache to make exactly 1 upstream request, got %d", hits)
	}

	// 3. Upstream 403 error must converge to 502 without leaking upstream body
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded for internal-egress-ip-198.51.100.42"}`))
	}))
	defer errSrv.Close()
	t.Setenv("OVH_UPDATE_API", errSrv.URL)

	errReq := httptest.NewRequest(http.MethodGet, "/api/version/check-update", nil)
	errReq.Header.Set("X-API-Key", "admin-secret")
	errW := httptest.NewRecorder()
	r.ServeHTTP(errW, errReq)
	if errW.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 BadGateway on upstream 403, got %d (%s)", errW.Code, errW.Body.String())
	}
	if strings.Contains(errW.Body.String(), "198.51.100.42") {
		t.Fatalf("CheckUpdate leaked raw upstream body: %s", errW.Body.String())
	}

	// 4. Oversized >1MB response body is rejected with 502
	bigSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("A"), (1<<20)+1024))
	}))
	defer bigSrv.Close()
	t.Setenv("OVH_UPDATE_API", bigSrv.URL)

	bigReq := httptest.NewRequest(http.MethodGet, "/api/version/check-update", nil)
	bigReq.Header.Set("X-API-Key", "admin-secret")
	bigW := httptest.NewRecorder()
	r.ServeHTTP(bigW, bigReq)
	if bigW.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 BadGateway on oversized >1MB upstream body, got %d", bigW.Code)
	}
}

// BUG-SEC-09: Verify IsPrivateOrLocalIP blocks all special/mapped IPv4 & IPv6 ranges,
// and SharedWebhookTransport is a Proxy==nil singleton.
func TestSEC09_WebhookSSRFBlocksAllMappedRangesAndUsesNoProxySingleton(t *testing.T) {
	v4CompatLoopback := net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 127, 0, 0, 1}
	blockedCases := []struct {
		label string
		ip    net.IP
	}{
		{"IPv4-compatible IPv6 ::127.0.0.1", v4CompatLoopback},
		{"IPv4-mapped IPv6 ::ffff:127.0.0.1", net.ParseIP("::ffff:127.0.0.1")},
		{"NAT64 loopback 64:ff9b::7f00:1", net.ParseIP("64:ff9b::7f00:1")},
		{"6to4 loopback 2002:7f00:0001::", net.ParseIP("2002:7f00:0001::")},
		{"CGNAT 100.64.0.1", net.ParseIP("100.64.0.1")},
		{"Benchmarking 198.18.0.1", net.ParseIP("198.18.0.1")},
		{"Site-Local fec0::1", net.ParseIP("fec0::1")},
		{"Limited broadcast 255.255.255.255", net.ParseIP("255.255.255.255")},
	}
	for _, tc := range blockedCases {
		if !notify.IsPrivateOrLocalIP(tc.ip) {
			t.Fatalf("SSRF bypass: notify.IsPrivateOrLocalIP(%s [%s]) returned false, want true", tc.label, tc.ip.String())
		}
	}
	for _, pub := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if notify.IsPrivateOrLocalIP(net.ParseIP(pub)) {
			t.Fatalf("public IP %s was falsely blocked", pub)
		}
	}

	tr1 := notify.SharedWebhookTransport()
	tr2 := notify.SharedWebhookTransport()
	if tr1 == nil || tr1 != tr2 {
		t.Fatalf("expected singleton SharedWebhookTransport")
	}
	if tr1.Proxy != nil {
		t.Fatalf("expected SharedWebhookTransport.Proxy to be nil (must not inherit environment proxy)")
	}
}

// BUG-SEC-10 & REJECTED-03:
// 1. Mobile Bearer device token is blocked with 403 FORBIDDEN_DEVICE_TOKEN on all master-key admin endpoints.
// 2. POST /api/accounts/:id/proxy-check returns JSON object (preserving REJECTED-03 verification).
func TestSEC10_AppTokenBlockedFromAdminEndpointsAndProxyCheckShape(t *testing.T) {
	auth.ResetAuthFailures()
	t.Cleanup(auth.ResetAuthFailures)

	gin.SetMode(gin.TestMode)
	st, _ := newTestState(t)
	st.APIKey = "master-admin-secret-key-long-enough-2026"
	mon := monitor.New(st)

	code, _, err := st.DeviceRepo.CreatePairingCode()
	if err != nil {
		t.Fatalf("CreatePairingCode: %v", err)
	}
	rawAppToken, _, err := st.DeviceRepo.RedeemPairingCode(code, "TestPhone")
	if err != nil {
		t.Fatalf("RedeemPairingCode: %v", err)
	}

	if err := st.AccountService.UpsertAccount(types.OVHAccount{
		ID: "p1", Name: "ProxyCheckAcc", Zone: "IE", IAM: "go-ovh-ie", Endpoint: "ovh-eu",
		AppKey: "ak", AppSecret: "as", ConsumerKey: "ck",
		ProxyURL: "http://127.0.0.1:1",
	}); err != nil {
		t.Fatalf("UpsertAccount: %v", err)
	}
	_ = st.ReloadAccounts()

	engine := router.New(router.Config{
		State:      st,
		Monitor:    mon,
		EnableAuth: true,
	}, nil)

	// Mobile device token CAN read settings
	readReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	readReq.Header.Set("Authorization", "Bearer "+rawAppToken)
	readW := httptest.NewRecorder()
	engine.ServeHTTP(readW, readReq)
	if readW.Code != http.StatusOK {
		t.Fatalf("expected device token GET /api/settings to return 200, got %d", readW.Code)
	}

	// Mobile device token MUST receive 403 FORBIDDEN_DEVICE_TOKEN on all master-only endpoints
	adminEndpoints := [][2]string{
		{http.MethodPost, "/api/app/pairing-codes"},
		{http.MethodDelete, "/api/app/devices/1"},
		{http.MethodPost, "/api/version/update"},
		{http.MethodPost, "/api/settings"},
		{http.MethodPost, "/api/accounts"},
		{http.MethodPut, "/api/accounts/p1"},
		{http.MethodDelete, "/api/accounts/p1"},
		{http.MethodPost, "/api/accounts/p1/set-default"},
	}
	for _, ep := range adminEndpoints {
		req := httptest.NewRequest(ep[0], ep[1], strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+rawAppToken)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "FORBIDDEN_DEVICE_TOKEN") {
			t.Fatalf("expected 403 FORBIDDEN_DEVICE_TOKEN for mobile token on %s %s, got %d (%s)",
				ep[0], ep[1], w.Code, w.Body.String())
		}
	}

	// Master X-API-Key CAN invoke POST /api/app/pairing-codes
	masterReq := httptest.NewRequest(http.MethodPost, "/api/app/pairing-codes", nil)
	masterReq.Header.Set("X-API-Key", st.APIKey)
	masterW := httptest.NewRecorder()
	engine.ServeHTTP(masterW, masterReq)
	if masterW.Code != http.StatusOK {
		t.Fatalf("expected master X-API-Key on POST /api/app/pairing-codes to return 200, got %d (%s)", masterW.Code, masterW.Body.String())
	}

	// REJECTED-03: CheckAccountProxy returns JSON object
	pcReq := httptest.NewRequest(http.MethodPost, "/api/accounts/p1/proxy-check", nil)
	pcReq.Header.Set("X-API-Key", st.APIKey)
	pcW := httptest.NewRecorder()
	engine.ServeHTTP(pcW, pcReq)
	var decoded interface{}
	if err := json.Unmarshal(pcW.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode proxy-check: %v", err)
	}
	if _, isObj := decoded.(map[string]interface{}); !isObj {
		t.Fatalf("expected CheckAccountProxy to return JSON object, got %T", decoded)
	}
}

// BUG-SEC-11 & BUG-SEC-12: Verify deploy scripts enforce umask 077 / chmod 600 without sed injection,
// nginx/default.conf preserves security headers in /assets/ + blocks /api/internal, and vite.config.ts defaults to 127.0.0.1.
func TestSEC11_SEC12_ScriptsPermissionsNginxHeadersAndViteHost(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))

	for _, scriptRel := range []string{
		"scripts/linux-oneclick-deploy.sh",
		"scripts/docker-deploy.sh",
		"scripts/init-first-run.sh",
	} {
		b, err := os.ReadFile(filepath.Join(repoRoot, scriptRel))
		if err != nil {
			t.Fatalf("read %s: %v", scriptRel, err)
		}
		src := string(b)
		if !strings.Contains(src, "umask 077") || !strings.Contains(src, "chmod 600") {
			t.Fatalf("%s must enforce umask 077 and chmod 600 on .env", scriptRel)
		}
		if strings.Contains(src, `sed -i "s|^${k}=.*|`) || strings.Contains(src, `sed -i "s/API_SECRET_KEY=`) {
			t.Fatalf("%s still contains unescaped sed replacement on .env", scriptRel)
		}
	}

	nginxBytes, err := os.ReadFile(filepath.Join(repoRoot, "deploy", "nginx", "default.conf"))
	if err != nil {
		t.Fatalf("read deploy/nginx/default.conf: %v", err)
	}
	nginxConf := string(nginxBytes)
	if !strings.Contains(nginxConf, "location ^~ /api/internal") {
		t.Fatalf("deploy/nginx/default.conf missing location ^~ /api/internal 403 block")
	}
	assetsRe := regexp.MustCompile(`location /assets/\s*\{([\s\S]*?)\}`)
	m := assetsRe.FindStringSubmatch(nginxConf)
	if len(m) < 2 {
		t.Fatalf("deploy/nginx/default.conf missing location /assets/ block")
	}
	for _, hdr := range []string{"Cache-Control", "X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy"} {
		if !strings.Contains(m[1], hdr) {
			t.Fatalf("deploy/nginx/default.conf location /assets/ missing security header %s", hdr)
		}
	}

	viteBytes, err := os.ReadFile(filepath.Join(repoRoot, "web", "vite.config.ts"))
	if err != nil {
		t.Fatalf("read web/vite.config.ts: %v", err)
	}
	viteConf := string(viteBytes)
	if strings.Contains(viteConf, `host: "0.0.0.0"`) || !strings.Contains(viteConf, "127.0.0.1") {
		t.Fatalf("web/vite.config.ts must default server.host to 127.0.0.1 instead of 0.0.0.0")
	}
}
