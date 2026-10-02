package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/types"
)

// TestVpsRebuildEndpoints verifies:
// 1. Both /api/vps-control/:service_name/rebuild and /reinstall route to ReinstallVps.
// 2. Upstream request goes to OVH /vps/{name}/rebuild (never legacy /reinstall).
// 3. imageId (and legacy templateId) are normalized as string imageId.
// 4. Missing imageId/templateId returns HTTP 400 Bad Request.
func TestVpsRebuildEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestState(t)

	// Add test account
	st.Accounts = []types.OVHAccount{
		{
			ID:          "acc-test-rebuild",
			Name:        "Test Account",
			Zone:        "FR",
			Endpoint:    "ovh-eu",
			AppKey:      "ak-test",
			AppSecret:   "as-test",
			ConsumerKey: "ck-test",
			IsDefault:   true,
		},
	}

	var lastOvhPath string
	var lastOvhMethod string
	var lastOvhBody map[string]interface{}

	mockOVH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("1700000000"))
			return
		}
		if r.URL.Path == "/vps/vps-test-rebuild/rebuild" && r.Method == http.MethodPost {
			lastOvhPath = r.URL.Path
			lastOvhMethod = r.Method
			bodyBytes, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(bodyBytes, &lastOvhBody)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":        888123,
				"action":    "rebuildVm",
				"status":    "todo",
				"type":      "rebuildVm",
				"progress":  0,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockOVH.Close()

	origEndpoint := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = mockOVH.URL
	defer func() {
		ovhsdk.Endpoints["ovh-eu"] = origEndpoint
	}()

	r := gin.New()
	r.POST("/api/vps-control/:service_name/rebuild", ReinstallVps(st))
	r.POST("/api/vps-control/:service_name/reinstall", ReinstallVps(st))

	// Case 1: Rebuild via /rebuild with imageId and sshKey
	{
		payload := map[string]interface{}{
			"imageId":           "debian12_64",
			"sshKey":            []string{"dev-key"},
			"doNotSendPassword": true,
		}
		payloadBytes, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/vps-control/vps-test-rebuild/rebuild", bytes.NewReader(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d. Body: %s", w.Code, w.Body.String())
		}
		if lastOvhPath != "/vps/vps-test-rebuild/rebuild" || lastOvhMethod != http.MethodPost {
			t.Fatalf("expected OVH call to POST /vps/vps-test-rebuild/rebuild, got %s %s", lastOvhMethod, lastOvhPath)
		}
		if lastOvhBody["imageId"] != "debian12_64" {
			t.Errorf("expected OVH imageId=debian12_64, got %v", lastOvhBody["imageId"])
		}
		if lastOvhBody["sshKey"] != "dev-key" {
			t.Errorf("expected OVH sshKey=dev-key, got %v", lastOvhBody["sshKey"])
		}
		if lastOvhBody["doNotSendPassword"] != true {
			t.Errorf("expected OVH doNotSendPassword=true, got %v", lastOvhBody["doNotSendPassword"])
		}
	}

	// Case 2: Rebuild via legacy /reinstall route with numeric templateId
	{
		lastOvhPath = ""
		payload := map[string]interface{}{
			"templateId":        99901,
			"doNotSendPassword": false,
		}
		payloadBytes, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/vps-control/vps-test-rebuild/reinstall", bytes.NewReader(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200 for legacy route, got %d. Body: %s", w.Code, w.Body.String())
		}
		// Must still call /vps/.../rebuild on OVH
		if lastOvhPath != "/vps/vps-test-rebuild/rebuild" {
			t.Fatalf("expected OVH call to /vps/.../rebuild, got %s", lastOvhPath)
		}
		// templateId must have been normalized to string "99901"
		if lastOvhBody["imageId"] != "99901" {
			t.Errorf("expected OVH imageId='99901', got %v", lastOvhBody["imageId"])
		}
	}

	// Case 3: Missing imageId / templateId returns 400 Bad Request
	{
		payload := map[string]interface{}{
			"doNotSendPassword": true,
		}
		payloadBytes, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/vps-control/vps-test-rebuild/rebuild", bytes.NewReader(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400 for missing imageId, got %d. Body: %s", w.Code, w.Body.String())
		}
	}
}
