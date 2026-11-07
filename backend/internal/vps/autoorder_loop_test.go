package vps

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/types"
)

// 驱动真实的 VPS 监控循环:首次检查发现的存量不下单;无货 → 有货的跳变才下单,
// 且只下一次(之后自动下单被关闭)。库存接口和下单接口都由本地 fixture 应答。
func TestMonitorLoopOrdersOnlyOnRestock(t *testing.T) {
	state, f := newVPSOrderState(t)
	state.VPSCheckInterval = 1
	sub := addAutoSub(t, state)

	var stock atomic.Value
	stock.Store("available") // 第一轮:已有存量(不算补货)
	var checks atomic.Int32
	orderHost := ""
	if u, err := url.Parse(ovhsdk.Endpoints["ovh-eu"]); err == nil {
		orderHost = u.Host
	}
	base := http.DefaultTransport
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == orderHost || r.URL.Hostname() == "127.0.0.1" {
			return base.RoundTrip(r)
		}
		if strings.Contains(r.URL.Path, "/vps/order/rule/datacenter") {
			checks.Add(1)
			body := fmt.Sprintf(`{"datacenters":[{"datacenter":"GRA","code":"eu-west-gra","status":%q,"linuxStatus":%q}]}`,
				stock.Load(), stock.Load())
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
	})
	restore := netfp.SwapSharedTransportForTesting(transport)
	t.Cleanup(restore)
	origClient := availabilityHTTPClient
	availabilityHTTPClient = func(_ *app.State, _ string, timeout time.Duration) (*http.Client, error) {
		return &http.Client{Transport: transport, Timeout: timeout}, nil
	}
	t.Cleanup(func() { availabilityHTTPClient = origClient })
	// 通知走本地 webhook,只为让循环认为"有可用通知通道"
	notify.AllowLocalWebhookForTesting = true
	t.Cleanup(func() { notify.AllowLocalWebhookForTesting = false })
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, `{"code":0}`) }))
	t.Cleanup(hook.Close)
	state.Config.Set(types.Config{NotifyWebhookURL: hook.URL})

	if !Start(state) {
		t.Fatal("monitor did not start")
	}
	t.Cleanup(func() { Stop(state); waitLoopExit() })

	waitFor := func(cond func() bool, what string) {
		deadline := time.Now().Add(10 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				for _, e := range state.Logger.Snapshot() {
					t.Log(e.Message)
				}
				t.Fatalf("timed out waiting for %s (checks=%d checkouts=%d)", what, checks.Load(), f.checkouts.Load())
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	// 首轮存量:不下单
	waitFor(func() bool { return checks.Load() >= 2 }, "two checks with initial stock")
	if f.checkouts.Load() != 0 {
		t.Fatalf("stock found on the first check must not be ordered, checkouts=%d", f.checkouts.Load())
	}
	// 无货 → 有货:下单一次
	stock.Store("out-of-stock")
	n := checks.Load()
	waitFor(func() bool { return checks.Load() >= n+1 }, "an out-of-stock check")
	stock.Store("available")
	waitFor(func() bool { return f.checkouts.Load() >= 1 }, "the restock order")
	// 再抖动一次:自动下单已关闭,不会再买
	stock.Store("out-of-stock")
	n = checks.Load()
	waitFor(func() bool { return checks.Load() >= n+1 }, "a second out-of-stock check")
	stock.Store("available")
	n = checks.Load()
	waitFor(func() bool { return checks.Load() >= n+2 }, "two more checks")
	if f.checkouts.Load() != 1 {
		t.Fatalf("one subscription must order only once, checkouts=%d", f.checkouts.Load())
	}
	if persistedSub(t, state, sub.ID).AutoOrder {
		t.Fatal("auto-order must be off after the order")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func waitLoopExit() {
	runningMu.Lock()
	wg := loopWg
	runningMu.Unlock()
	if wg != nil {
		wg.Wait()
	}
}
