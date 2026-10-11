package worker

import (
	"fmt"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/handlers"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/proxyguard"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/vps"
)

// Supervisor 后台工作任务统一编排与生命周期管理者
type Supervisor struct {
	state   *app.State
	monitor *monitor.Monitor
	stopCh  chan struct{}
}

// NewSupervisor 创建后台任务编排器
func NewSupervisor(state *app.State, mon *monitor.Monitor) *Supervisor {
	return &Supervisor{
		state:   state,
		monitor: mon,
		stopCh:  make(chan struct{}),
	}
}

// Start 统一装配并启动所有后台工作协程
func (s *Supervisor) Start() {
	state := s.state
	mon := s.monitor

	// 1. 启动订单队列处理与状态刷新轮询
	go purchase.ProcessQueueLoop(state)
	go purchase.OrderStatusLoop(state)

	// 2. 预热各账户子公司的区域配置
	go catalog.WarmRegionCache(state)

	// 3. 出站代理看门狗初始化与钩子装配
	proxyguard.Init(state)
	if mon != nil {
		proxyguard.SetDisableAutoOrder(mon.DisableAutoOrderForAccount)
		proxyguard.SetReload(func() {
			mon.LoadFromDB()
		})
	}
	state.OVH.SetProxyErrorHook(proxyguard.Report)
	state.SetProxyErrorHook(proxyguard.Report)
	ApplySharedProxy(state)
	state.OVH.SetLogf(func(format string, args ...interface{}) {
		state.Logger.Warn(fmt.Sprintf(format, args...), "proxy")
	})

	// 4. Telegram 长轮询服务
	go telegram.StartPoller(state, func(u map[string]interface{}) {
		handlers.ProcessTelegramUpdate(state, mon, u)
	})

	// 5. 自动启动监控（按需）
	if mon != nil && len(mon.Snapshot()) > 0 {
		mon.Start()
		state.Logger.Info("自动启动服务器监控", "system")
	}
	if len(state.VPSSubscriptions) > 0 {
		if vps.Start(state) {
			state.Logger.Info("自动启动 VPS 监控", "system")
		}
	}

	// 6. 定时清理过期的 App 配对码
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if state.DB != nil {
					state.DB.CleanupPairingCodes()
				}
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop 停止全部后台工作协程（支持超时防卡死）
func (s *Supervisor) Stop() {
	select {
	case <-s.stopCh:
		// already closed
	default:
		close(s.stopCh)
	}

	handlers.StopPoller()
	if s.monitor != nil {
		s.monitor.Stop()
	}
	vps.Stop(s.state)
}

// ApplySharedProxy 把默认账户的代理设为"公开请求"的统一出口。
// 账户增删改之后需要重调 —— 默认账户可能换了。
func ApplySharedProxy(state *app.State) {
	if state == nil {
		return
	}
	acc, ok := state.FindAccount("")
	if !ok {
		return
	}
	if err := netfp.SetSharedProxy(acc.ProxyURL); err != nil {
		if state.Logger != nil {
			state.Logger.Warn("公开请求的统一出口设置失败,将走直连: "+err.Error(), "proxy")
		}
		return
	}
	if p := netfp.SharedProxy(); p != "" {
		if state.Logger != nil {
			state.Logger.Info("公开目录/区域探测统一走: "+p, "proxy")
		}
	}
}
