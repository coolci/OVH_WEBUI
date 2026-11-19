package auth

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// AuthSourceContextKey Gin Context 中记录认证凭据来源的键名
	AuthSourceContextKey = "auth_source"
	// AuthSourceMasterKey 通过主密钥 X-API-Key 完成认证
	AuthSourceMasterKey = "master_key"
	// AuthSourceDeviceToken 通过移动端配对令牌 Authorization: Bearer 完成认证
	AuthSourceDeviceToken = "device_token"
)

// Config 认证配置（从环境变量初始化）
type Config struct {
	APIKey  string
	Enabled bool
	// WhitelistPaths 跳过验证的路径
	WhitelistPaths map[string]struct{}
	// DeviceTokenValid App 设备令牌校验(配对体系,见 internal/db/appdevices.go)。
	// 依赖注入而不是直接 import db:这个包要保持无状态可测。
	DeviceTokenValid func(token string) (int64, bool)
}

// DefaultWhitelist 不需要 X-API-Key 即可访问的路径。
// 注意 /api/version/check-update 已移出白名单，防止未认证流量耗尽宿主机 GitHub API 匿名配额。
func DefaultWhitelist() map[string]struct{} {
	return map[string]struct{}{
		"/health":       {},
		"/api/health":   {},
		"/api/version":  {}, // 前端启动时拉版本号,登录前可见
		"/api/app/pair": {}, // App 配对兑换:凭 2 分钟一次性码,不需要密钥
	}
}

func abortRateLimited(c *gin.Context, wait time.Duration) {
	secs := int(wait.Seconds()) + 1
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", strconv.Itoa(secs))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error": "Too many failed attempts",
		"message": "API 密钥连续错误次数过多,已暂时拒绝该来源的请求。" +
			"请等待约 " + strconv.Itoa(secs) + " 秒后再试;" +
			"请核对 API_SECRET_KEY 配置;自动生成的密钥在数据目录的 .api-secret 文件中",
		"code":       "AUTH_RATE_LIMITED",
		"retryAfter": secs,
	})
}

func checkRequestTimestamp(c *gin.Context) bool {
	if ts := c.GetHeader("X-Request-Time"); ts != "" {
		if reqMs, err := strconv.ParseInt(ts, 10, 64); err == nil {
			diff := time.Now().UnixMilli() - reqMs
			if diff < 0 {
				diff = -diff
			}
			if diff > 5*60*1000 {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "Request expired",
					"message": "请求已过期（时间戳验证失败）",
					"code":    "TIMESTAMP_EXPIRED",
				})
				return false
			}
		}
	}
	return true
}

// Middleware Gin 中间件：验证 X-API-Key 或移动端设备令牌。
//
// 安全顺序铁律（BUG-SEC-03）：
// 先以常数时间校验 X-API-Key 或核验有效 Bearer 设备令牌，合法请求立即放行（绝不被共享反代/NAT 来源 IP 的失败计数误封）；
// 仅当凭据缺失或校验失败时，才检查并累加来源 IP 的限流封禁桶。
func Middleware(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		// CORS 预检放行
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		path := c.Request.URL.Path

		// 只验证 /api 路径
		if !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}

		// 白名单放行
		if _, ok := cfg.WhitelistPaths[path]; ok {
			c.Next()
			return
		}

		ip := resolveClientIP(c)
		now := time.Now()
		key := c.GetHeader("X-API-Key")

		// 1. 携带 X-API-Key：先以常量时间核验主密钥
		if key != "" {
			if cfg.APIKey != "" && subtle.ConstantTimeCompare([]byte(key), []byte(cfg.APIKey)) == 1 {
				if !checkRequestTimestamp(c) {
					return
				}
				authRecordValid(ip, now)
				c.Set(AuthSourceContextKey, AuthSourceMasterKey)
				c.Next()
				return
			}
			if blocked, wait := authReserve(ip, now); blocked {
				abortRateLimited(c, wait)
				return
			}
			authRelease(ip, authFailed, now)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid API key",
				"message": "API 密钥无效。请核对 API_SECRET_KEY 配置或数据目录的 .api-secret 文件;连续错误多次后会被暂时锁定",
				"code":    "INVALID_API_KEY",
			})
			return
		}

		// 2. 未携带 X-API-Key，检查 Authorization: Bearer 设备令牌
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if token != "" && cfg.DeviceTokenValid != nil {
				if id, ok := cfg.DeviceTokenValid(token); ok {
					authRecordValid(ip, now)
					c.Set("app_device_id", id)
					c.Set(AuthSourceContextKey, AuthSourceDeviceToken)
					c.Next()
					return
				}
			}
			if blocked, wait := authReserve(ip, now); blocked {
				abortRateLimited(c, wait)
				return
			}
			authRelease(ip, authFailed, now)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid device token",
				"message": "设备令牌无效或已被吊销。请到网页控制台重新配对(设置 → App 管理)",
				"code":    "INVALID_DEVICE_TOKEN",
			})
			return
		}

		// 3. 凭据缺失：若该来源 IP 已处于暴力封禁窗口则返回 429，否则返回 401
		if blocked, wait := authReserve(ip, now); blocked {
			abortRateLimited(c, wait)
			return
		}
		authRelease(ip, authNeutral, now)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "Missing API key",
			"message": "缺少API密钥，请通过官方前端访问",
			"code":    "NO_API_KEY",
		})
	}
}

// RequireMasterKey 高敏管理接口专用守卫中间件（BUG-SEC-10）：
// 仅允许持有主密钥（X-API-Key）的管理员调用，拒绝移动端设备令牌（Bearer）越权操作。
func RequireMasterKey(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}
		if _, isDevice := c.Get("app_device_id"); isDevice || c.GetString(AuthSourceContextKey) == AuthSourceDeviceToken {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Master API key required",
				"message": "该高敏管理操作仅限主密钥 (X-API-Key) 执行，移动端设备令牌无权调用",
				"code":    "FORBIDDEN_DEVICE_TOKEN",
			})
			return
		}
		if c.GetString(AuthSourceContextKey) == AuthSourceMasterKey {
			c.Next()
			return
		}
		key := c.GetHeader("X-API-Key")
		if key != "" && cfg.APIKey != "" && subtle.ConstantTimeCompare([]byte(key), []byte(cfg.APIKey)) == 1 {
			c.Set(AuthSourceContextKey, AuthSourceMasterKey)
			c.Next()
			return
		}
		if strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Master API key required",
				"message": "该高敏管理操作仅限主密钥 (X-API-Key) 执行，移动端设备令牌无权调用",
				"code":    "FORBIDDEN_DEVICE_TOKEN",
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":   "Missing API key",
			"message": "缺少主密钥 (X-API-Key)",
			"code":    "NO_API_KEY",
		})
	}
}

// resolveClientIP 失败限流的计数键。
//
// 必须用 gin 的 ClientIP():它只在请求来自 TRUSTED_PROXIES 声明的代理时才读
// X-Forwarded-For / X-Real-IP。
func resolveClientIP(c *gin.Context) string {
	return c.ClientIP()
}
