package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/purchase"
)

// CleanupCarts POST /api/ovh/carts/cleanup
// 手动触发一次遗留购物车清理(后台本来每 6 小时跑一次)。只删带本程序标记、本进程没在用的车。
func CleanupCarts(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
		defer cancel()
		results := purchase.CleanupLeftoverCarts(ctx, state)
		deleted := 0
		for _, r := range results {
			deleted += r.Deleted
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "deleted": deleted, "accounts": results})
	}
}
