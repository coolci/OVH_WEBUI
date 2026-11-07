package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// 移植自上游 v0.4.18:不支持的下单约束必须在产生任何副作用之前被拒绝,
// 不能被静默丢掉后按默认配置下单。
func TestOrderRequestsRejectUnrecognizedConstraintsBeforeAnySideEffects(t *testing.T) {
	state, _ := newWatchTestMonitor(t)
	state.Accounts = []types.OVHAccount{{ID: "test", Endpoint: "ovh-eu", Zone: "IE", AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}
	state.Queue = []types.QueueItem{{ID: "existing", AccountID: "test", PlanCode: "27skle02-v1", Datacenter: "gra", Status: "paused", RetryInterval: 60}}
	before := state.Queue[0]
	original := http.DefaultTransport
	http.DefaultTransport = queuePreorderTransport(func(*http.Request) (*http.Response, error) {
		t.Error("invalid parameters reached an OVH API")
		return nil, io.ErrUnexpectedEOF
	})
	defer func() { http.DefaultTransport = original }()
	router := orderRouter(state)
	for _, route := range []struct {
		method, path string
		body         map[string]interface{}
	}{
		{"POST", "/queue", map[string]interface{}{"account_id": "test", "planCode": "27skle02-v1", "datacenter": "gra"}},
		{"PUT", "/queue/existing", map[string]interface{}{"datacenter": "gra", "retryInterval": 60}},
		{"POST", "/quick-order", map[string]interface{}{"account_id": "test", "planCode": "27skle02-v1", "datacenter": "gra", "options": []string{"ram-32g-ecc-2400"}}},
		{"POST", "/prewarm", map[string]interface{}{"account_id": "test", "planCode": "27skle02-v1", "datacenter": "gra"}},
	} {
		for _, field := range []string{"hwRam", "hwDisk", "allowFreqDrift", "maxPrice", "autoPya"} {
			t.Run(route.method+route.path+"/"+field, func(t *testing.T) {
				body := map[string]interface{}{field: "unsupported"}
				for name, value := range route.body {
					body[name] = value
				}
				record := sendJSON(router, route.method, route.path, body)
				if record.Code != http.StatusBadRequest || !strings.Contains(record.Body.String(), "unknown field") {
					t.Fatalf("ignored %s: %d %s", field, record.Code, record.Body.String())
				}
			})
		}
	}
	if len(state.Queue) != 1 || !reflect.DeepEqual(state.Queue[0], before) {
		t.Fatal("rejected parameters changed the queue")
	}
}

// 本地前端与 tools/catalog_watch 实际发送的请求体都不能被当成未知字段拒掉。
// (这些请求可能因目录/账户等其他原因失败,这里只核对不是解析层的拒绝。)
func TestOrderRequestsAcceptEveryFieldLocalClientsSend(t *testing.T) {
	state, _ := newWatchTestMonitor(t)
	state.Accounts = []types.OVHAccount{{ID: "test", Endpoint: "ovh-eu", Zone: "IE", AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}
	state.Queue = []types.QueueItem{{ID: "existing", AccountID: "test", PlanCode: "27skle02-v1", Datacenter: "gra", Status: "paused", RetryInterval: 60}}
	original := http.DefaultTransport
	http.DefaultTransport = queuePreorderTransport(func(*http.Request) (*http.Response, error) { return nil, io.ErrUnexpectedEOF })
	defer func() { http.DefaultTransport = original }()
	router := orderRouter(state)
	for name, route := range map[string]struct {
		method, path string
		body         map[string]interface{}
	}{
		// src/hooks/ovh/use-queue.ts useCreateQueueItem
		"web create": {"POST", "/queue", map[string]interface{}{"account_id": "test", "planCode": "27skle02-v1", "datacenter": "gra", "retryInterval": 30, "options": []string{}, "autoPay": false, "force": false, "coupon": "X"}},
		// src/lib/api.ts addQueueItem (accountId 兼容写法)
		"legacy create": {"POST", "/queue", map[string]interface{}{"accountId": "test", "account_id": "test", "planCode": "27skle02-v1", "datacenter": "gra", "retryInterval": 30}},
		// src/hooks/ovh/use-queue.ts useUpdateQueueItem
		"web edit": {"PUT", "/queue/existing", map[string]interface{}{"datacenter": "gra", "options": []string{}, "retryInterval": 60, "autoPay": false, "coupon": ""}},
		// tools/catalog_watch/watch.py
		"catalog watch": {"POST", "/quick-order", map[string]interface{}{"account_id": "test", "planCode": "27skle02-v1", "datacenter": "gra", "options": []string{}, "quantity": 1, "autoPay": false, "force": true}},
		// src/lib/api.ts quickOrder
		"web quick order": {"POST", "/quick-order", map[string]interface{}{"account_id": "test", "accountId": "test", "planCode": "27skle02-v1", "datacenter": "gra", "options": []string{}, "autoPay": false, "fromMonitor": false, "skipDuplicateCheck": false, "coupon": ""}},
		// src/hooks/use-prewarm.ts
		"web prewarm": {"POST", "/prewarm", map[string]interface{}{"account_id": "test", "planCode": "27skle02-v1", "datacenter": "gra", "options": []string{}, "coupon": ""}},
	} {
		t.Run(name, func(t *testing.T) {
			record := sendJSON(router, route.method, route.path, route.body)
			if strings.Contains(record.Body.String(), "unknown field") || strings.Contains(record.Body.String(), "one JSON value") {
				t.Fatalf("local client payload rejected by strict parsing: %d %s", record.Code, record.Body.String())
			}
		})
	}
}

func TestOrderJSONRequiresOneValueAndPreservesValidation(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{"planCode":"27skle02-v1"}`, true},
		{"{\"planCode\":\"27skle02-v1\"}\n\t", true},
		{`{"planCode":"27skle02-v1","maxPrice":1}`, false},
		{`{"planCode":"27skle02-v1"} {}`, false},
		{`{"planCode":"27skle02-v1"} trailing`, false},
		{`{}`, false},
		{``, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			var value struct {
				PlanCode string `json:"planCode" binding:"required"`
			}
			if err := bindOrderJSON(context, &value); (err == nil) != tc.ok {
				t.Fatalf("accepted=%v err=%v", err == nil, err)
			}
		})
	}
}

func orderRouter(st *app.State) *gin.Engine {
	router := gin.New()
	router.POST("/queue", AddQueueItem(st))
	router.PUT("/queue/:id", UpdateQueueItem(st))
	router.POST("/quick-order", QuickOrder(st))
	router.POST("/prewarm", PrewarmOrder(st))
	return router
}

func sendJSON(router *gin.Engine, method, path string, body map[string]interface{}) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	request := httptest.NewRequest(method, path, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	record := httptest.NewRecorder()
	router.ServeHTTP(record, request)
	return record
}
