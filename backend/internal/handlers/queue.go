package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/platform/httperr"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/service/order"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// AddQueueItem POST /api/queue
// 多账户:body 必须带 account_id,后端用它确定下单走哪个账户
func AddQueueItem(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			AccountID      string   `json:"account_id"`
			AccountIDCamel string   `json:"accountId"`
			PlanCode       string   `json:"planCode"`
			Datacenter     string   `json:"datacenter"`
			Options        []string `json:"options"`
			RetryInterval  int      `json:"retryInterval"`
			// AutoPay 下单成功后用默认支付方式自动付款(显式开关,默认关)
			AutoPay bool `json:"autoPay"`
			// Force 强制添加自定义或未在当前目录收录的型号入队
			Force bool `json:"force"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.AccountID == "" && body.AccountIDCamel != "" {
			body.AccountID = body.AccountIDCamel
		}
		if body.AccountID == "" {
			if acc := c.Query("account"); acc != "" {
				body.AccountID = acc
			} else if acc := c.Query("accountId"); acc != "" {
				body.AccountID = acc
			}
		}
		if body.AccountID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "缺少 account_id", "code": "E34CBF1D4"})
			return
		}
		if _, ok := state.FindAccount(body.AccountID); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "account_id 不存在", "code": "E7B315B13"})
			return
		}
		body.PlanCode = strings.TrimSpace(body.PlanCode)
		body.Datacenter = strings.TrimSpace(body.Datacenter)
		if body.PlanCode == "" || body.Datacenter == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "缺少 planCode 或 datacenter", "code": "E75DA4B08"})
			return
		}
		// 入队前挡住"这个账户根本买不到这台机器"的任务(跨区 / 非 Eco / planCode 不存在)。
		// 前端的下单对话框有独立的账户选择器(web/src/routes/servers.tsx:646),
		// 机型列表却是按另一个账户拉的 —— 拿欧区机型配美区账户是一键就能做出来的组合。
		// 而这种任务进了队列后:availabilities 返回 200 + 空数组 → PurchaseServer 判"无货"
		// → 按 retryInterval 永远重试,日志里永远只有一句"当前无货",用户看不出错在哪。
		// 所以在这里就说清楚,而不是让它在后台空转到天荒地老。
		// 探测失败(catalog.PlanVerdictUnknown)不拦 —— 一次网络瞬断不该让用户下不了单。
		// 用户明确指定 Force(新品 / 自定义型号)时,只放过"目录没收录"这一类,记日志后入队;
		// 跨区 / 非 Eco 仍然拦,Force 不能拿来绕过确定性买不到的组合。
		if verdict, hint := catalog.ClassifyPlan(state, body.AccountID, body.PlanCode, "queue"); hint != "" {
			if body.Force && verdict == catalog.PlanVerdictNoSuchPlan {
				state.Logger.Warn(fmt.Sprintf("[queue] 用户强制添加未收录型号(判定 %d): %s 在 %s: %s", verdict, body.PlanCode, body.Datacenter, hint), "queue")
			} else {
				state.Logger.Warn(fmt.Sprintf("[queue] 拒绝任务(判定 %d): %s", verdict, hint), "queue")
				canForce := verdict == catalog.PlanVerdictNoSuchPlan
				c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": hint, "can_force": canForce})
				return
			}
		}
		// 没给 / 给 0 = 用全局默认(设置页可改);超出区间夹回来
		body.RetryInterval = types.ClampRetryInterval(body.RetryInterval, state.Config.RetryInterval())
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
		// 入队 + 落库由 OrderService 统一保证:失败时自动撤回,不存在半成功任务
		if err := state.OrderService.Enqueue(item, false); err != nil {
			state.Logger.Error("添加任务后保存队列失败,已撤回: "+err.Error(), "queue")
			httperr.InternalError(c, "任务没能写进数据库，已撤回："+err.Error(), "ERR_ENQUEUE_FAILED")
			return
		}
		state.Logger.Info("添加任务 "+item.ID+" ("+item.PlanCode+" 在 "+item.Datacenter+", 账户 "+body.AccountID+") 到队列并立即启动 (状态: running)", "")
		c.JSON(http.StatusOK, gin.H{"status": "success", "id": item.ID})
	}
}

// RemoveQueueItem DELETE /api/queue/:id
func RemoveQueueItem(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		removed, err := state.OrderService.RemoveQueueItem(id)
		if err != nil {
			state.Logger.Error("删除任务后保存队列失败: "+err.Error(), "queue")
			httperr.InternalError(c, "已从运行中的队列移除，但没能写进数据库，重启后这条任务会重新出现："+err.Error(), "ERR_DB_SAVE")
			return
		}
		state.Logger.Info("标记任务 "+id+" 为删除，正在进行的下单已取消，后台线程将停止处理", "system")
		if removed != nil {
			state.Logger.Info("Removed "+removed.PlanCode+" from queue (ID: "+id+")", "system")
			// 云下单:把绑定的 Telegram 进度卡片改成"已终止"(未绑卡片时 NotifyTaskProgress 直接返回)
			telegram.NotifyTaskProgress(state, removed, "cancelled", map[string]string{
				"reason": "已在网页控制台删除此任务",
			})
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// UpdateQueueInterval PUT /api/queue/:id/interval  body: { "retryInterval": 秒 }
func UpdateQueueInterval(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var body struct {
			RetryInterval int `json:"retryInterval"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			httperr.BadRequest(c, fmt.Sprintf("重试间隔必须是 %d ~ %d 之间的整数秒", types.MinRetryInterval, types.MaxRetryInterval), "ERR_INVALID_INTERVAL")
			return
		}
		if err := state.OrderService.UpdateQueueInterval(id, body.RetryInterval); err != nil {
			if errors.Is(err, order.ErrInvalidInterval) {
				httperr.BadRequest(c, err.Error(), "ERR_INVALID_INTERVAL")
				return
			}
			if errors.Is(err, order.ErrItemNotFound) {
				httperr.NotFound(c, "任务不存在", "ERR_QUEUE_NOT_FOUND")
				return
			}
			state.Logger.Error("改任务间隔后保存队列失败: "+err.Error(), "queue")
			httperr.InternalError(c, "间隔已在本次运行中改掉，但没能写进数据库，重启后会回到原值："+err.Error(), "ERR_DB_SAVE")
			return
		}
		state.Logger.Info(fmt.Sprintf("任务 %s 重试间隔改为 %d 秒", id, body.RetryInterval), "queue")
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// ClearQueue DELETE /api/queue/clear
func ClearQueue(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		oldQueue, err := state.OrderService.ClearQueue()
		if err != nil {
			state.Logger.Error("清空队列后保存失败: "+err.Error(), "queue")
			httperr.InternalError(c, "已清空运行中的队列，但没能写进数据库，重启后这些任务会重新出现："+err.Error(), "ERR_DB_SAVE")
			return
		}
		// 云下单:绑定了进度卡片的任务逐条回写"已终止"
		for i := range oldQueue {
			telegram.NotifyTaskProgress(state, &oldQueue[i], "cancelled", map[string]string{
				"reason": "已在网页控制台清空抢购队列",
			})
		}
		state.Logger.Info("Cleared all queue items ("+strconv.Itoa(len(oldQueue))+" items removed)", "")
		c.JSON(http.StatusOK, gin.H{"status": "success", "count": len(oldQueue)})
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

		target, err := state.OrderService.UpdateQueueStatus(id, body.Status)
		if err != nil {
			if errors.Is(err, order.ErrItemNotFound) {
				httperr.NotFound(c, "任务不存在", "ERR_QUEUE_NOT_FOUND")
				return
			}
			if errors.Is(err, order.ErrInvalidStatus) {
				httperr.BadRequest(c, err.Error(), "ERR_INVALID_STATUS")
				return
			}
			state.Logger.Error("改任务状态后保存队列失败: "+err.Error(), "queue")
			httperr.InternalError(c, "状态已在本次运行中改掉，但没能写进数据库，重启后会回到原状态："+err.Error(), "ERR_DB_SAVE")
			return
		}
		state.Logger.Info("Updated "+target.PlanCode+" status to "+target.Status, "")
		// 云下单:网页端暂停/恢复同步到 Telegram 进度卡片
		switch target.Status {
		case "paused":
			telegram.NotifyTaskProgress(state, &target, "paused", nil)
		case "running":
			telegram.NotifyTaskProgress(state, &target, "queued", nil)
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// RefreshOrderStatuses POST /api/purchase-history/refresh-status
//
// 手动刷新所有未到终态订单的支付状态(GET /me/order/{id}/status)。
// 后台每 10 分钟也会自动刷,这里是给"我刚付完款想马上看到"的场景。
func RefreshOrderStatuses(state *app.State) gin.HandlerFunc {
	// 手动刷新会对每条未终态订单各打一次 /me/order/{id},而 force=true 正是用来
	// 跳过那个 2 分钟节流的 —— 等于把限流闸门交给用户的手速。
	// OVH 对 /me 命名空间有自己的限流,打多了返回 429,而抢购主链路
	// (查库存 / 建车 / 结账)跟它共用同一个账户配额:刷历史把配额刷没了,
	// 补货那一刻就抢不到。所以入口这层必须有自己的节流。
	var (
		mu       sync.Mutex
		lastCall time.Time
	)
	const minInterval = 15 * time.Second

	return func(c *gin.Context) {
		mu.Lock()
		if wait := minInterval - time.Since(lastCall); !lastCall.IsZero() && wait > 0 {
			mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": fmt.Sprintf("刷新太频繁,请等 %d 秒。手动刷新会对每条未完成订单各查一次 OVH,"+
					"把账户配额刷光会影响正在跑的抢购。", int(wait.Seconds())+1),
				"retryAfterSeconds": int(wait.Seconds()) + 1,
			})
			return
		}
		lastCall = time.Now()
		mu.Unlock()

		n := purchase.RefreshOrderStatuses(state, true)
		c.JSON(http.StatusOK, gin.H{"success": true, "updated": n})
	}
}

// ClearPurchaseHistory DELETE /api/purchase-history
func ClearPurchaseHistory(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := state.OrderService.ClearHistory(); err != nil {
			state.Logger.Error("清空购买历史失败: "+err.Error(), "queue")
			httperr.InternalError(c, "没能写进数据库，购买历史未被清空："+err.Error(), "ERR_DB_SAVE")
			return
		}
		state.Logger.Info("Purchase history cleared", "")
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}
