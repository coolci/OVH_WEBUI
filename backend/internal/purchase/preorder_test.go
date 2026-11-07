package purchase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

type preorderTransport func(*http.Request) (*http.Response, error)

func (f preorderTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// 预订不再等公开销售目录:目录未收录、目录拉取失败(429)时照样去试购物车,
// 以真实下单接口为准。与普通任务的区别只在硬件匹配:预订保存的每个硬件段都必须
// 原样出现在库存 FQN 里(普通任务会忽略本轮库存里没出现的段)。
func TestPreorderTriesLiveCartWithoutWaitingForCatalog(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, tc := range []struct {
		sub            string
		listed, outage bool
		options        []string
		// 预订/普通任务各自是否应去建车
		preorderCart, normalCart bool
	}{
		{"IE", false, false, nil, true, true},
		{"DE", false, true, nil, true, true},
		{"FR", true, false, nil, true, true},
		{"ES", true, false, []string{"ram-32g"}, true, true},
		// 库存里根本没有 ram-64g:普通任务忽略这个段照样建车,预订必须继续等
		{"NL", true, false, []string{"ram-64g"}, false, true},
		// softraid 段本轮库存里没有:普通任务忽略它照下,预订绝不能改买只有 ram-32g 的那套
		{"GB", true, false, []string{"ram-32g", "softraid-2x480ssd"}, false, true},
	} {
		t.Run(tc.sub, func(t *testing.T) {
			dir := t.TempDir()
			database, err := db.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			lg := logger.New(filepath.Join(dir, "test.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
			st := app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
			st.Accounts = []types.OVHAccount{{ID: "eu", Endpoint: "ovh-eu", Zone: tc.sub, AppKey: "test", AppSecret: "test", ConsumerKey: "test"}}
			carts, checkouts := 0, 0
			transport := preorderTransport(func(r *http.Request) (*http.Response, error) {
				body, status := "{}", http.StatusOK
				switch {
				case strings.Contains(r.URL.Path, "/order/catalog/public/eco"):
					body = `{"plans":[]}`
					if tc.outage {
						status = http.StatusTooManyRequests
					}
					if tc.listed {
						body = `{"plans":[{"planCode":"27skle03-v1"}]}`
					}
				case strings.HasSuffix(r.URL.Path, "/auth/time"):
					body = fmt.Sprint(time.Now().Unix())
				case strings.Contains(r.URL.Path, "/dedicated/server/datacenter/availabilities"):
					body = `[{"fqn":"27skle03-v1.ram-32g","datacenters":[{"datacenter":"gra","availability":"1H-low"}]}]`
				case strings.HasSuffix(r.URL.Path, "/order/cart"):
					carts++
					status, body = http.StatusBadRequest, `{"message":"test stops at cart creation"}`
				default:
					if strings.Contains(r.URL.Path, "checkout") {
						checkouts++
					}
					return nil, fmt.Errorf("unexpected test request: %s", r.URL)
				}
				return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			http.DefaultTransport = transport // public catalog; every request stays inside this fixture
			client, err := st.OVH.ClientFor("eu")
			if err != nil {
				t.Fatal(err)
			}
			client.Client = &http.Client{Transport: transport}
			for _, preorder := range []bool{false, true} {
				carts = 0
				item := &types.QueueItem{ID: "pre", AccountID: "eu", PlanCode: "27skle03-v1", Datacenter: "gra", Preorder: preorder, MaxRetries: 1, Options: tc.options}
				outcome := PurchaseServer(context.Background(), st, item)
				want := tc.normalCart
				if preorder {
					want = tc.preorderCart
				}
				if (carts == 1) != want {
					t.Fatalf("preorder=%v: carts=%d want cart=%v outcome=%+v", preorder, carts, want, outcome)
				}
				if !want && (outcome.Attempted || outcome.Fatal) {
					t.Fatalf("preorder=%v: a missing configuration is only 'no stock yet': %+v", preorder, outcome)
				}
			}
			if checkouts != 0 {
				t.Fatal("fixture must never reach checkout")
			}
		})
	}
}

func TestPreorderRequiresAllExactHardwareSegments(t *testing.T) {
	const fqn = "27skle03-v1.ram-64g-ddr4-ecc-2666.softraid-2x480ssd"
	for _, tc := range []struct {
		options []string
		want    bool
	}{
		{[]string{"ram-64g-ddr4-ecc-2666", "softraid-2x480ssd"}, true},
		{[]string{"ram-128g-ddr4-ecc-2666"}, false},
		{[]string{"ram-64g-ddr4-ecc-2133"}, false},
		{[]string{"softraid-2x480ssd-2x8000sa"}, false},
	} {
		if got := preorderFQNCoversOptions(fqn, tc.options); got != tc.want {
			t.Errorf("%v: got %v", tc.options, got)
		}
	}
}
