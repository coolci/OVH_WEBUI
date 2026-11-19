package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
)

// Regression tests for precise subscription identity and multiple order accounts.
// The helper creates an isolated temporary SQLite database, with no real accounts.
func TestEmptyConfigurationDeletionPreservesSibling(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	mon.AddSubscription("audit-plan", nil, true, false, "", nil, nil, false, 0, "", false, []string{"ram-64g"})
	mon.AddSubscription("audit-plan", nil, true, false, "", nil, nil, false, 0, "", false, nil)
	r := gin.New()
	r.DELETE("/monitor/subscriptions/:planCode", RemoveSubscription(st, mon))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/monitor/subscriptions/audit-plan?optionsKey=", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	subs := mon.Snapshot()
	if len(subs) != 1 || monitor.OptionsKey(subs[0].Options) != "ram-64g" {
		t.Fatalf("unexpected: %+v", subs)
	}
}

type auditWatchTransport func(*http.Request) (*http.Response, error)

func (f auditWatchTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWatchAllKeepsEveryAccount(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	for _, id := range []string{"audit-one", "audit-two"} {
		if err := st.DB.UpsertAccount(types.OVHAccount{ID: id, Name: id, Endpoint: "ovh-eu", Zone: "IE"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.ReloadAccounts(); err != nil {
		t.Fatal(err)
	}
	restore := netfp.SwapSharedTransportForTesting(auditWatchTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"plans":[]}`)), Request: r}, nil
	}))
	defer restore()
	// No DC is selected so subscription prewarming returns immediately. No
	// notification provider is configured, so any loop stops before inventory IO.
	defer mon.Stop()
	for _, tc := range []struct {
		ref  string
		want int
	}{{"@all", 2}, {"", 2}, {"@1", 1}} {
		out := watchText(st, mon, []string{"audit-plan", tc.ref, "x1"})
		subs := mon.Snapshot()
		if len(subs) != 1 || len(monitor.NormalizeOrderAccounts(subs[0].AutoOrderAccountID, subs[0].AutoOrderAccountIDs)) != tc.want {
			t.Fatalf("ref=%q unexpected: %+v / %s", tc.ref, subs, out)
		}
	}

}

func TestExplicitEmptyConfigurationPausePreservesSibling(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	mon.AddSubscription("audit-plan", nil, true, false, "", nil, nil, false, 0, "", false, []string{"ram-64g"})
	mon.AddSubscription("audit-plan", nil, true, false, "", nil, nil, false, 0, "", false, nil)
	r := gin.New()
	r.POST("/monitor/subscriptions/:planCode/pause", PauseSubscription(st, mon))
	w := httptest.NewRecorder()
	// Empty optionsKey is intentionally different from a missing key.
	req := httptest.NewRequest(http.MethodPost, "/monitor/subscriptions/audit-plan/pause", strings.NewReader(`{"paused":true,"optionsKey":""}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	subs := mon.Snapshot()
	if len(subs) != 2 || subs[0].Paused || !subs[1].Paused {
		t.Fatalf("unexpected: %+v", subs)
	}
}
