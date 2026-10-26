package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// GetQueue GET /api/queue
func GetQueue(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if state.OrderService != nil {
			c.JSON(http.StatusOK, state.OrderService.ListQueue())
			return
		}
		state.QueueMu.Lock()
		// 返回副本，确保为空时序列化为 [] 而不是 null
		cp := make([]types.QueueItem, len(state.Queue))
		copy(cp, state.Queue)
		state.QueueMu.Unlock()
		c.JSON(http.StatusOK, cp)
	}
}

// GetPurchaseHistory GET /api/purchase-history
func GetPurchaseHistory(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if state.OrderService != nil {
			c.JSON(http.StatusOK, state.OrderService.ListHistory())
			return
		}
		state.HistoryMu.Lock()
		cp := make([]types.PurchaseHistoryEntry, len(state.History))
		copy(cp, state.History)
		state.HistoryMu.Unlock()
		c.JSON(http.StatusOK, cp)
	}
}
