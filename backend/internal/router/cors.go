package router

import (
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// IsOriginAllowed CORS 来源校验:
// 放行本机(localhost / 127.0.0.1 / ::1)、内网私有 IP (192.168.x.x, 10.x.x.x, 172.16-31.x.x，支持任意前端端口如 8080/19997 等)，
// 以及用户显式在 CORS_ALLOWED_ORIGINS 里声明的来源。
func IsOriginAllowed(origin string) bool {
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	hostname := strings.ToLower(u.Hostname())
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return true
	}
	if ip := net.ParseIP(hostname); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() {
			return true
		}
	}
	for _, o := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" && strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

// AllowedOrigins 兼容保留函数
func AllowedOrigins(port string) []string {
	if port == "" {
		port = "19998"
	}
	out := []string{}
	for _, host := range []string{"localhost", "127.0.0.1", "[::1]"} {
		for _, p := range []string{port, "8080", "5173", "19997", "3000"} {
			out = append(out, "http://"+host+":"+p)
			out = append(out, "https://"+host+":"+p)
		}
	}
	for _, o := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// NewCORSMiddleware 构建跨域安全中间件
func NewCORSMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOriginFunc:  IsOriginAllowed,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-API-Key", "X-Request-Time"},
		ExposeHeaders:    []string{"X-Cache-Warning", "X-Partial-Failures", "X-Cache-Age-Seconds"},
		AllowCredentials: false,
	})
}
