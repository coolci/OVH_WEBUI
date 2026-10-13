package handlers

import (
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

func TestAuditProxyAuthFailuresAreClientIsolated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth.ResetAuthFailures()
	defer auth.ResetAuthFailures()
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(auth.Middleware(auth.Config{APIKey: "audit-key", Enabled: true}))
	r.GET("/api/audit", func(c *gin.Context) { c.Status(http.StatusOK) })
	call := func(client, key string) int {
		req := httptest.NewRequest("GET", "http://console.example.test/api/audit", nil)
		req.RemoteAddr = "172.28.0.2:54321"
		req.Header.Set("X-Forwarded-For", client)
		req.Header.Set("X-Real-IP", client)
		req.Header.Set("X-API-Key", key)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	for i := 0; i < auth.MaxAuthFailures; i++ {
		call("198.51.100.10", "wrong")
	}
	if got := call("198.51.100.20", "audit-key"); got != http.StatusOK {
		t.Fatalf("different client with correct key received %d after other client failed", got)
	}
}
