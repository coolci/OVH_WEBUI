package ovh

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/types"
)

func TestRegression_BUG_OVH_03_SDKPreserves60sTimeoutAcrossRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/time" {
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"nichandle":"ab12345-ovh"}`)
	}))
	defer srv.Close()

	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = orig }()

	acc := types.OVHAccount{
		ID: "acc-1", Name: "Test", Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "ak", AppSecret: "as", ConsumerKey: "ck", IsDefault: true,
	}
	f := NewFactory(nil, func(id string) (types.OVHAccount, bool) {
		return acc, true
	})

	baseCli, err := f.ClientFor("acc-1")
	if err != nil {
		t.Fatal(err)
	}
	if baseCli.Timeout != 60*time.Second || baseCli.Client.Timeout != 60*time.Second {
		t.Fatalf("expected initial baseCli Timeout=60s, got sdk=%v http=%v", baseCli.Timeout, baseCli.Client.Timeout)
	}

	var me map[string]interface{}
	if err := baseCli.Get("/me", &me); err != nil {
		t.Fatal(err)
	}
	if baseCli.Client.Timeout != 60*time.Second {
		t.Fatalf("expected baseCli.Client.Timeout to stay 60s after request, got %v", baseCli.Client.Timeout)
	}

	isoCli, err := f.IsolatedClientFor("acc-1")
	if err != nil {
		t.Fatal(err)
	}
	if isoCli.Timeout != 60*time.Second || isoCli.Client.Timeout != 60*time.Second {
		t.Fatalf("expected initial isoCli Timeout=60s, got sdk=%v http=%v", isoCli.Timeout, isoCli.Client.Timeout)
	}
	if err := isoCli.Get("/me", &me); err != nil {
		t.Fatal(err)
	}
	if isoCli.Client.Timeout != 60*time.Second {
		t.Fatalf("expected isoCli.Client.Timeout to stay 60s after request, got %v", isoCli.Client.Timeout)
	}
}

func TestRegression_BUG_OVH_11_CartRegistryTTLExpiration(t *testing.T) {
	const cid = "stale-ttl-cart"
	defer ReleaseCart(cid)

	TrackCart(cid)
	if !CartInUse(cid) {
		t.Fatal("expected freshly tracked cart to be in use")
	}

	inUseCarts.Store(cid, time.Now().Add(-(cartTrackTTL + time.Minute)))
	if CartInUse(cid) {
		t.Fatal("expected expired cart tracking entry beyond TTL to be evicted")
	}
}
