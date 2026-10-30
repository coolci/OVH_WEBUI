package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/purchase"
)

// QuickOrder POST /api/queue/quick-order
// 外部主动调用或前端触发: 委托给领域层 purchase.EnqueueQuickOrder 执行校验与入队。
func QuickOrder(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var params purchase.QuickOrderParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "任务参数格式不正确"})
			return
		}
		// 监控已在进程内直接调用 purchase.EnqueueQuickOrder;HTTP 入口不能声称自己来自监控,
		// 否则任意调用方都能跳过重复下单检查和自动建单停止闸门。
		params.FromMonitor = false
		params.SkipDuplicateCheck = false
		if params.AccountID == "" && params.AccountIDCamel != "" {
			params.AccountID = params.AccountIDCamel
		}
		if params.AccountID == "" {
			if acc := c.Query("account"); acc != "" {
				params.AccountID = acc
			} else if acc := c.Query("accountId"); acc != "" {
				params.AccountID = acc
			}
		}

		res := purchase.EnqueueQuickOrder(state, params)
		if !res.Success {
			resp := gin.H{
				"success": false,
				"error":   res.Error,
			}
			if res.Code != "" {
				resp["code"] = res.Code
			}
			c.JSON(res.HTTPStatus, resp)
			return
		}

		resp := gin.H{
			"success": true,
			"message": res.Message,
			"options": res.Options,
		}
		if res.Price != nil {
			resp["price"] = res.Price
		}
		c.JSON(http.StatusOK, resp)
	}
}
