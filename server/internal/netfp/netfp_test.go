package netfp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 配了代理却连不上时，请求必须**失败**，绝不能悄悄走直连。
// 悄悄直连的表现是一切正常、隔离却已经没了，而用户无从察觉 ——
// 这是整个功能里最要命的一条。
func TestProxyNeverFallsBackToDirect(t *testing.T) {
	// 起一个真实服务端；如果发生了直连，请求会成功，测试就会发现
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	for _, proxyURL := range []string{
		"http://127.0.0.1:1", // 几乎不可能有人监听的端口
		"socks5://127.0.0.1:1",
	} {
		hit = false
		cli, err := Client(Options{ProxyURL: proxyURL, Profile: Profiles["default"], Timeout: 3 * time.Second})
		if err != nil {
			t.Fatalf("%s: 构造客户端失败: %v", proxyURL, err)
		}
		resp, err := cli.Get(srv.URL)
		if err == nil {
			resp.Body.Close()
			t.Fatalf("%s: 代理不通时请求居然成功了 —— 说明退回了直连，隔离失效", proxyURL)
		}
		if hit {
			t.Fatalf("%s: 请求绕过代理直接打到了目标服务端", proxyURL)
		}
		if !IsProxyError(err) {
			t.Errorf("%s: 应当被识别成代理故障，实际: %v", proxyURL, err)
		}
	}
}

// 代理故障要能回调出去，否则看门狗收不到信号。
func TestProxyErrorCallback(t *testing.T) {
	var got error
	cli, err := Client(Options{
		ProxyURL:     "socks5://127.0.0.1:1",
		Profile:      Profiles["default"],
		Timeout:      3 * time.Second,
		OnProxyError: func(e error) { got = e },
	})
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if resp, err := cli.Get("https://example.invalid/"); err == nil {
		resp.Body.Close()
		t.Fatal("应当失败")
	}
	if got == nil {
		t.Fatal("代理故障没有回调出来 —— 看门狗永远等不到信号")
	}
	if !IsProxyError(got) {
		t.Fatalf("回调出来的不是 ProxyError: %v", got)
	}
}

// 没配代理时不该把普通网络错误当成代理故障 ——
// 误判会把正常任务停掉。
func TestDirectConnectionNeverReportsProxyError(t *testing.T) {
	cli, err := Client(Options{Profile: Profiles["default"], Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	_, err = cli.Get("https://this-host-does-not-exist.invalid/")
	if err == nil {
		t.Skip("这个环境里该域名居然能解析，跳过")
	}
	if IsProxyError(err) {
		t.Fatalf("直连时的网络错误被当成了代理故障: %v", err)
	}
}

// 判据必须保守：目标站点的问题不能算到代理头上。
func TestLooksLikeProxyFailureIsConservative(t *testing.T) {
	yes := []string{
		"proxyconnect tcp: dial tcp 1.2.3.4:8080: connect: connection refused",
		"socks connect tcp 1.2.3.4:1080->example.com:443: unknown error",
		"Get \"https://x\": proxy authentication required",
	}
	no := []string{
		"Error 429: Too many requests",
		"x509: certificate signed by unknown authority",
		"http: server gave HTTP response to HTTPS client",
		"Error 400: Invalid signature",
		"context deadline exceeded (Client.Timeout exceeded while awaiting headers)",
	}
	for _, s := range yes {
		if !looksLikeProxyFailure(errors.New(s)) {
			t.Errorf("应判为代理故障: %q", s)
		}
	}
	for _, s := range no {
		if looksLikeProxyFailure(errors.New(s)) {
			t.Errorf("不该判为代理故障（会误停任务）: %q", s)
		}
	}
}

// 代理地址里的密码不能进日志。
func TestScrubProxyURL(t *testing.T) {
	got := ScrubProxyURL("socks5://alice:SuperSecret@1.2.3.4:1080")
	if strings.Contains(got, "SuperSecret") {
		t.Fatalf("密码没抹掉: %s", got)
	}
	if !strings.Contains(got, "alice") || !strings.Contains(got, "1.2.3.4:1080") {
		t.Fatalf("抹得太狠，定位不了是哪个代理: %s", got)
	}
	if ScrubProxyURL("") != "" {
		t.Fatal("空串应当原样返回")
	}
}

// 形状不对的代理地址要在保存那一刻就挡下来，
// 而不是等到补货那一刻才发现出口不对。
func TestValidateProxyURL(t *testing.T) {
	ok := []string{
		"", // 空 = 直连
		"http://1.2.3.4:8080",
		"https://proxy.example.com:3128",
		"socks5://u:p@1.2.3.4:1080",
		"socks5h://1.2.3.4:1080",
	}
	bad := []string{
		"1.2.3.4:1080",          // 没有协议
		"ftp://1.2.3.4:21",      // 不支持的协议
		"socks5://1.2.3.4",      // 没端口
		"http://",               // 没主机
		"socks4://1.2.3.4:1080", // 不支持
	}
	for _, s := range ok {
		if err := ValidateProxyURL(s); err != nil {
			t.Errorf("%q 应当合法: %v", s, err)
		}
	}
	for _, s := range bad {
		if err := ValidateProxyURL(s); err == nil {
			t.Errorf("%q 应当被拒绝", s)
		}
	}
}

// 指纹配置要真的落到 transport 上，不能只是个名字。
func TestProfileAppliesToTransport(t *testing.T) {
	tr, err := Transport(Options{Profile: Profiles["legacy-http1"]})
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	ht, ok := tr.(*headerTransport)
	if !ok {
		t.Fatalf("有 UA 的配置应当被包了一层 headerTransport，实际 %T", tr)
	}
	base, ok := ht.base.(*http.Transport)
	if !ok {
		t.Fatalf("底层应当是 *http.Transport，实际 %T", ht.base)
	}
	if base.ForceAttemptHTTP2 {
		t.Error("legacy-http1 不该允许 h2")
	}
	if got := base.TLSClientConfig.NextProtos; len(got) != 1 || got[0] != "http/1.1" {
		t.Errorf("关掉 h2 时必须把 ALPN 写死成 http/1.1，实际 %v", got)
	}
	if base.TLSClientConfig.MaxVersion != 0x0303 { // TLS 1.2
		t.Errorf("legacy-http1 应当封顶 TLS 1.2，实际 %#x", base.TLSClientConfig.MaxVersion)
	}
}

// 不认识的指纹名要退回 default 并说明，不能静默当成 default。
func TestLookupProfileUnknown(t *testing.T) {
	p, warn := LookupProfile("nope")
	if p.Name != "default" {
		t.Fatalf("应退回 default，实际 %s", p.Name)
	}
	if warn == "" {
		t.Fatal("退回时必须给出说明，否则用户以为自己配的生效了")
	}
	if _, warn := LookupProfile(""); warn != "" {
		t.Fatalf("空名字是正常情况，不该告警: %s", warn)
	}
}
