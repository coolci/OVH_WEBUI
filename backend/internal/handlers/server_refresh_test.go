package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/types"
)

func TestServerRefreshMetadataPreservesAccountAndSnapshotAge(t *testing.T) {
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{
		{ID: "a", Endpoint: "ovh-eu", Zone: "IE", IsDefault: true},
		{ID: "b", Endpoint: "ovh-us", Zone: "US"},
	}
	old := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	fresh := time.Now().Truncate(time.Second)
	st.ServerCache.SetBucketAt(st.ServerCacheKey("a"), []types.ServerPlan{{PlanCode: "old-a"}}, old)
	st.ServerCache.SetBucketAt(st.ServerCacheKey("b"), []types.ServerPlan{{PlanCode: "fresh-b"}}, fresh)
	router := gin.New()
	router.GET("/servers", GetServers(st))
	for _, tc := range []struct {
		account, plan string
		ts            time.Time
		failed        bool
	}{
		{"a", "old-a", old, true}, {"b", "fresh-b", fresh, false},
	} {
		// No API refresh requested: exercise stale fallback without external requests.
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/servers?account="+tc.account, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
		}
		var body struct {
			Servers   []types.ServerPlan `json:"servers"`
			CacheInfo struct {
				Timestamp         float64 `json:"timestamp"`
				RefreshFailed     bool    `json:"refreshFailed"`
				UsingExpiredCache bool    `json:"usingExpiredCache"`
				CacheDuration     int     `json:"cacheDuration"`
			} `json:"cacheInfo"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Servers) != 1 || body.Servers[0].PlanCode != tc.plan {
			t.Fatalf("wrong account data: %+v", body)
		}
		if body.CacheInfo.Timestamp != float64(tc.ts.Unix()) || body.CacheInfo.RefreshFailed != tc.failed || body.CacheInfo.UsingExpiredCache != tc.failed || body.CacheInfo.CacheDuration != 120 {
			t.Fatalf("incorrect freshness: %+v", body.CacheInfo)
		}
	}
}
