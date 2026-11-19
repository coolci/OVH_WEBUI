package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// GetLogs GET /api/logs
func GetLogs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		entries := state.Logger.Snapshot()
		level := strings.ToUpper(strings.TrimSpace(c.Query("level")))
		source := strings.ToLower(strings.TrimSpace(c.Query("source")))
		order := strings.ToLower(strings.TrimSpace(c.Query("order")))
		limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))

		if level != "" || source != "" {
			filtered := make([]types.LogEntry, 0, len(entries))
			for _, e := range entries {
				if level != "" && !strings.EqualFold(e.Level, level) {
					continue
				}
				if source != "" && !strings.EqualFold(e.Source, source) {
					continue
				}
				filtered = append(filtered, e)
			}
			entries = filtered
		}
		if order == "desc" {
			for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
		if limit > 0 && len(entries) > limit {
			if order == "desc" {
				entries = entries[:limit]
			} else {
				entries = entries[len(entries)-limit:]
			}
		}
		c.JSON(http.StatusOK, entries)
	}
}

// FlushLogs POST /api/logs/flush
func FlushLogs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		state.Logger.Flush()
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "日志已刷新", "code": "ECEA0FB83"})
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
