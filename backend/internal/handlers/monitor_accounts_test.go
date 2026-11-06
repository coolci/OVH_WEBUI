package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/types"
)

func TestMonitorAccountListCreateAndPartialUpdates(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	st.Accounts = []types.OVHAccount{{ID: "a", Endpoint: "ovh-eu", Zone: "FI"}, {ID: "b", Endpoint: "ovh-eu", Zone: "FI"}}
	// Every network interaction is local or synthetic; no purchase worker runs.
	originalTransport := http.DefaultTransport
	http.DefaultTransport = queuePreorderTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "127.0.0.1" {
			return originalTransport.RoundTrip(r)
		}
		body := `[]`
		if strings.Contains(r.URL.Path, "/catalog/") {
			body = `{"plans":[]}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	defer netfp.SwapSharedTransportForTesting(http.DefaultTransport)()
	defer func() { http.DefaultTransport = originalTransport }()
	notify.AllowLocalWebhookForTesting = true
	defer func() { notify.AllowLocalWebhookForTesting = false }()
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"code":0}`) }))
	defer webhook.Close()
	st.Config.Set(types.Config{NotifyWebhookURL: webhook.URL})
	defer mon.Stop()
	router := gin.New()
	router.POST("/monitor", AddSubscription(st, mon))
	router.PUT("/monitor/:planCode", UpdateSubscription(st, mon))
	request := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		return w
	}
	if w := request("POST", "/monitor", `{"planCode":" custom-code ","autoOrder":true,"autoOrderAccountId":"missing-legacy","autoOrderAccountIds":[" a ","b","a"],"options":["custom-addon"],"quantity":2}`); w.Code != 200 {
		t.Fatalf("custom multi-account subscription rejected: %d %s", w.Code, w.Body.String())
	}
	mon.Stop()
	assertAccounts := func(want []string) {
		t.Helper()
		got := mon.FindSubscription("custom-code", "custom-addon")
		if got == nil || !reflect.DeepEqual(got.AutoOrderAccountIDs, want) {
			t.Fatalf("accounts = %+v; want %v", got, want)
		}
		reloaded := monitor.New(st)
		reloaded.LoadFromDB()
		stored := reloaded.FindSubscription("custom-code", "custom-addon")
		if stored == nil || !reflect.DeepEqual(stored.AutoOrderAccountIDs, want) {
			t.Fatalf("persisted accounts = %+v; want %v", stored, want)
		}
	}
	assertAccounts([]string{"a", "b"})
	if w := request("PUT", "/monitor/custom-code", `{"quantity":3}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	assertAccounts([]string{"a", "b"})
	for _, body := range []string{`{"autoOrderAccountIds":["a","deleted"]}`, `{"autoOrderAccountIds":"a"}`} {
		if w := request("PUT", "/monitor/custom-code", body); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
		assertAccounts([]string{"a", "b"})
	}
	if w := request("PUT", "/monitor/custom-code", `{"autoOrderAccountId":"b"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	assertAccounts([]string{"b"})
	if w := request("PUT", "/monitor/custom-code", `{"autoOrderAccountId":"a","autoOrderAccountIds":[]}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	assertAccounts([]string{})
	if got := mon.FindSubscription("custom-code", "custom-addon"); got.AutoOrderAccountID != "" {
		t.Fatal("cleared list fell back to legacy account", got)
	}
	if w := request("POST", "/monitor", `{"planCode":"rejected","autoOrderAccountIds":["a","deleted"]}`); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	if mon.FindSubscription("rejected") != nil {
		t.Fatal("invalid account list partially created a subscription")
	}
}

func TestMonitorLegacyEmptyAccountDoesNotSelectAll(t *testing.T) {
	st, _ := newWatchTestMonitor(t)
	st.Accounts = []types.OVHAccount{{ID: "a"}, {ID: "b"}}
	for _, ids := range [][]string{nil, {}} {
		got, err := validatedMonitorAccounts(st, "", ids)
		if err != nil || len(got) != 0 {
			t.Fatalf("legacy empty account became all accounts: %v %v", got, err)
		}
	}
}
