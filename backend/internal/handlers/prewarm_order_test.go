package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

type mockAuditTransport func(*http.Request) (*http.Response, error)

func (f mockAuditTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestPrewarmOrderAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	st := app.NewState(storage.Paths{LogsDir: filepath.Join(dir, "logs"), DataDir: dir}, config.New(database), logger.New(filepath.Join(dir, "logs.json"), nil), database)
	purchase.ClearPrewarmPool(st)
	defer purchase.ClearPrewarmPool(st)

	accountID := "acc-prewarm-api"
	st.Accounts = []types.OVHAccount{
		{
			ID:          accountID,
			Name:        accountID,
			Endpoint:    "ovh-eu",
			Zone:        "PT",
			AppKey:      "ak",
			AppSecret:   "as",
			ConsumerKey: "ck",
			IsDefault:   true,
		},
	}

	var cartCreated atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			cartCreated.Add(1)
			fmt.Fprint(w, `{"cartId":"api-pw-cart-1"}`)
		case strings.HasSuffix(r.URL.Path, "/assign"):
			fmt.Fprint(w, `null`)
		case strings.HasSuffix(r.URL.Path, "/eco"):
			fmt.Fprint(w, `{"itemId":777}`)
		case strings.Contains(r.URL.Path, "/configuration"):
			fmt.Fprint(w, `{}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/eco/options"):
			fmt.Fprint(w, `[]`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()

	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = srv.URL
	t.Cleanup(func() { ovhsdk.Endpoints["ovh-eu"] = orig })

	oldTransport := http.DefaultTransport
	http.DefaultTransport = mockAuditTransport(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/order/catalog/public/eco") {
			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"plans":[{"planCode":"24sk-api-plan","configurations":[{"name":"region","values":["europe"]}]}]}`)),
				Request:    r,
			}, nil
		}
		return nil, fmt.Errorf("blocked: %s", r.URL)
	})
	defer func() { http.DefaultTransport = oldTransport }()
	// Catalog fetches use the shared fingerprint client, not http.DefaultTransport.
	defer netfp.SwapSharedTransportForTesting(http.DefaultTransport)()

	r := gin.New()
	r.POST("/api/purchase/prewarm", PrewarmOrder(st))

	// 1. 第一次请求：后台启动装车
	body, _ := json.Marshal(map[string]interface{}{
		"account_id": accountID,
		"planCode":   "24sk-api-plan",
		"datacenter": "gra",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/purchase/prewarm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("预期 200 OK，实际: %d %s", w.Code, w.Body.String())
	}

	// 等待装车完成
	deadline := time.Now().Add(3 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		if purchase.HasPrewarmedCart(accountID, "24sk-api-plan", "gra", nil, "") {
			found = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !found {
		t.Fatalf("预热装车未进入池 (cartCreated=%d)", cartCreated.Load())
	}

	// 2. 第二次请求：池中已有车，应当直接返回 ready: true
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/purchase/prewarm", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	var resp2 map[string]interface{}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2["ready"] != true {
		t.Fatalf("池中有就绪车时应当返回 ready: true，实际: %+v", resp2)
	}
}
