package netfp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// 判「通」的标准是拿到**任何** HTTP 响应，不是 200。
// 用 404/302 判失败会把「链路正常但这个路径不存在」误报成代理故障 ——
// 用户会据此去换一个其实没问题的代理。
func TestProbeTreatsAnyResponseAsReachable(t *testing.T) {
	for _, code := range []int{200, 302, 401, 404, 500} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		p := ProbeTarget(Options{Timeout: 5 * time.Second}, "t", srv.URL)
		srv.Close()
		if !p.OK {
			t.Errorf("HTTP %d 应当算连通，实际失败: %s", code, p.Error)
		}
		if p.Status != code {
			t.Errorf("状态码没带回来: 期望 %d，实际 %d", code, p.Status)
		}
	}
}

// 连不上要明确报失败，不能装成延迟很高。
func TestProbeUnreachable(t *testing.T) {
	p := ProbeTarget(Options{Timeout: 2 * time.Second}, "t", "http://127.0.0.1:1/")
	if p.OK {
		t.Fatal("连不上的目标不该判成通")
	}
	if p.Error == "" {
		t.Fatal("失败必须带原因，否则用户不知道是代理问题还是目标问题")
	}
}

// min / avg 都要有 —— 只给一个数看不出抖动，
// 而抖动大的代理会在补货那一刻不定时慢一拍。
func TestProbeReportsMinAndAvg(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	p := ProbeTarget(Options{Timeout: 5 * time.Second}, "t", srv.URL)
	if !p.OK {
		t.Fatalf("应当连通: %s", p.Error)
	}
	if p.AvgMS < p.MinMS {
		t.Fatalf("平均值不该小于最小值: min=%d avg=%d", p.MinMS, p.AvgMS)
	}
}

// 代理不通时每个目标都要失败，而不是悄悄直连过去。
func TestProbeThroughDeadProxyFailsAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	probes := ProbeAll(
		Options{ProxyURL: "socks5://127.0.0.1:1", Timeout: 2 * time.Second},
		[]ProbeSpec{{Name: "a", URL: srv.URL}, {Name: "b", URL: srv.URL}},
	)
	for _, p := range probes {
		if p.OK {
			t.Fatalf("%s: 代理不通时居然成功了 —— 说明绕过代理直连了", p.Name)
		}
	}
}

// 并发探测的结果顺序必须和传入顺序一致，否则界面上标签和数字会错位。
func TestProbeAllPreservesOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	specs := []ProbeSpec{{Name: "first", URL: srv.URL}, {Name: "second", URL: srv.URL}, {Name: "third", URL: srv.URL}}
	got := ProbeAll(Options{Timeout: 5 * time.Second}, specs)
	for i, p := range got {
		if p.Name != specs[i].Name {
			t.Fatalf("第 %d 个结果错位: 期望 %s，实际 %s", i, specs[i].Name, p.Name)
		}
	}
}
