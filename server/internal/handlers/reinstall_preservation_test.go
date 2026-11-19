package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/types"
	ovhsdk "github.com/ovh/go-ovh/ovh"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReinstallPreservationUpstreamContract(t *testing.T) {
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "preserve-fixture", Endpoint: "ovh-eu", Zone: "IE", AppKey: "fixture", AppSecret: "fixture", ConsumerKey: "fixture", IsDefault: true}}
	calls := 0
	var payload map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		if r.Method == "POST" && r.URL.Path == "/dedicated/server/fixture/reinstall" {
			calls++
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			fmt.Fprint(w, `{"taskId":123}`)
			return
		}
		t.Errorf("unexpected mock request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(500)
	}))
	defer upstream.Close()
	original := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = upstream.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = original }()
	router := gin.New()
	router.POST("/install/:service_name", InstallOS(st))
	valid := `[{"diskGroupId":1,"partitioning":{"schemeName":"default"}},{"diskGroupId":2,"erase":false}]`
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"keep only", `{"templateName":"debian12_64","storageConfig":[{"diskGroupId":2,"erase":false}]}`, 400},
		{"ZFS override", `{"templateName":"debian12_64","useProxmox9Zfs":true,"storageConfig":` + valid + `}`, 400},
		{"valid", `{"templateName":"debian12_64","storageConfig":` + valid + `}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := calls
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/install/fixture", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			if w.Code != tc.code {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
			if tc.code == 400 && calls != before {
				t.Fatal("invalid preservation reached OVH")
			}
		})
	}
	if calls != 1 {
		t.Fatalf("upstream calls=%d", calls)
	}
	storage := payload["storage"].([]interface{})
	keep := storage[1].(map[string]interface{})
	if keep["diskGroupId"] != float64(2) || keep["erase"] != false {
		t.Fatalf("preservation lost: %+v", payload)
	}
}
