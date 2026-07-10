package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

// GetQueue GET /api/queue
func GetQueue(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		state.QueueMu.Lock()
		defer state.QueueMu.Unlock()
		// 返回副本，确保为空时序列化为 [] 而不是 null
		cp := make([]interface{}, 0, len(state.Queue))
		for _, it := range state.Queue {
			cp = append(cp, it)
		}
		c.JSON(http.StatusOK, cp)
	}
}

// GetPurchaseHistory GET /api/purchase-history
func GetPurchaseHistory(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		state.HistoryMu.Lock()
		defer state.HistoryMu.Unlock()
		cp := make([]interface{}, 0, len(state.History))
		for _, h := range state.History {
			cp = append(cp, h)
		}
		c.JSON(http.StatusOK, cp)
	}
}
