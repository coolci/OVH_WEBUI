package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// AddQueueItem POST /api/queue
// 多账户:body 必须带 account_id,后端用它确定下单走哪个账户
func AddQueueItem(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			AccountID     string   `json:"account_id"`
			PlanCode      string   `json:"planCode"`
			Datacenter    string   `json:"datacenter"`
			Options       []string `json:"options"`
			RetryInterval int      `json:"retryInterval"`
			// AutoPay 下单成功后用默认支付方式自动付款(显式开关,默认关)
			AutoPay bool `json:"autoPay"`
			// Force 强制添加自定义或未在当前目录收录的型号入队
			Force bool `json:"force"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.AccountID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "缺少 account_id"})
			return
		}
		if _, ok := state.FindAccount(body.AccountID); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "account_id 不存在"})
			return
		}
		body.PlanCode = strings.TrimSpace(body.PlanCode)
		body.Datacenter = strings.TrimSpace(body.Datacenter)
		if body.PlanCode == "" || body.Datacenter == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "缺少 planCode 或 datacenter"})
			return
		}
		// 入队前检查型号归属。若用户明确指定 Force（例如新品或自定义型号），则记日志并放行入队。
		if verdict, hint := catalog.ClassifyPlan(state, body.AccountID, body.PlanCode, "queue"); hint != "" {
			if !body.Force {
				state.Logger.Warn(fmt.Sprintf("[queue] 拒绝任务(判定 %d): %s", verdict, hint), "queue")
				c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": hint, "can_force": true})
				return
			}
			state.Logger.Warn(fmt.Sprintf("[queue] 用户强制添加自定义型号任务(判定 %d): %s 在 %s: %s", verdict, body.PlanCode, body.Datacenter, hint), "queue")
		}
		if body.RetryInterval == 0 {
			body.RetryInterval = 30
		}
		item := types.QueueItem{
			ID:            uuid.NewString(),
			AccountID:     body.AccountID,
			PlanCode:      body.PlanCode,
			Datacenter:    body.Datacenter,
			Options:       body.Options,
			Status:        "running",
			CreatedAt:     types.NowISO(),
			UpdatedAt:     types.NowISO(),
			RetryInterval: body.RetryInterval,
			RetryCount:    0,
			LastCheckTime: 0,
			AutoPay:       body.AutoPay,
			Force:         body.Force,
		}
		state.QueueMu.Lock()
		state.Queue = append(state.Queue, item)
		state.QueueMu.Unlock()
		// 落库失败不撤任务:它已经在内存里跑起来了,撤掉等于用户明确要抢的机器不抢了。
		// 但必须说出来 —— 不落库意味着重启后这条任务就没了,而界面上它看着一切正常。
		warn := ""
		if err := state.SaveQueue(); err != nil {
			warn = "任务已在本次运行中启动，但没能写进数据库，重启后会丢失：" + err.Error()
			state.Logger.Error("添加任务后保存队列失败: "+err.Error(), "queue")
		}
		state.Logger.Info("添加任务 "+item.ID+" ("+item.PlanCode+" 在 "+item.Datacenter+", 账户 "+body.AccountID+") 到队列并立即启动 (状态: running)", "")
		resp := gin.H{"status": "success", "id": item.ID}
		if warn != "" {
			resp["warning"] = warn
		}
		c.JSON(http.StatusOK, resp)
	}
}

// RemoveQueueItem DELETE /api/queue/:id
func RemoveQueueItem(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		state.DeletedTaskIDsMu.Lock()
		state.DeletedTaskIDs[id] = struct{}{}
		state.DeletedTaskIDsMu.Unlock()
		state.Logger.Info("标记任务 "+id+" 为删除，后台线程将立即停止处理", "system")

		state.QueueMu.Lock()
		var removed *types.QueueItem
		// 重新分配新 slice，避免 [:0] 与原 backing array 共享导致快照读到已被覆盖的元素
		kept := make([]types.QueueItem, 0, len(state.Queue))
		for i := range state.Queue {
			if state.Queue[i].ID == id {
				cp := state.Queue[i]
				removed = &cp
				continue
			}
			kept = append(kept, state.Queue[i])
		}
		state.Queue = kept
		state.QueueMu.Unlock()
		if err := state.SaveQueue(); err != nil {
			// 删除没落库 → 重启后这条任务会"复活"并继续抢。必须告诉用户。
			state.Logger.Error("删除任务后保存队列失败: "+err.Error(), "queue")
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "已从运行中的队列移除，但没能写进数据库，重启后这条任务会重新出现：" + err.Error(),
			})
			return
		}
		if removed != nil {
			state.Logger.Info("Removed "+removed.PlanCode+" from queue (ID: "+id+")", "system")
			if removed.TelegramMessageID != 0 && strings.TrimSpace(removed.TelegramChatID) != "" {
				telegram.NotifyTaskProgress(state, removed, "cancelled", map[string]string{
					"reason": "已在网页控制台删除此任务",
				})
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// ClearQueue DELETE /api/queue/clear
func ClearQueue(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		state.QueueMu.Lock()
		count := len(state.Queue)
		oldQueue := append([]types.QueueItem{}, state.Queue...)
		state.DeletedTaskIDsMu.Lock()
		for _, it := range state.Queue {
			state.DeletedTaskIDs[it.ID] = struct{}{}
		}
		state.DeletedTaskIDsMu.Unlock()
		state.Queue = []types.QueueItem{}
		state.QueueMu.Unlock()
		if err := state.SaveQueue(); err != nil {
			state.Logger.Error("清空队列后保存失败: "+err.Error(), "queue")
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "已清空运行中的队列，但没能写进数据库，重启后这些任务会重新出现：" + err.Error(),
			})
			return
		}

		for _, it := range oldQueue {
			if it.TelegramMessageID != 0 && strings.TrimSpace(it.TelegramChatID) != "" {
				itCopy := it
				telegram.NotifyTaskProgress(state, &itCopy, "cancelled", map[string]string{
					"reason": "已在网页控制台清空抢购队列",
				})
			}
		}

		state.Logger.Info("Cleared all queue items ("+strconv.Itoa(count)+" items removed)", "")
		c.JSON(http.StatusOK, gin.H{"status": "success", "count": count})
	}
}

// UpdateQueueStatus PUT /api/queue/:id/status
func UpdateQueueStatus(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var body struct {
			Status string `json:"status"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Status == "" {
			body.Status = "pending"
		}
		state.QueueMu.Lock()
		var target *types.QueueItem
		for i := range state.Queue {
			if state.Queue[i].ID == id {
				state.Queue[i].Status = body.Status
				state.Queue[i].UpdatedAt = types.NowISO()
				cp := state.Queue[i]
				target = &cp
				state.Logger.Info("Updated "+state.Queue[i].PlanCode+" status to "+body.Status, "")
				break
			}
		}
		state.QueueMu.Unlock()
		if err := state.SaveQueue(); err != nil {
			state.Logger.Error("改任务状态后保存队列失败: "+err.Error(), "queue")
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "状态已在本次运行中改掉，但没能写进数据库，重启后会回到原状态：" + err.Error(),
			})
			return
		}

		if target != nil && target.TelegramMessageID != 0 && strings.TrimSpace(target.TelegramChatID) != "" {
			if body.Status == "paused" {
				telegram.NotifyTaskProgress(state, target, "paused", nil)
			} else if body.Status == "running" {
				telegram.NotifyTaskProgress(state, target, "queued", nil)
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// RefreshOrderStatuses POST /api/purchase-history/refresh-status
//
// 手动刷新所有未到终态订单的支付状态(GET /me/order/{id}/status)。
// 后台每 10 分钟也会自动刷,这里是给"我刚付完款想马上看到"的场景。
func RefreshOrderStatuses(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		n := purchase.RefreshOrderStatuses(state, true)
		c.JSON(http.StatusOK, gin.H{"success": true, "updated": n})
	}
}

// ClearPurchaseHistory DELETE /api/purchase-history
func ClearPurchaseHistory(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		state.HistoryMu.Lock()
		state.History = state.History[:0]
		state.HistoryMu.Unlock()
		_ = state.SaveHistory()
		state.Logger.Info("Purchase history cleared", "")
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}
