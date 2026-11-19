package ovh

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"
)

// 每个隔离 client 都是新建的,以前每个都要先 GET /auth/time 才能发第一个签名请求。
// 时差缓存之后,同一 endpoint 只有第一个 client 真去校时,后面的直接签名。
func TestIsolatedClientsShareAuthTimeDelta(t *testing.T) {
	ResetAuthTimeCache()
	defer ResetAuthTimeCache()

	var timeCalls, apiCalls atomic.Int32
	skew := int64(-90) // OVH 比本机慢 90 秒:签名时间戳必须按 OVH 的钟来
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1.0/auth/time":
			timeCalls.Add(1)
			fmt.Fprint(w, time.Now().Unix()+skew)
		case "/1.0/me":
			apiCalls.Add(1)
			var ts int64
			fmt.Sscan(r.Header.Get("X-Ovh-Timestamp"), &ts)
			if d := ts - (time.Now().Unix() + skew); d < -2 || d > 2 {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"message":"timestamp off by %d"}`, d)
				return
			}
			fmt.Fprint(w, `{"nichandle":"x"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	base, err := ovhsdk.NewClient(srv.URL+"/1.0", "k", "s", "c")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		c, err := IsolateClient(base)
		if err != nil {
			t.Fatal(err)
		}
		var me map[string]interface{}
		if err := c.Get("/me", &me); err != nil {
			t.Fatalf("第 %d 个隔离 client 签名请求失败: %v", i+1, err)
		}
	}
	if apiCalls.Load() != 3 {
		t.Fatalf("应有 3 次 /me 请求, 实际 %d", apiCalls.Load())
	}
	if timeCalls.Load() != 1 {
		t.Fatalf("3 个隔离 client 只应真正校时 1 次, 实际 %d 次", timeCalls.Load())
	}
}
