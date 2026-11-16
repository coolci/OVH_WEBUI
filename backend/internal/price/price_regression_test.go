package price

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func newPriceTestState(t *testing.T) *app.State {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	lg := logger.New(filepath.Join(dir, "t.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
}

func resetPriceCache() {
	internalPriceCache.Lock()
	internalPriceCache.m = map[string]cacheEntry{}
	internalPriceCache.Unlock()
}

func TestRegression_BUG_OVH_04_UnassignedCartDiscardedWhenAssignFails(t *testing.T) {
	resetPriceCache()
	defer resetPriceCache()

	var assignFailed atomic.Int32
	var cartDeleted atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/order/cart" && r.Method == http.MethodPost:
			fmt.Fprint(w, `{"cartId":"unassigned-cart-001"}`)
		case r.URL.Path == "/order/cart/unassigned-cart-001" && r.Method == http.MethodDelete:
			cartDeleted.Add(1)
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			assignFailed.Add(1)
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"message":"Too many requests on /assign"}`)
		case strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodPost:
			fmt.Fprint(w, `{"itemId":101}`)
		case strings.HasSuffix(r.URL.Path, "/requiredConfiguration"):
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/configuration"):
			fmt.Fprint(w, `{"id":1}`)
		case strings.HasSuffix(r.URL.Path, "/summary"):
			fmt.Fprint(w, `{"prices":{"withoutTax":{"value":19.99,"currencyCode":"EUR"},"withTax":{"value":19.99,"currencyCode":"EUR"},"tax":{"value":0,"currencyCode":"EUR"}},"details":[]}`)
		default:
			fmt.Fprint(w, `null`)
		}
	}))
	defer srv.Close()
	origEP := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = origEP }()

	st := newPriceTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "acc-1", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}

	res, cartID := GetInternalCachedKeepCart(st, "acc-1", "24ska01", "gra", nil, "P1M")
	if assignFailed.Load() != 1 {
		t.Fatalf("expected /assign to be called once, got %d", assignFailed.Load())
	}
	if cartID != "" {
		t.Fatalf("expected unassigned cart to be discarded (cartID=\"\"), got %q (res=%+v)", cartID, res)
	}
	deadline := time.Now().Add(2 * time.Second)
	for (cartDeleted.Load() < 1 || ovh.CartInUse("unassigned-cart-001")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if cartDeleted.Load() < 1 || ovh.CartInUse("unassigned-cart-001") {
		t.Fatalf("expected unassigned cart to be deleted & released: deleted=%d inUse=%v",
			cartDeleted.Load(), ovh.CartInUse("unassigned-cart-001"))
	}
}

func TestRegression_BUG_OVH_10_TransientFailureNotNegativeCached(t *testing.T) {
	resetPriceCache()
	defer resetPriceCache()

	var upstreamHealthy atomic.Bool
	var cartCreateCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/time" {
			fmt.Fprint(w, time.Now().Unix())
			return
		}
		if r.URL.Path == "/order/cart" && r.Method == http.MethodPost {
			cartCreateCalls.Add(1)
			if !upstreamHealthy.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprint(w, `{"message":"Service temporarily unavailable"}`)
				return
			}
			fmt.Fprint(w, `{"cartId":"healthy-cart"}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/assign") {
			fmt.Fprint(w, `null`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/eco") && r.Method == http.MethodPost {
			fmt.Fprint(w, `{"itemId":1}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/requiredConfiguration") {
			fmt.Fprint(w, `[]`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/summary") {
			fmt.Fprint(w, `{"prices":{"withoutTax":{"value":29.99,"currencyCode":"EUR"}}}`)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()
	origEP := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	defer func() { ovhsdk.Endpoints["ovh-eu"] = origEP }()

	st := newPriceTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "acc-neg", Name: "EU", Endpoint: "ovh-eu", Zone: "IE", AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}

	// Call 1: transient 503
	res1, _ := GetInternalCachedKeepCart(st, "acc-neg", "24ska01", "rbx", nil, "P1M")
	if res1.Success {
		t.Fatal("expected first call to fail with 503")
	}

	// Upstream recovers immediately within the 20s restock window -> Call 2 must succeed!
	upstreamHealthy.Store(true)
	res2, cart2 := GetInternalCachedKeepCart(st, "acc-neg", "24ska01", "rbx", nil, "P1M")
	defer ovh.ReleaseCart(cart2)
	if !res2.Success || cart2 != "healthy-cart" || cartCreateCalls.Load() != 2 {
		t.Fatalf("expected 2nd call to succeed without negative cache poison: res2=%+v cart2=%q cartCreateCalls=%d",
			res2, cart2, cartCreateCalls.Load())
	}
}
