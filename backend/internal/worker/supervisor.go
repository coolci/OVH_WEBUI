package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/catalogwatch"
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
	ctx     context.Context
	cancel  context.CancelFunc
	// queueDone 队列处理循环真正返回时关闭。循环返回前会等全部在途任务
	// (包括已发出、不接受取消的结账请求)结束,Stop 必须等它,不然数据库先关、
	// 结账随后返回成功,成功记录写不进库,重启后任务复活再买一台。
	queueDone chan struct{}
}

// stopWait 关停时等后台循环收尾的上限。结账请求不接受取消,
// 最长受 SDK 超时约束;这里给足时间,超时则记错误日志后继续关停
// (此时任务已带着结账未决标记落库,重启后会先对账)。
var stopWait = 3 * time.Minute

// NewSupervisor 创建后台任务编排器
func NewSupervisor(state *app.State, mon *monitor.Monitor) *Supervisor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Supervisor{
		state:   state,
		monitor: mon,
		stopCh:  make(chan struct{}),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 统一装配并启动所有后台工作协程
func (s *Supervisor) Start() {
	state := s.state
	mon := s.monitor

	// 1. 启动订单队列处理与状态刷新轮询
	s.queueDone = make(chan struct{})
	go func() {
		defer close(s.queueDone)
		purchase.ProcessQueueLoopWithContext(s.ctx, state)
	}()
	go purchase.OrderStatusLoopWithContext(s.ctx, state)
	// 清理本程序上次运行留下的购物车(只删带本程序标记、且本进程没在用的)
	go purchase.CartJanitorLoop(s.ctx, state)

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

	// Independent catalog watcher: browser lifecycle never controls this worker.
	catalogwatch.For(state).Run(s.ctx)

	// 5. 自动启动监控（按需）
	// 用户手动停止过监控的话,重启后保持停止(AutoStartAllowed 读库失败时也不自动恢复)
	if mon != nil && len(mon.Snapshot()) > 0 && mon.AutoStartAllowed() {
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

	if s.cancel != nil {
		s.cancel()
	}

	handlers.StopPoller()
	if s.monitor != nil {
		s.monitor.Stop()
	}
	vps.Stop(s.state)

	// 等各循环真正退出再返回:调用方紧接着就会关数据库。
	deadline := time.Now().Add(stopWait)
	left := func() time.Duration {
		if d := time.Until(deadline); d > 0 {
			return d
		}
		return 0
	}
	if s.queueDone != nil {
		select {
		case <-s.queueDone:
		case <-time.After(left()):
			s.logStopTimeout("抢购队列(可能有结账请求仍未返回)")
		}
	}
	if s.monitor != nil && !s.monitor.WaitStopped(left()) {
		s.logStopTimeout("服务器监控")
	}
	if !vps.WaitStopped(left()) {
		s.logStopTimeout("VPS 监控")
	}
	if w, ok := s.state.CatalogWatch.(interface{ Wait(time.Duration) bool }); ok && !w.Wait(left()) {
		s.logStopTimeout("目录上新自动下单")
	}
}

func (s *Supervisor) logStopTimeout(what string) {
	if s.state != nil && s.state.Logger != nil {
		s.state.Logger.Error("关停时等待「"+what+"」退出超时,继续关闭;在途结果以重启后的对账为准", "system")
	}
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
