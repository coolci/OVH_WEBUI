package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/types"
)

func TestGetSettings_MasksCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	rawSecret := "my-super-secret-ovh-key-12345"
	rawConsumer := "my-secret-consumer-key-67890"
	rawToken := "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"

	_ = st.Config.Set(types.Config{
		AppSecret:   rawSecret,
		ConsumerKey: rawConsumer,
		TgToken:     rawToken,
		AppKey:      "public-app-key",
	})

	router := gin.New()
	router.GET("/api/settings", GetSettings(st))

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp types.Config
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AppSecret == rawSecret || !isMasked(resp.AppSecret) {
		t.Fatalf("AppSecret should be masked, got %q", resp.AppSecret)
	}
	if resp.ConsumerKey == rawConsumer || !isMasked(resp.ConsumerKey) {
		t.Fatalf("ConsumerKey should be masked, got %q", resp.ConsumerKey)
	}
	if resp.TgToken == rawToken || !isMasked(resp.TgToken) {
		t.Fatalf("TgToken should be masked, got %q", resp.TgToken)
	}
}

func TestSaveSettings_PreservesMaskedCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	rawSecret := "original-secret-12345"
	rawConsumer := "original-consumer-67890"
	rawToken := "original-tg-token-xyz"

	_ = st.Config.Set(types.Config{
		AppSecret:   rawSecret,
		ConsumerKey: rawConsumer,
		TgToken:     rawToken,
		AppKey:      "original-app-key",
	})

	router := gin.New()
	router.POST("/api/settings", SaveSettings(st))

	// 模拟前端表单提交带有掩码的凭据，但修改了 AppKey
	body := types.Config{
		AppKey:      "updated-app-key",
		AppSecret:   "ori...345",
		ConsumerKey: "******",
		TgToken:     "ori...xyz",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := st.Config.Get()
	if saved.AppKey != "updated-app-key" {
		t.Fatalf("expected updated AppKey, got %q", saved.AppKey)
	}
	if saved.AppSecret != rawSecret {
		t.Fatalf("masked AppSecret should preserve original value %q, got %q", rawSecret, saved.AppSecret)
	}
	if saved.ConsumerKey != rawConsumer {
		t.Fatalf("masked ConsumerKey should preserve original value %q, got %q", rawConsumer, saved.ConsumerKey)
	}
	if saved.TgToken != rawToken {
		t.Fatalf("masked TgToken should preserve original value %q, got %q", rawToken, saved.TgToken)
	}
}

func TestSaveSettings_RejectsSSRFWebhook(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	router := gin.New()
	router.POST("/api/settings", SaveSettings(st))

	for _, badURL := range []string{
		"http://127.0.0.1:8080/hook",
		"http://localhost:8080/hook",
		"http://10.0.0.1/hook",
		"http://192.168.1.1/hook",
		"http://169.254.169.254/latest/meta-data/",
	} {
		body := types.Config{
			NotifyWebhookURL: badURL,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for SSRF target %q, got %d", badURL, w.Code)
		}
	}
}

// 用户实测场景: 账户设置为法国 (FR, go-ovh-fr)，但全局设置 GET /api/settings
// 仍返回旧的 IE / go-ovh-ie。修复后必须自动与默认账户同步。
func TestGetSettings_ReflectsDefaultAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	// 初始为 IE
	_ = st.Config.Set(types.Config{
		Zone:     "IE",
		IAM:      "go-ovh-ie",
		Endpoint: "ovh-eu",
	})

	// 设置默认账户为法国 (FR)
	st.Accounts = []types.OVHAccount{
		{
			ID:          "acc-fr",
			Name:        "French Account",
			Zone:        "FR",
			IAM:         "go-ovh-fr",
			Endpoint:    "ovh-eu",
			AppKey:      "fr-app-key",
			AppSecret:   "fr-secret",
			ConsumerKey: "fr-consumer",
			IsDefault:   true,
		},
	}

	st.SyncDefaultAccountConfig()

	router := gin.New()
	router.GET("/api/settings", GetSettings(st))

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp types.Config
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Zone != "FR" {
		t.Fatalf("expected Zone 'FR', got %q", resp.Zone)
	}
	if resp.IAM != "go-ovh-fr" {
		t.Fatalf("expected IAM 'go-ovh-fr', got %q", resp.IAM)
	}
	if st.Config.Get().Zone != "FR" {
		t.Fatalf("st.Config.Get().Zone should be 'FR', got %q", st.Config.Get().Zone)
	}
}
