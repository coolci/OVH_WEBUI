package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/types"
)

// GetStats GET /api/stats
// 1:1 对应 Python app.py:402-405: monitor_running 直接读 monitor.running，
// 这样 monitor goroutine 异常退出时 UI 能立刻反映 false
func GetStats(state *app.State, mon *monitor.Monitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		success, failed := state.CountPurchase()
		state.ServerPlansMu.RLock()
		total := len(state.ServerPlans)
		state.ServerPlansMu.RUnlock()
		s := types.Stats{
			ActiveQueues:          state.CountActiveQueues(),
			TotalServers:          total,
			AvailableServers:      state.CountAvailableServers(),
			PurchaseSuccess:       success,
			PurchaseFailed:        failed,
			QueueProcessorRunning: state.QueueProcessorRunning,
			MonitorRunning:        mon != nil && mon.Running(),
		}
		c.JSON(http.StatusOK, s)
	}
}
