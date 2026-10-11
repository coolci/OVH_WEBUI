package router

import (
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/auth"
	"github.com/ovh-webui/server/internal/handlers"
	"github.com/ovh-webui/server/internal/monitor"
)

// Config 路由装配上下文配置
type Config struct {
	State      *app.State
	Monitor    *monitor.Monitor
	EnableAuth bool
	HasUI      bool
	WebDistFS  fs.FS
	OnRestart  func(exe string)
}

// New 构建并装配 Gin Web 路由引擎
func New(cfg Config, console *slog.Logger) *gin.Engine {
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 默认不信任任何代理。gin 默认信任所有代理 + 读 X-Forwarded-For,
	// 真的在反向代理后面跑,用 TRUSTED_PROXIES 显式声明(逗号分隔 IP/CIDR)。
	var trusted []string
	for _, p := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			trusted = append(trusted, p)
		}
	}
	if err := r.SetTrustedProxies(trusted); err != nil && console != nil {
		console.Warn("SetTrustedProxies 失败,将按不信任代理处理", "err", err)
	}

	// 核心安全与恢复中间件
	r.Use(gin.Recovery())
	r.Use(NewCORSMiddleware())

	// API 鉴权中间件
	r.Use(auth.Middleware(auth.Config{
		APIKey:           cfg.State.APIKey,
		Enabled:          cfg.EnableAuth,
		WhitelistPaths:   auth.DefaultWhitelist(),
		DeviceTokenValid: cfg.State.DB.DeviceTokenValid,
	}))

	// 全局健康检查
	r.GET("/health", handlers.Health())

	// /api 路由组
	api := r.Group("/api")
	{
		registerAPIRoutes(api, cfg)
		registerServerControlRoutes(api, cfg)
		registerVPSControlRoutes(api, cfg)
		registerVPSMonitorAndSupportRoutes(api, cfg)
	}

	// 静态资源与前端 SPA 路由接管
	MountEmbeddedUI(r, cfg.WebDistFS, cfg.HasUI)

	return r
}
