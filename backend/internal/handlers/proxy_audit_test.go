package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/auth"
)

func TestAuditPairURLUsesPublicHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PUBLIC_BASE_URL", "https://console.example.test")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "http://console.example.test/api/app/pairing-codes", nil)
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	got := pairURL(c, "AUDIT123")
	want := "https://console.example.test/api/app/pair#AUDIT123"
	if got != want {
		t.Fatalf("TLS terminated by proxy: got %q, want %q", got, want)
	}
}

// auditAuthRouter 模拟 nginx(172.28.0.2)转发;trusted 为 gin 信任的代理列表。
func auditAuthRouter(trusted []string) func(client, key string) int {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(trusted)
	r.Use(auth.Middleware(auth.Config{APIKey: "audit-key", Enabled: true}))
	r.GET("/api/audit", func(c *gin.Context) { c.Status(http.StatusOK) })
	return func(client, key string) int {
		req := httptest.NewRequest("GET", "http://console.example.test/api/audit", nil)
		req.RemoteAddr = "172.28.0.2:54321"
		req.Header.Set("X-Forwarded-For", client)
		req.Header.Set("X-Real-IP", client)
		req.Header.Set("X-API-Key", key)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
}

// 代理已在 TRUSTED_PROXIES 中声明时,失败锁定按真实客户端隔离。
func TestAuditProxyAuthFailuresAreClientIsolated(t *testing.T) {
	auth.ResetAuthFailures()
	defer auth.ResetAuthFailures()
	call := auditAuthRouter([]string{"172.28.0.0/16"})
	for i := 0; i < auth.MaxAuthFailures; i++ {
		call("198.51.100.10", "wrong")
	}
	if got := call("198.51.100.20", "audit-key"); got != http.StatusOK {
		t.Fatalf("different client with correct key received %d after other client failed", got)
	}
	if got := call("198.51.100.10", "audit-key"); got != http.StatusTooManyRequests {
		t.Fatalf("locked client was not rate limited: %d", got)
	}
}

// 未声明的来源伪造 X-Forwarded-For / X-Real-IP 不能换个"新 IP"绕过锁定继续爆破密钥。
func TestAuditSpoofedForwardHeadersCannotBypassLockout(t *testing.T) {
	auth.ResetAuthFailures()
	defer auth.ResetAuthFailures()
	call := auditAuthRouter(nil)
	for i := 0; i < auth.MaxAuthFailures; i++ {
		call(fmt.Sprintf("203.0.113.%d", i+1), "wrong")
	}
	if got := call("203.0.113.250", "wrong"); got != http.StatusTooManyRequests {
		t.Fatalf("spoofed forwarding header bypassed the lockout: %d", got)
	}
}
