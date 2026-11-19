package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
)

func TestQueueCustomPlanDoesNotConfuseMissingCatalogWithUnsupportedFamily(t *testing.T) {
	oldTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = oldTransport }()
	http.DefaultTransport = queuePreorderTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"plans":[]}`
		if strings.Contains(r.URL.Path, "/availabilities") {
			plan := r.URL.Query().Get("planCode")
			body = `[]`
			regionMatches := r.URL.Host == "eu.api.ovh.com"
			if plan == "custom-US-only" {
				regionMatches = r.URL.Host == "api.us.ovhcloud.com"
			}
			if regionMatches {
				body = `[{"planCode":"` + plan + `"}]`
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	defer netfp.SwapSharedTransportForTesting(http.DefaultTransport)()
	st, _ := newWatchTestMonitor(t)
	// Use a subsidiary unused by other handler fixtures: catalog failures are
	// deliberately cached process-wide and must not alter this test's verdicts.
	st.Accounts = []types.OVHAccount{{ID: "custom-queue", Endpoint: "ovh-eu", Zone: "LT"}}
	router := gin.New()
	router.POST("/queue", AddQueueItem(st))
	for _, tc := range []struct {
		plan   string
		force  bool
		status int
	}{
		{"custom-QueueModel.LZ", false, 400},
		{"custom-QueueModel.LZ", true, 200},
		{"custom-US-only", true, 400},
		{"25scale99-v1", true, 400},
		{"plan-a,plan-b", true, 400},
	} {
		body := fmt.Sprintf(`{"account_id":"custom-queue","planCode":%q,"datacenter":"gra","force":%t,"options":["custom-addon"]}`, tc.plan, tc.force)
		req := httptest.NewRequest("POST", "/queue", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body.String())
		}
	}
	queue, err := st.DB.ListQueue()
	if err != nil || len(queue) != 1 || !queue[0].Force || queue[0].PlanCode != "custom-QueueModel.LZ" || queue[0].Options[0] != "custom-addon" {
		t.Fatalf("custom task not persisted correctly: %+v %v", queue, err)
	}
}
