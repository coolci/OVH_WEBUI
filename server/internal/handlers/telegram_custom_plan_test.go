package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
	ovhsdk "github.com/ovh/go-ovh/ovh"
)

func TestTelegramQuickOrderCustomPlanAndAddonsWithoutCatalog(t *testing.T) {
	const planCode = "cloud-NewModel.LZ"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case "/dedicated/server/datacenter/availabilities":
			if r.URL.Query().Get("planCode") != planCode {
				t.Errorf("wrong planCode: %s", r.URL)
			}
			fmt.Fprintf(w, `[{"fqn":%q,"memory":"ram-64g","storage":"softraid-2x960nvme","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`, planCode+".ram-64g.softraid-2x960nvme")
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	oldEndpoint := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = oldEndpoint }()
	oldTransport := http.DefaultTransport
	http.DefaultTransport = queuePreorderTransport(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"plans":[]}`)), Request: r}, nil
		}
		return nil, fmt.Errorf("external request blocked: %s", r.URL)
	})
	defer netfp.SwapSharedTransportForTesting(http.DefaultTransport)()
	defer func() { http.DefaultTransport = oldTransport }()
	for _, explicitOptions := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicitOptions=%v", explicitOptions), func(t *testing.T) {
			st, mon := newWatchTestMonitor(t)
			st.Accounts = []types.OVHAccount{{ID: "cloud-custom", Name: "Cloud", Endpoint: "ovh-eu", Zone: "FI", AppKey: "test", AppSecret: "test", ConsumerKey: "test", IsDefault: true}}
			options := []string{}
			want := []string{"ram-64g", "softraid-2x960nvme"}
			if explicitOptions {
				options = []string{"ram-64g-" + planCode, "softraid-2x960nvme-" + planCode}
				want = options
			}
			body, err := json.Marshal(map[string]any{"mode": "buy", "planCode": planCode, "datacenter": "gra", "quantity": 2, "accountRef": "1", "options": options})
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.POST("/cloud", TelegramQuickOrder(st, mon))
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/cloud", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			var response struct {
				Success bool `json:"success"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if w.Code != 200 || !response.Success {
				t.Fatalf("custom cloud order rejected: %d %s", w.Code, w.Body.String())
			}
			queue, err := st.DB.ListQueue()
			if err != nil || len(queue) != 2 {
				t.Fatalf("tasks=%v err=%v", queue, err)
			}
			for _, item := range queue {
				if item.PlanCode != planCode || !reflect.DeepEqual(item.Options, want) || !item.Force || item.AutoPay || item.AccountID != "cloud-custom" {
					t.Fatalf("wrong custom cloud task: %+v", item)
				}
			}
		})
	}
}
