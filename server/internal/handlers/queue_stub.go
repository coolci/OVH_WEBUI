package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/types"
)

// queueOrderView 保留原任务字段,附加成功订单编号与目录型号名;不依赖队列数据库额外列。
type queueOrderView struct {
	types.QueueItem
	OrderID  string `json:"orderId,omitempty"`
	PlanName string `json:"planName,omitempty"`
}

// planNamesFromCache 从目录缓存提取 planCode → 人类可读型号名，按任务所属账户分桶查找。
func planNamesFromCache(state *app.State, items []types.QueueItem) map[string]string {
	names := make(map[string]string)
	byBucket := make(map[string]map[string]string)
	lookupBucket := func(accountID string) map[string]string {
		key := state.ServerCacheKey(accountID)
		if m, ok := byBucket[key]; ok {
			return m
		}
		m := make(map[string]string)
		if plans, _ := state.ServerCache.GetBucket(key); len(plans) > 0 {
			for _, p := range plans {
				if p.PlanCode != "" && p.Name != "" {
					m[p.PlanCode] = p.Name
				}
			}
		} else if key == app.DefaultServerBucket {
			state.ServerPlansMu.RLock()
			for _, p := range state.ServerPlans {
				if p.PlanCode != "" && p.Name != "" {
					m[p.PlanCode] = p.Name
				}
			}
			state.ServerPlansMu.RUnlock()
		}
		byBucket[key] = m
		return m
	}
	for _, it := range items {
		k := it.AccountID + "\x00" + it.PlanCode
		if _, exists := names[k]; exists {
			continue
		}
		if name := lookupBucket(it.AccountID)[it.PlanCode]; name != "" {
			names[k] = name
		}
	}
	return names
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
		planNames := planNamesFromCache(state, cp)
		view := make([]queueOrderView, len(cp))
		for i, item := range cp {
			view[i].QueueItem = item
			view[i].PlanName = planNames[item.AccountID+"\x00"+item.PlanCode]
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
		var cancellations map[string]db.OrderCancellation
		if len(items) > 0 && state.HasDB() {
			var err error
			cancellations, err = state.DB.ListOrderCancellations()
			if err != nil {
				c.JSON(500, gin.H{"error": "读取订单操作记录失败"})
				return
			}
		}
		view := make([]historyOrderView, len(items))
		for i, item := range items {
			view[i].PurchaseHistoryEntry = item
			id, err := strconv.ParseInt(item.OrderID, 10, 64)
			if err != nil || id <= 0 || cancellations == nil {
				continue
			}
			if action, ok := cancellations[db.OrderCancellationKey(item.AccountID, id)]; ok {
				view[i].OrderCancellation = action
			}
		}
		c.JSON(http.StatusOK, view)
	}
}
