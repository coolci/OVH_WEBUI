package purchase

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"
)

func periodPrices(entries ...[3]string) []interface{} {
	out := []interface{}{}
	for _, e := range entries {
		out = append(out, map[string]interface{}{"duration": e[0], "pricingMode": e[1], "pricingType": e[2], "capacities": []interface{}{"renew"}})
	}
	return out
}

// 移植上游 v0.4.16:同周期内挑计价,绝不退成别的周期(月付主机不能挂年付配件)。
func TestPickPricingNeverSubstitutesPeriod(t *testing.T) {
	for _, tc := range []struct {
		name         string
		prices       []interface{}
		want         string
		wantDuration string
		wantMode     string
		wantOK       bool
	}{
		{"same period preferred", periodPrices([3]string{"P12M", "upfront12", "rental"}, [3]string{"P1M", "default", "rental"}), "P1M", "P1M", "default", true},
		{"only yearly offered", periodPrices([3]string{"P12M", "upfront12", "rental"}), "P1M", "", "", false},
		{"yearly alias accepted", periodPrices([3]string{"P1Y", "upfront12", "rental"}), "P12M", "P1Y", "upfront12", true},
		{"nothing offered", nil, "P1M", "P1M", "default", true},
	} {
		d, m, ok := pickPricing(tc.prices, tc.want, "")
		if d != tc.wantDuration || m != tc.wantMode || ok != tc.wantOK {
			t.Errorf("%s: got %s/%s want %s/%s", tc.name, d, m, tc.wantDuration, tc.wantMode)
		}
	}
}

// 月付加购被拒后的"按目录计价重试"只能换计价模式,不能把整单换成年付。
func TestLookupEcoPricingRejectsOtherPeriods(t *testing.T) {
	st := queueCompletionState(t)
	for _, tc := range []struct {
		name   string
		offers string
		found  bool
		mode   string
	}{
		{"yearly only", `[{"planCode":"24sk202","prices":[{"duration":"P12M","pricingMode":"upfront12"}]}]`, false, ""},
		{"monthly other mode", `[{"planCode":"24sk202","prices":[{"duration":"P12M","pricingMode":"upfront12"},{"duration":"P1M","pricingMode":"degressivity12"}]}]`, true, "degressivity12"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/auth/time":
					fmt.Fprint(w, time.Now().Unix())
				case "/order/cart/c/eco":
					fmt.Fprint(w, tc.offers)
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			client, err := ovhsdk.NewClient(srv.URL, "ak", "as", "ck")
			if err != nil {
				t.Fatal(err)
			}
			d, m, found := lookupEcoPricing(st, client, "c", "24sk202", "")
			if found != tc.found || (found && (d != "P1M" || m != tc.mode)) {
				t.Fatalf("got %s/%s found=%v", d, m, found)
			}
		})
	}
}
