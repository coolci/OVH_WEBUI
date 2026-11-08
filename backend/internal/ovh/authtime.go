package ovh

import (
	"bytes"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 隔离 client(IsolateClient)每次都是新建的 go-ovh Client,它在第一个签名请求前
// 会先 GET /auth/time 校时 —— 抢购链路上每一轮都平白多一次串行往返(几十到几百毫秒)。
//
// 这里按 endpoint 缓存"本机 − OVH"的时差:缓存有效期内,隔离 client 的 /auth/time
// 由本地按缓存时差直接应答;过期或没有缓存时放行一次真实请求并刷新缓存。
// 有效期不设太长:宿主机时钟被 NTP 校正或手工改过时,最多 authTimeTTL 后就会重新对齐。

const authTimeTTL = 10 * time.Minute

type authTimeEntry struct {
	delta time.Duration // 本机时间 − OVH 时间,与 go-ovh 的 timeDelta 同义
	at    time.Time
}

var authTimeCache sync.Map // endpoint 基址(host + /1.0 等前缀) → authTimeEntry

// ResetAuthTimeCache 清空时差缓存(供测试使用)
func ResetAuthTimeCache() {
	authTimeCache.Range(func(k, _ any) bool {
		authTimeCache.Delete(k)
		return true
	})
}

type authTimeTransport struct {
	next http.RoundTripper
}

var authTimeWrappers sync.Map // 底层 transport(指针) → *authTimeTransport

// wrapAuthTime 给底层 transport 套上时差缓存。同一个底层 transport 复用同一个包装,
// 隔离 client 之间仍然是"同一个 transport、同一个连接池"。
func wrapAuthTime(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	if reflect.ValueOf(next).Kind() != reflect.Pointer {
		// 非指针(如测试里的函数类型)不能做 map 键;每次新包一层又会让隔离 client
		// 之间不再共享同一个 transport。这种情况原样返回,不做时差缓存。
		// 真实账户的出站 transport(netfp / http.Transport)都是指针。
		return next
	}
	if w, ok := authTimeWrappers.Load(next); ok {
		return w.(*authTimeTransport)
	}
	w, _ := authTimeWrappers.LoadOrStore(next, &authTimeTransport{next: next})
	return w.(*authTimeTransport)
}

func (t *authTimeTransport) base() http.RoundTripper {
	if t.next != nil {
		return t.next
	}
	return http.DefaultTransport
}

func (t *authTimeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet || !strings.HasSuffix(req.URL.Path, "/auth/time") {
		return t.base().RoundTrip(req)
	}
	key := req.URL.Host + strings.TrimSuffix(req.URL.Path, "/auth/time")
	if v, ok := authTimeCache.Load(key); ok {
		e := v.(authTimeEntry)
		if time.Since(e.at) < authTimeTTL {
			body := strconv.FormatInt(time.Now().Add(-e.delta).Unix(), 10)
			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{"Content-Type": []string{"application/json"}},
				Body:          io.NopCloser(strings.NewReader(body)),
				ContentLength: int64(len(body)),
				Request:       req,
			}, nil
		}
	}
	resp, err := t.base().RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if ts, perr := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); perr == nil && ts > 0 {
		authTimeCache.Store(key, authTimeEntry{delta: time.Since(time.Unix(ts, 0)), at: time.Now()})
	}
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	resp.ContentLength = int64(len(raw))
	return resp, nil
}
