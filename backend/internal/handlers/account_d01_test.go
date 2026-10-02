package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"
)

// TestCreateAccountInvalidCreds verifies PRD F01.3.3 / D-01:
// When an account is created with invalid OVH credentials:
// 1. The account is saved in SQLite (so the user can edit/fix it without re-typing everything).
// 2. cred_state is set to "invalid".
// 3. HTTP response is 422 Unprocessable Entity with remediation actions.
func TestCreateAccountInvalidCreds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestState(t)

	// Mock server that returns valid /auth/time but 403 on /me
	mockOVH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("1700000000"))
			return
		}
		if r.URL.Path == "/me" {
			w.Header().Set("X-OVH-Query-Id", "EU.ext-test-d01")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message":  "application key is invalid",
				"httpCode": "403 Forbidden",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockOVH.Close()

	// Redirect ovh-eu endpoint to mock server for this test
	origEndpoint := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = mockOVH.URL
	defer func() {
		ovhsdk.Endpoints["ovh-eu"] = origEndpoint
	}()

	r := gin.New()
	r.POST("/api/accounts", CreateAccount(st))

	body := map[string]interface{}{
		"name":        "Test Invalid Account",
		"zone":        "FR",
		"appKey":      "bad-ak",
		"appSecret":   "bad-as",
		"consumerKey": "bad-ck",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/accounts", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// 1. Must return 422 Unprocessable Entity
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected HTTP 422, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// 2. Response fields
	if resp["valid"] != false {
		t.Errorf("expected valid=false, got %v", resp["valid"])
	}
	if resp["credState"] != "invalid" {
		t.Errorf("expected credState=invalid, got %v", resp["credState"])
	}
	remediations, ok := resp["remediation"].([]interface{})
	if !ok || len(remediations) == 0 {
		t.Errorf("expected non-empty remediation list, got %v", resp["remediation"])
	}

	// 3. Verify account was saved in DB with cred_state = invalid
	accs, err := st.DB.ListAccounts()
	if err != nil {
		t.Fatalf("list accounts from DB: %v", err)
	}
	if len(accs) != 1 {
		t.Fatalf("expected 1 account in DB, got %d", len(accs))
	}
	if accs[0].CredState != "invalid" {
		t.Errorf("expected DB account cred_state=invalid, got %s", accs[0].CredState)
	}
}
