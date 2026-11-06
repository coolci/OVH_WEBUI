package handlers

// Isolated request-contract regressions; no external network.
import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerSupportRollbackMissingConfirm(t *testing.T) {
	st, r, _ := supportFixture(t)
	r.POST("/vps-control/:service_name/snapshot/revert", RevertVpsSnapshot(st))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/vps-control/vps-fixture/snapshot/revert?account=eu", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "EFD5D37BB") {
		t.Fatalf("expected current confirmation mismatch, got %d %s", w.Code, w.Body.String())
	}
	t.Logf("Frontend-equivalent request: HTTP %d %s", w.Code, w.Body.String())
}

func TestServerSupportAccountDetailsAllFailed(t *testing.T) {
	st, r, _ := supportFixture(t)
	client, err := st.OVH.ClientFor("eu")
	if err != nil {
		t.Fatal(err)
	}
	client.Client.Transport = queuePreorderTransport(func(req *http.Request) (*http.Response, error) {
		status, body := 200, `[1001,1002]`
		if strings.HasSuffix(req.URL.Path, "/auth/time") {
			body = `1700000000`
		} else if strings.Contains(req.URL.Path, "/me/notification/email/history/") {
			status, body = 403, `{"message":"fixture: detail access denied"}`
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})
	r.GET("/emails", GetEmailHistory(st))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/emails?account=eu", nil))
	if w.Code != 502 || !strings.Contains(w.Body.String(), "读取失败") || w.Header().Get("X-Partial-Failures") != "2" {
		t.Fatalf("expected retryable detail error, got %d %s header=%s", w.Code, w.Body.String(), w.Header().Get("X-Partial-Failures"))
	}
	t.Logf("Two existing emails, both detail requests denied: HTTP %d %s X-Partial-Failures=%s", w.Code, w.Body.String(), w.Header().Get("X-Partial-Failures"))
}

func TestSnapshotConfirmedRequestReachesMockOnce(t *testing.T) {
	st, r, _ := supportFixture(t)
	client, err := st.OVH.ClientFor("eu")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client.Client.Transport = queuePreorderTransport(func(req *http.Request) (*http.Response, error) {
		body := `{"id":1}`
		if strings.HasSuffix(req.URL.Path, "/auth/time") {
			body = `1700000000`
		} else if req.Method == "POST" {
			calls++
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})
	r.POST("/vps-control/:service_name/snapshot/revert", RevertVpsSnapshot(st))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/vps-control/vps-fixture/snapshot/revert?account=eu", strings.NewReader(`{"confirm":true}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 || calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", w.Code, calls, w.Body.String())
	}
}
