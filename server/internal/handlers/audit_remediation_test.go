package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/types"
)

func TestUpdateAccount_PreservesMaskedCredsAndScrubbedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	acc := types.OVHAccount{
		ID:          "acc-mask-1",
		Name:        "Primary FR",
		Endpoint:    "ovh-eu",
		Zone:        "FR",
		AppKey:      "real-app-key-123456",
		AppSecret:   "real-app-secret-654321",
		ConsumerKey: "real-consumer-key-999999",
		ProxyURL:    "http://alice:supersecret@10.0.0.2:8080",
		IsDefault:   true,
	}
	if err := st.AccountService.UpsertAccount(acc); err != nil {
		t.Fatalf("UpsertAccount failed: %v", err)
	}
	_ = st.ReloadAccounts()

	router := gin.New()
	router.PUT("/api/accounts/:id", UpdateAccount(st))

	payload := map[string]interface{}{
		"name":        "Primary FR Renamed",
		"endpoint":    "ovh-eu",
		"zone":        "FR",
		"appKey":      "re***56",
		"appSecret":   "re••••••21",
		"consumerKey": "re***99",
		"proxyUrl":    "http://alice:***@10.0.0.2:8080",
	}
	bodyBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/accounts/acc-mask-1?verify=false", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	updated, ok := st.FindAccount("acc-mask-1")
	if !ok {
		t.Fatal("account not found after update")
	}
	if updated.Name != "Primary FR Renamed" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}
	if updated.AppKey != "real-app-key-123456" {
		t.Fatalf("masked AppKey overwrote original value: got %q", updated.AppKey)
	}
	if updated.AppSecret != "real-app-secret-654321" {
		t.Fatalf("masked AppSecret overwrote original value: got %q", updated.AppSecret)
	}
	if updated.ConsumerKey != "real-consumer-key-999999" {
		t.Fatalf("masked ConsumerKey overwrote original value: got %q", updated.ConsumerKey)
	}
	if updated.ProxyURL != "http://alice:supersecret@10.0.0.2:8080" {
		t.Fatalf("scrubbed ProxyURL overwrote original password: got %q", updated.ProxyURL)
	}
}

func TestValidateServiceName_RejectsDotTraversalAndIncludesCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/servers/:serviceName", ValidateServiceName(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for _, bad := range []string{".", ".."} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "service_name", Value: bad}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/servers/"+bad, nil)
		ValidateServiceName()(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("serviceName %q: expected 400, got %d", bad, w.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("serviceName %q: invalid json: %v", bad, err)
		}
		if resp["code"] != "E9A400893" {
			t.Fatalf("serviceName %q: expected code E9A400893, got %v", bad, resp["code"])
		}
	}
}

func TestGetLogs_FilteringAndOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	st.Logger.Info("info message 1", "monitor")
	st.Logger.Error("error message 1", "purchase")
	st.Logger.Warn("warn message 1", "monitor")

	router := gin.New()
	router.GET("/api/logs", GetLogs(st))

	req := httptest.NewRequest(http.MethodGet, "/api/logs?level=ERROR&source=purchase&limit=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var entries []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal logs failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 filtered log entry, got %d", len(entries))
	}
	if entries[0]["level"] != "ERROR" || entries[0]["source"] != "purchase" {
		t.Fatalf("unexpected log entry: %+v", entries[0])
	}
}

func TestIsMasked_DoesNotMatchPlainURLs(t *testing.T) {
	if isMasked("https://hooks.slack.com/services/T00/B00/X00...123") {
		t.Fatal("URL with:// and ... but no *** should not be treated as a masked credential")
	}
	if !isMasked("ab***yz") {
		t.Fatal("ab***yz should be treated as masked")
	}
	if !isMasked(strings.Repeat("*", 6)) {
		t.Fatal("****** should be treated as masked")
	}
}
