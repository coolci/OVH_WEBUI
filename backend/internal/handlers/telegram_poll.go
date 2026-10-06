package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/telegram"
)

// 长轮询（getUpdates）的接线 —— 收 Telegram update 的唯一一条路。
//
// webhook 那条路已经删掉了：它要求公网 HTTPS 域名 + 受信证书，
// 而且需要把 /api/telegram/webhook 放进鉴权白名单（Telegram 不可能带 X-API-Key），
// 于是只能靠 secret_token 证明来源，还得为老部署留一个"没注册 secret"的兼容模式。
// 长轮询没有入站端点，这一整套东西连同它的出错面一起消失了。

var globalPoller *telegram.Poller

// InitPoller 在 main 里调用一次，把 update 处理函数注入 poller。
// 处理逻辑和 webhook 完全共用 ProcessUpdate，不另写一份。
func InitPoller(state *app.State, mon *monitor.Monitor) *telegram.Poller {
	globalPoller = telegram.NewPoller(state, func(data map[string]interface{}) {
		u := ProcessUpdate(state, mon, data)
		if u.status >= 400 {
			state.Logger.Debug("长轮询处理 update 未通过: "+http.StatusText(u.status), "telegram")
		}
	})
	return globalPoller
}

// StartPollerIfEnabled 启动时拉起长轮询。配了 Token 就跑 —— 没有别的模式可选。
func StartPollerIfEnabled(state *app.State) {
	if globalPoller == nil {
		return
	}
	if strings.TrimSpace(state.Config.Get().TgToken) == "" {
		return
	}
	if err := globalPoller.Start(); err != nil {
		state.Logger.Error("启动 Telegram 长轮询失败: "+err.Error(), "telegram")
	}
}

// RestartPoller 配置变更(换 Token / 首次填 Token)后重新拉起。
// Start 是幂等的,已经在跑就什么都不做;Token 变了必须先停再起,
// 否则旧循环还拿着旧 Token 在拉,新 Token 那边一条消息都收不到。
func RestartPoller(state *app.State) {
	if globalPoller == nil {
		return
	}
	globalPoller.Stop()
	StartPollerIfEnabled(state)
}

// StopPoller 进程退出时调用。
func StopPoller() {
	if globalPoller != nil {
		globalPoller.Stop()
	}
}

// GetTelegramPollerStatus GET /api/telegram/poller
//
// 设置页用它显示"到底有没有在收消息"。长轮询是唯一的收取方式,
// 它停了就等于一键下单、文本下单、所有命令全部失效,而界面上不会有任何别的迹象。
func GetTelegramPollerStatus(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := state.Config.Get()
		out := gin.H{
			"success":  true,
			"hasToken": strings.TrimSpace(cfg.TgToken) != "",
		}
		if globalPoller != nil {
			out["poller"] = globalPoller.Status()
		}
		c.JSON(http.StatusOK, out)
	}
}
