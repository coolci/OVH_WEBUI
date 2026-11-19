package router

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MountEmbeddedUI 把嵌入的前端挂到根路径。
// 没启用 -tags ui 时 hasUI 为 false，不注册任何 NoRoute；
// 启用时：/api/* 未匹配 → 404 JSON；命中具体文件 → 直接 serve；其余 → 返回 index.html 让 SPA 路由接管。
//
// 注意：index.html 不能交给 http.FileServer 去 serve，否则它会把 /index.html 301 重定向到 /，
// 触发与 SPA fallback 的相互重定向死循环（ERR_TOO_MANY_REDIRECTS）。
// 直接读出来缓存到内存，命中 SPA 路径时手工写回，绕开 FileServer 的内部行为。
func MountEmbeddedUI(r *gin.Engine, distFS fs.FS, hasUI bool) {
	if !hasUI || distFS == nil {
		return
	}
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		// 没构出 index.html，等于没 UI；退化为纯 API
		return
	}
	fileServer := http.FileServer(http.FS(distFS))

	serveIndex := func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Cache-Control", "no-cache")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(indexHTML)
	}

	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		if strings.HasPrefix(reqPath, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		clean := strings.TrimPrefix(reqPath, "/")
		// 根路径或显式访问 index.html：直接写 index.html，绕开 FileServer 的 301 重定向
		if clean == "" || clean == "index.html" {
			serveIndex(c)
			return
		}
		// 命中具体文件 → FileServer 处理（带正确 Content-Type + 缓存语义）
		if info, err := fs.Stat(distFS, clean); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		// SPA 客户端路由：写 index.html，让前端 router 接管
		serveIndex(c)
	})
}
