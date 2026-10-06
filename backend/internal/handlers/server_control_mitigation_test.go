package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/types"
)

func TestMitigationErrorFieldsIPObjectNotFound(t *testing.T) {
	apiErr := &ovhsdk.APIError{
		Code:    http.StatusNotFound,
		Message: "The requested object (ip = 192.0.2.9/32) does not exist",
		QueryID: "EU.ext-mitigation-test",
	}
	// Preserve the complete diagnostic even if the OVH failure was wrapped.
	err := fmt.Errorf("read mitigation: %w", apiErr)
	fields := mitigationErrorFields(err)
	if fields["errorCode"] != "IP_OBJECT_NOT_FOUND" {
		t.Fatalf("expected IP_OBJECT_NOT_FOUND, got %v", fields)
	}
	if fields["detail"] != err.Error() {
		t.Errorf("original diagnostic was lost: got %v, want %q", fields["detail"], err.Error())
	}
	if fields["queryId"] != apiErr.QueryID {
		t.Errorf("OVH query ID was lost: got %v, want %q", fields["queryId"], apiErr.QueryID)
	}
	message, ok := fields["error"].(string)
	if !ok || !strings.Contains(message, "未知") {
		t.Errorf("missing IP object must report unknown status, got %v", fields["error"])
	}
}

func TestMitigationErrorFieldsDoesNotMisclassifyOtherFailures(t *testing.T) {
	missingMessage := "The requested object (ip = 192.0.2.9/32) does not exist"
	tests := []struct {
		name string
		err  error
	}{
		{"forbidden", &ovhsdk.APIError{Code: http.StatusForbidden, Message: missingMessage}},
		{"rate limited", &ovhsdk.APIError{Code: http.StatusTooManyRequests, Message: missingMessage}},
		{"different object", &ovhsdk.APIError{Code: http.StatusNotFound, Message: "The requested object (service = ns-test) does not exist"}},
		{"different IP error", &ovhsdk.APIError{Code: http.StatusNotFound, Message: "The requested object (ip = 192.0.2.9/32) cannot be read"}},
		{"network", errors.New("dial tcp: connection refused")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := mitigationErrorFields(tt.err)
			if fields["errorCode"] == "IP_OBJECT_NOT_FOUND" {
				t.Errorf("unrelated failure was classified as missing IP object: %v", fields)
			}
			if fields["detail"] != tt.err.Error() {
				t.Errorf("original diagnostic was lost: got %v, want %q", fields["detail"], tt.err.Error())
			}
			if message, ok := fields["error"].(string); !ok || message == "" {
				t.Errorf("missing error message: %v", fields)
			}
		})
	}
}

func TestPermanentMitigationRemovedWithoutOVHClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		method  string
		handler gin.HandlerFunc
	}{
		{http.MethodPost, EnableMitigation(nil)},
		{http.MethodDelete, DisableMitigation(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			router := gin.New()
			router.Handle(tt.method, "/api/server-control/:service_name/mitigation/:ip", tt.handler)
			// A nil state would panic if the handler tried to resolve an OVH client.
			for _, query := range []string{"?block=192.0.2.9%2F32", ""} {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(tt.method, "/api/server-control/ns-test/mitigation/192.0.2.9"+query, nil)
				router.ServeHTTP(w, req)
				if w.Code != http.StatusGone {
					t.Fatalf("expected HTTP 410 for query %q, got %d: %s", query, w.Code, w.Body.String())
				}
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if response["success"] != false {
					t.Errorf("removed operation must not report success: %v", response)
				}
				if response["code"] != "PERMANENT_MITIGATION_REMOVED" {
					t.Errorf("missing stable removed-operation code: %v", response)
				}
				if message, ok := response["error"].(string); !ok || message == "" {
					t.Errorf("missing explanation of removed operation: %v", response)
				}
			}
		})
	}
}

func TestGetMitigationRetainsIPObjectFailureAndSkipsIPv6(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{{
		ID: "acc-mitigation-test", Name: "Mock OVH account", Zone: "FR", Endpoint: "ovh-eu",
		AppKey: "ak-test", AppSecret: "as-test", ConsumerKey: "ck-test", IsDefault: true,
	}}
	const ipv4Block = "192.0.2.9/32"
	const ipv6Block = "2001:db8::1/128"
	const queryID = "EU.ext-mitigation-list-test"
	const missingMessage = "The requested object (ip = 192.0.2.9/32) does not exist"
	var mu sync.Mutex
	var ipRequests []string
	mockOVH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			_, _ = w.Write([]byte("1700000000"))
		case "/dedicated/server/ns-test/ips":
			_ = json.NewEncoder(w).Encode([]string{ipv4Block, ipv6Block})
		default:
			mu.Lock()
			ipRequests = append(ipRequests, r.Method+" "+r.URL.Path)
			mu.Unlock()
			if r.Method == http.MethodGet && r.URL.Path == "/ip/"+ipv4Block+"/mitigation" {
				// go-ovh reads X-Ovh-QueryID (no hyphen between Query and ID).
				w.Header().Set("X-Ovh-QueryID", queryID)
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": missingMessage})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "unexpected request"})
		}
	}))
	defer mockOVH.Close()
	originalEndpoint := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = mockOVH.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = originalEndpoint }()

	router := gin.New()
	router.GET("/api/server-control/:service_name/mitigation", GetMitigation(st))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/server-control/ns-test/mitigation", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 with per-IP diagnostics, got %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Success bool                     `json:"success"`
		IPs     []map[string]interface{} `json:"ips"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || len(response.IPs) != 2 {
		t.Fatalf("failed IP rows must remain in response: %s", w.Body.String())
	}
	ipv4, ipv6 := response.IPs[0], response.IPs[1]
	if ipv4["ipBlock"] != ipv4Block || ipv4["errorCode"] != "IP_OBJECT_NOT_FOUND" || ipv4["queryId"] != queryID {
		t.Errorf("IPv4 row lost its identity or diagnostics: %v", ipv4)
	}
	if message, ok := ipv4["error"].(string); !ok || !strings.Contains(message, "未知") {
		t.Errorf("IPv4 failure must report unknown status: %v", ipv4)
	}
	if detail, ok := ipv4["detail"].(string); !ok || !strings.Contains(detail, missingMessage) || !strings.Contains(detail, queryID) {
		t.Errorf("IPv4 row lost raw OVH diagnostic: %v", ipv4)
	}
	if mitigations, ok := ipv4["mitigations"].([]interface{}); !ok || len(mitigations) != 0 {
		t.Errorf("expected empty mitigation records alongside explicit error: %v", ipv4)
	}
	if ipv6["ipBlock"] != ipv6Block || ipv6["error"] != nil {
		t.Errorf("IPv6 row should remain without an IPv4 endpoint error: %v", ipv6)
	}
	if note, ok := ipv6["note"].(string); !ok || note == "" {
		t.Errorf("IPv6 skip must have an explanatory note: %v", ipv6)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(ipRequests) != 1 || ipRequests[0] != "GET /ip/"+ipv4Block+"/mitigation" {
		t.Errorf("only the IPv4 mitigation endpoint should be called: %v", ipRequests)
	}
}

func TestGetMitigationRetainsFailedDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{{
		ID: "acc-mitigation-detail-test", Name: "Mock OVH account", Zone: "FR", Endpoint: "ovh-eu",
		AppKey: "ak-test", AppSecret: "as-test", ConsumerKey: "ck-test", IsDefault: true,
	}}
	const ip = "192.0.2.10"
	const block = ip + "/32"
	const emptyBlock = "192.0.2.11/32"
	apiErr := &ovhsdk.APIError{
		Code: http.StatusForbidden, Message: "This call has not been granted", QueryID: "EU.ext-mitigation-detail-test",
	}
	mockOVH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			_, _ = w.Write([]byte("1700000000"))
		case "/dedicated/server/ns-test/ips":
			_ = json.NewEncoder(w).Encode([]string{block, emptyBlock})
		case "/ip/" + block + "/mitigation":
			_ = json.NewEncoder(w).Encode([]string{ip})
		case "/ip/" + emptyBlock + "/mitigation":
			_ = json.NewEncoder(w).Encode([]string{})
		case "/ip/" + block + "/mitigation/" + ip:
			w.Header().Set("X-Ovh-QueryID", apiErr.QueryID)
			w.WriteHeader(apiErr.Code)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": apiErr.Message})
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "unexpected request"})
		}
	}))
	defer mockOVH.Close()
	originalEndpoint := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = mockOVH.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = originalEndpoint }()

	router := gin.New()
	router.GET("/api/server-control/:service_name/mitigation", GetMitigation(st))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/server-control/ns-test/mitigation", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 with per-IP detail diagnostics, got %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Success bool                     `json:"success"`
		IPs     []map[string]interface{} `json:"ips"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || len(response.IPs) != 2 {
		t.Fatalf("IP rows were lost after a detail failure: %s", w.Body.String())
	}
	row := response.IPs[0]
	if row["ipBlock"] != block || row["error"] != nil {
		t.Fatalf("successful list must preserve its IP row: %v", row)
	}
	mitigations, ok := row["mitigations"].([]interface{})
	if !ok || len(mitigations) != 1 {
		t.Fatalf("failed detail must remain in mitigation list, got %v", row)
	}
	detail, ok := mitigations[0].(map[string]interface{})
	if !ok {
		t.Fatalf("invalid detail response: %v", mitigations[0])
	}
	if detail["ipOnMitigation"] != ip || detail["state"] != "unknown" {
		t.Errorf("failed detail must retain IP and report unknown state: %v", detail)
	}
	if detail["detail"] != apiErr.Error() || detail["_detailError"] != apiErr.Error() || detail["queryId"] != apiErr.QueryID {
		t.Errorf("detail failure lost original diagnostic or query ID: %v", detail)
	}
	if message, ok := detail["error"].(string); !ok || message == "" {
		t.Errorf("detail failure needs an explicit error for the UI: %v", detail)
	}
	if detail["errorCode"] == "IP_OBJECT_NOT_FOUND" || detail["upstreamStatus"] != float64(http.StatusForbidden) {
		t.Errorf("detail 403 was misclassified: %v", detail)
	}
	// A genuinely empty successful list remains distinct from the failed detail.
	emptyRow := response.IPs[1]
	emptyMitigations, ok := emptyRow["mitigations"].([]interface{})
	if emptyRow["ipBlock"] != emptyBlock || emptyRow["error"] != nil || !ok || len(emptyMitigations) != 0 {
		t.Errorf("successful empty list must remain an error-free row: %v", emptyRow)
	}
}
