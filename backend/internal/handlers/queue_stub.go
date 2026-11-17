package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// queueOrderView 保留原任务字段,附加成功订单编号;不依赖队列数据库额外列。
type queueOrderView struct {
	types.QueueItem
	OrderID string `json:"orderId,omitempty"`
}

// GetQueue GET /api/queue
func GetQueue(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 快照永不为 nil,空队列序列化为 [] 而不是 null
		cp := state.QueueSnapshot()
		orders := make(map[string]string)
		for _, h := range state.HistorySnapshot() {
			if h.Status == "success" && h.OrderID != "" {
				orders[h.TaskID] = h.OrderID
			}
		}
		view := make([]queueOrderView, len(cp))
		for i, item := range cp {
			view[i].QueueItem = item
			if item.Status == "completed" {
				view[i].OrderID = orders[item.ID]
			}
		}
		c.JSON(http.StatusOK, view)
	}
}

// GetPurchaseHistory GET /api/purchase-history
func GetPurchaseHistory(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		items := state.HistorySnapshot()
		view := make([]historyOrderView, len(items))
		for i, item := range items {
			view[i].PurchaseHistoryEntry = item
			id, err := strconv.ParseInt(item.OrderID, 10, 64)
			if err != nil || id <= 0 || !state.HasDB() {
				continue
			}
			action, err := state.DB.GetOrderCancellation(item.AccountID, id)
			if err != nil {
				c.JSON(500, gin.H{"error": "读取订单操作记录失败"})
				return
			}
			view[i].OrderCancellation = action
		}
		c.JSON(http.StatusOK, view)
	}
}
