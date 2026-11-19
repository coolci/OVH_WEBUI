package purchase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
)

func TestCustomPlanRetryRetainsRegionAndProductGuards(t *testing.T) {
	// The signed inventory is momentarily empty, while the public regional probe
	// still knows the custom product. No cart or checkout is permitted by this mock.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case "/dedicated/server/datacenter/availabilities":
			io.WriteString(w, `[]`)
		default:
			t.Errorf("unexpected purchase request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	swapEndpoint(t, "ovh-eu", srv.URL)
	oldTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = oldTransport }()
	http.DefaultTransport = auditTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"plans":[]}`
		if strings.Contains(r.URL.Path, "/availabilities") {
			plan := r.URL.Query().Get("planCode")
			matches := r.URL.Host == "eu.api.ovh.com"
			if plan == "custom-retry-US" {
				matches = r.URL.Host == "api.us.ovhcloud.com"
			}
			body = `[]`
			if matches {
				body = `[{"planCode":"` + plan + `"}]`
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	defer netfp.SwapSharedTransportForTesting(http.DefaultTransport)()
	for _, tc := range []struct {
		plan         string
		force, fatal bool
	}{
		{"custom-retry", true, false},
		{"custom-retry", false, true},
		{"custom-retry-US", true, true},
		{"25scale97-v1", true, true},
	} {
		t.Run(fmt.Sprintf("%s/force=%v", tc.plan, tc.force), func(t *testing.T) {
			st := quickOrderTestState(t)
			st.Accounts = []types.OVHAccount{{ID: "custom-retry", Endpoint: "ovh-eu", Zone: "FI", AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}
			item := types.QueueItem{ID: "retry-task", AccountID: "custom-retry", PlanCode: tc.plan, Datacenter: "gra", Force: tc.force}
			got := PurchaseServer(context.Background(), st, &item)
			if got.Fatal != tc.fatal || got.Success || got.Attempted {
				t.Fatalf("incorrect custom retry: %+v", got)
			}
		})
	}
}
