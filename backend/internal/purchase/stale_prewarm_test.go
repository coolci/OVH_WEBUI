package purchase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/types"
)

// 预热车在 OVH 那边已经没了(404):同一轮里立刻不用预热车重新装车并下单,
// 而不是记一次失败、等下一轮(默认 60 秒)。只重来一次,不会循环。
func TestStalePrewarmedCartIsRebuiltInSameAttempt(t *testing.T) {
	s := quickOrderTestState(t)
	ClearPrewarmPool(nil)
	resetEcoOptionsCache()
	s.Accounts = []types.OVHAccount{{ID: "stale-acc", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}

	var staleCheckouts, freshCheckouts, cartsCreated atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/dedicated/server/datacenter/availabilities":
			io.WriteString(w, `[{"fqn":"stale-plan.ram-64g","datacenters":[{"datacenter":"gra","availability":"1H"}]}]`)
		case r.URL.Path == "/order/cart/stale-cart/checkout":
			staleCheckouts.Add(1)
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"message":"The requested object (cartId = stale-cart) does not exist"}`)
		case r.URL.Path == "/order/cart/fresh-cart/checkout":
			freshCheckouts.Add(1)
			io.WriteString(w, `{"orderId":4242,"url":"https://stale.invalid/order"}`)
		case r.URL.Path == "/order/cart" && r.Method == http.MethodPost:
			cartsCreated.Add(1)
			io.WriteString(w, `{"cartId":"fresh-cart"}`)
		case strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodPost:
			io.WriteString(w, `{"itemId":1}`)
		case strings.HasSuffix(r.URL.Path, "/eco/options") && r.Method == http.MethodGet:
			io.WriteString(w, `[{"planCode":"ram-64g","prices":[]}]`)
		default:
			io.WriteString(w, `null`)
		}
	}))
	t.Cleanup(srv.Close)
	swapEndpoint(t, "ovh-eu", srv.URL)
	base := http.DefaultTransport
	transport := auditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.String(), srv.URL) {
			return base.RoundTrip(r)
		}
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			body := `{"plans":[{"planCode":"stale-plan","configurations":[{"name":"region","values":["europe"]}]}]}`
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		}
		return nil, fmt.Errorf("external request blocked: %s", r.URL)
	})
	http.DefaultTransport = transport
	restore := netfp.SwapSharedTransportForTesting(transport)
	t.Cleanup(func() { restore(); http.DefaultTransport = base; ClearPrewarmPool(nil); resetEcoOptionsCache() })

	item := &types.QueueItem{ID: "stale-task", AccountID: "stale-acc", PlanCode: "stale-plan", Datacenter: "gra",
		Options: []string{"ram-64g"}, Status: "running", Force: true}
	PutPrewarmedCart(&PrewarmedCart{CartID: "stale-cart", AccountID: "stale-acc", PlanCode: "stale-plan",
		Datacenter: "gra", Options: []string{"ram-64g"}, CreatedAt: time.Now()})

	out := PurchaseServer(context.Background(), s, item)
	if !out.Success {
		t.Fatalf("预热车失效后应在同一轮重新装车下单成功: %+v", out)
	}
	if staleCheckouts.Load() != 1 || freshCheckouts.Load() != 1 || cartsCreated.Load() != 1 {
		t.Fatalf("应先用预热车结账 1 次被拒,再新建 1 辆车结账 1 次: stale=%d fresh=%d carts=%d",
			staleCheckouts.Load(), freshCheckouts.Load(), cartsCreated.Load())
	}
}

// 只有"车本身没了 / 过期"的确定性拒绝才重新装车;超时、429、配置类错误一律不算
func TestStaleCartErrorClassification(t *testing.T) {
	type apiErr = struct {
		code int
		msg  string
		want bool
	}
	for _, c := range []apiErr{
		{404, "The requested object (cartId = x) does not exist", true},
		{400, "This cart has expired", true},
		{400, "24sk202.ram-64g is not available in gra", false},
		{429, "Too many requests", false},
		{408, "timeout", false},
		{500, "cart does not exist", false},
	} {
		got := staleCartError(fakeAPIError(c.code, c.msg))
		if got != c.want {
			t.Errorf("%d %q: got %v want %v", c.code, c.msg, got, c.want)
		}
	}
	if staleCartError(fmt.Errorf("dial tcp: i/o timeout")) {
		t.Error("传输层错误不能当成车失效")
	}
}

func fakeAPIError(code int, msg string) error {
	return &ovhsdk.APIError{Code: code, Message: msg}
}
