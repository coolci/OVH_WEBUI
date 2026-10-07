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
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求参数解析失败: " + err.Error()})
			return
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
