package notify

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// webhook 的 SSRF 校验与真正拨号必须用同一组 IP:校验完再把主机名交给拨号器解析,
// 两次解析之间 DNS 可以换成内网地址(DNS rebinding)。这里断言拨号器拿到的是 IP 字面量。
func TestWebhookDialsTheVettedIPNotTheHostname(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"code":0}`) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_, port, _ := net.SplitHostPort(u.Host)

	var mu sync.Mutex
	var dialed []string
	base := &net.Dialer{}
	orig := http.DefaultTransport
	http.DefaultTransport = &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		mu.Lock()
		dialed = append(dialed, addr)
		mu.Unlock()
		return base.DialContext(ctx, network, addr)
	}}
	defer func() { http.DefaultTransport = orig }()

	if err := sendWebhook("http://localhost:"+port+"/hook", "hello"); err != nil {
		t.Fatalf("delivery through the pinned IP failed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(dialed) == 0 {
		t.Fatal("dialer was never called")
	}
	for _, addr := range dialed {
		host, _, _ := net.SplitHostPort(addr)
		if net.ParseIP(host) == nil || strings.EqualFold(host, "localhost") {
			t.Fatalf("dialer re-resolved the hostname instead of using the vetted IP: %q", addr)
		}
	}
}
