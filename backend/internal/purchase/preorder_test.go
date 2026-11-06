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

func TestLegacyPreorderUsesNormalPurchaseWithoutCatalogWait(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, tc := range []struct {
		sub            string
		listed, outage bool
		options        []string
	}{
		{"IE", false, false, nil},
		{"DE", false, true, nil},
		{"FR", true, false, nil},
		{"ES", true, false, []string{"ram-32g"}},
		{"NL", true, false, []string{"ram-64g"}},
		{"GB", true, false, []string{"ram-32g", "softraid-2x480ssd"}},
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
			carts, checkouts, catalogRequests := 0, 0, 0
			transport := preorderTransport(func(r *http.Request) (*http.Response, error) {
				body, status := "{}", http.StatusOK
				switch {
				case strings.Contains(r.URL.Path, "/order/catalog/public/eco"):
					catalogRequests++
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
			var normalOutcome Outcome
			normalCarts := 0
			for _, legacyFlag := range []bool{false, true} {
				carts = 0
				item := &types.QueueItem{ID: "pre", AccountID: "eu", PlanCode: "27skle03-v1", Datacenter: "gra", Preorder: legacyFlag, MaxRetries: 1, Options: tc.options}
				outcome := PurchaseServer(context.Background(), st, item)
				if outcome.WaitingForCatalog || catalogRequests != 0 {
					t.Fatalf("unexpected catalog gate: %+v requests=%d", outcome, catalogRequests)
				}
				if len(tc.options) == 0 && carts != 1 {
					t.Fatalf("unlisted/outage/listed task must try cart: carts=%d outcome=%+v", carts, outcome)
				}
				if legacyFlag {
					if carts != normalCarts || outcome != normalOutcome {
						t.Fatalf("legacy flag changed ordinary behavior: carts %d/%d, outcome %+v/%+v", carts, normalCarts, outcome, normalOutcome)
					}
				} else {
					normalCarts = carts
					normalOutcome = outcome
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
