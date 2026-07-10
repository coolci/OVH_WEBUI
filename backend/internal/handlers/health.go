package handlers

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// Health 健康检查（GET /health 与 GET /api/health 共用）
// Docker / K8s / 负载均衡探活用；免鉴权。
func Health() gin.HandlerFunc {
	started := time.Now()
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"time":    time.Now().Format(time.RFC3339),
			"uptime":  int(time.Since(started).Seconds()),
			"port":    os.Getenv("PORT"),
			"service": "ovh-webui",
		})
	}
}
