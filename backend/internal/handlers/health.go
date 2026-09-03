package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Health 健康检查（GET /health 与 GET /api/health 共用）
func Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	}
}
