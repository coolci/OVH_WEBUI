package purchase

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// 只删"带本程序标记、本进程没在用、不是只读"的车;用户自己建的车、老车、在用的车一律不动。
func TestCartJanitorDeletesOnlyOurLeftovers(t *testing.T) {
	carts := map[string]string{
		"ours-leftover": `{"cartId":"ours-leftover","description":"ovh-webui","readOnly":false}`,
		"users-own":     `{"cartId":"users-own","description":"my manual cart","readOnly":false}`,
		"ours-in-use":   `{"cartId":"ours-in-use","description":"ovh-webui","readOnly":false}`,
		"ours-readonly": `{"cartId":"ours-readonly","description":"ovh-webui","readOnly":true}`,
		"old-unmarked":  `{"cartId":"old-unmarked","readOnly":false}`,
	}
	var mu sync.Mutex
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.URL.Path == "/order/cart" && r.Method == http.MethodGet:
			fmt.Fprint(w, `["ours-leftover","users-own","ours-in-use","ours-readonly","old-unmarked"]`)
		case strings.HasPrefix(r.URL.Path, "/order/cart/") && r.Method == http.MethodGet:
			fmt.Fprint(w, carts[strings.TrimPrefix(r.URL.Path, "/order/cart/")])
		case strings.HasPrefix(r.URL.Path, "/order/cart/") && r.Method == http.MethodDelete:
			mu.Lock()
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/order/cart/"))
			mu.Unlock()
			fmt.Fprint(w, `null`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"unexpected"}`)
		}
	}))
	t.Cleanup(srv.Close)
	swapEndpoint(t, "ovh-eu", srv.URL)
	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{ID: "acc", Name: "A", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}
	ovh.TrackCart("ours-in-use")

	res := CleanupLeftoverCarts(context.Background(), s)
	mu.Lock()
	defer mu.Unlock()
	if len(deleted) != 1 || deleted[0] != "ours-leftover" {
		t.Fatalf("only our untracked leftover may be deleted, deleted=%v", deleted)
	}
	if len(res) != 1 || res[0].Listed != 5 || res[0].Deleted != 1 || res[0].Kept != 4 || res[0].Error != "" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestCartCreateBodyCarriesMarker(t *testing.T) {
	body := ovh.CartCreateBody("IE")
	if body["description"] != ovh.CartMarker || body["ovhSubsidiary"] != "IE" {
		t.Fatalf("carts we create must carry the marker: %v", body)
	}
}
