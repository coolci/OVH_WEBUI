package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/logger"
)

// GetLogs GET /api/logs
//
// Query:
//   - limit  返回条数（默认 200，最大 500）
//   - level  INFO|WARNING|ERROR|DEBUG
//   - source 子串过滤
//   - order  desc(默认，最新在前) | asc
//
// 兼容：无参数时也返回对象 { logs, total, returned }；
// 旧前端若期望数组，可继续用 limit + 解析 logs 字段。
// 读路径不强制 Flush，避免高频繁刷新时反复写盘。
func GetLogs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 0
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		opts := logger.QueryOpts{
			Limit:  limit,
			Level:  c.Query("level"),
			Source: c.Query("source"),
			Order:  c.DefaultQuery("order", "desc"),
		}
		// 兼容旧调用：?limit=10 且未指定 order 时，dashboard 需要「最新 N 条」
		// Query 内部已取尾部再按 order 排列

		items, total := state.Logger.Query(opts)
		effLimit := opts.Limit
		if effLimit <= 0 {
			effLimit = 200
		}
		if effLimit > 500 {
			effLimit = 500
		}

		// 兼容：若客户端显式要数组（flat=1），直接返回数组
		if strings.EqualFold(c.Query("flat"), "1") || strings.EqualFold(c.Query("format"), "array") {
			c.JSON(http.StatusOK, items)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"logs":      items,
			"total":     total,
			"returned":  len(items),
			"truncated": total > len(items),
			"limit":     effLimit,
			"order":     opts.Order,
		})
	}
}

// FlushLogs POST /api/logs/flush
func FlushLogs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		state.Logger.Flush()
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "日志已刷新"})
	}
}

// ClearLogs DELETE /api/logs
func ClearLogs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := state.Logger.Clear(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		state.Logger.Info("Logs cleared", "system")
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}
