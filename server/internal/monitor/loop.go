package monitor

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/purchase"
)

// tgRecheckInterval loop 内 TG 健康检查节流间隔。5 分钟 verify 一次,
// 失败立即调 Stop() 自停监控。
const tgRecheckInterval = 5 * time.Minute

// checkNotifyOrStop 节流后体检通知通道。
//
// 以前这里在所有通道失效时直接停掉整个监控 —— 于是当 bot 被封、网络超时、
// 或用户仅需要后台自动下单而不配置 TG 时，监控循环和抢购引擎直接被杀停。
// 现在通道不可用时仅记录 Warning 日志提示，不杀停监控循环，保证自动下单和轮询继续工作。
//
// 返回 true=继续 loop。
func (m *Monitor) checkNotifyOrStop() bool {
	m.tgCheckMu.Lock()
	due := time.Since(m.lastTGCheck) >= tgRecheckInterval
	m.tgCheckMu.Unlock()
	if !due {
		return true
	}
	ok, reason := notify.AnyAvailable(m.state, false) // 周期性只查配置:真 POST 对 Server酱/Bark 就是发通知(issue #2)
	m.tgCheckMu.Lock()
	m.lastTGCheck = time.Now()
	m.tgCheckMu.Unlock()
	if !ok {
		m.state.Logger.Warn("当前未配置可用通知通道("+reason+")，通知推送已静默；监控与自动下单引擎保持运行。", "monitor")
		return true
	}
	return true
}

func (m *Monitor) CheckNewServers(currentServerList []map[string]interface{}) {
	current := map[string]struct{}{}
	for _, s := range currentServerList {
		if pc, ok := s["planCode"].(string); ok && pc != "" {
			current[pc] = struct{}{}
		}
	}
	m.subsMu.Lock()
	if len(m.knownServers) == 0 {
		m.knownServers = current
		m.subsMu.Unlock()
		m.state.Logger.Info(fmt.Sprintf("初始化已知服务器列表: %d 台", len(current)), "monitor")
		return
	}
	newServers := map[string]bool{}
	for k := range current {
		if _, ok := m.knownServers[k]; !ok {
			newServers[k] = true
		}
	}
	if len(newServers) > 0 {
		m.knownServers = current
	}
	m.subsMu.Unlock()
	if len(newServers) == 0 {
		return
	}
	// 通知在锁外发:SendNewServerAlert 是网络请求,持 subsMu 发会卡住所有订阅读写
	for _, s := range currentServerList {
		if pc, _ := s["planCode"].(string); newServers[pc] {
			m.SendNewServerAlert(s)
		}
	}
	m.state.Logger.Info(fmt.Sprintf("检测到 %d 台新服务器上架", len(newServers)), "monitor")
}

func (m *Monitor) runSubscriptionCheck(sub *Subscription, traceID string) {
	planCode := sub.PlanCode
	m.state.Logger.Info("开始处理订阅: "+planCode, "monitor")
	m.CheckAvailabilityChange(sub, traceID)
	m.state.Logger.Info("完成处理订阅: "+planCode, "monitor")
}

// monitorLoop 兼容入口:按当前代际号跑
func (m *Monitor) monitorLoop() {
	m.subsMu.Lock()
	gen := m.generation
	m.subsMu.Unlock()
	m.monitorLoopGen(gen)
}

// stillMine 这个循环该不该继续:running 为真且代际号还是自己那一代
func (m *Monitor) stillMine(gen int64) bool {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	return m.running && m.generation == gen
}

func (m *Monitor) monitorLoopGen(gen int64) {
	m.state.Logger.Info("监控循环已启动", "monitor")
	for {
		if !m.stillMine(gen) {
			break
		}

		// 周期性体检通知通道。若无可用通道仅 Warn 提示，不杀停监控循环。
		if !m.checkNotifyOrStop() {
			break
		}

		m.cleanupExpiredCaches()

		roundStart := time.Now()
		m.subsMu.Lock()
		count := len(m.subscriptions)
		subsCopy := make([]*Subscription, count)
		copy(subsCopy, m.subscriptions)
		interval := m.checkInterval
		m.subsMu.Unlock()

		if count > 0 {
			// 确保处于激活状态的自动下单订阅在后台保持预热车待命
			m.EnsureAllPrewarmedCarts()

			m.state.Logger.Info(fmt.Sprintf("开始检查 %d 个订阅...", count), "monitor")
			workers := m.maxWorkers
			if count < workers {
				workers = count
			}
			if workers < 1 {
				workers = 1
			}
			sem := make(chan struct{}, workers)
			var wg sync.WaitGroup
			for _, sub := range subsCopy {
				if sub.checkConfig().Paused {
					continue // 暂停的订阅:保留配置与历史,本轮不检查
				}
				running := m.stillMine(gen)
				if !running {
					break
				}
				if !m.stillInSubscriptions(sub) {
					m.state.Logger.Debug(fmt.Sprintf("订阅 %s 在检查期间被删除，跳过", sub.PlanCode), "monitor")
					continue
				}
				traceID := uuid.NewString()
				wg.Add(1)
				sem <- struct{}{}
				go func(s *Subscription, tid string) {
					defer wg.Done()
					defer func() { <-sem }()
					defer func() {
						if r := recover(); r != nil {
							m.state.Logger.Error(fmt.Sprintf("[trace:%s] 并发检查订阅 %s 时异常: %v",
								tid, s.PlanCode, r), "monitor")
						}
					}()
					m.runSubscriptionCheck(s, tid)
				}(sub, traceID)
			}
			wg.Wait()

			// 本轮改出了新状态就落库。
			// 以前这一步根本不存在:LastStatus / History 只活在内存里,
			// 只有用户手动增删改订阅时才顺带被写进去。程序重启后 LastStatus 是空的,
			// 下一轮就把"现在有货"误判成首次检查的初始存量 —— 不通知,也不自动下单。
			// 用户重启一次,就正好错过一次补货。
			if m.dirty.Swap(false) {
				m.SaveToDB()
			}
		} else {
			m.state.Logger.Info("当前无订阅，跳过检查", "monitor")
		}

		// 等下次（可中断 sleep）。按"上一轮开始 + 间隔"定点起跑,而不是本轮跑完再睡满一个间隔:
		// 以前实际检查周期 = 本轮耗时(验价 / 发通知 / 下单)+ 间隔,补货检测平白慢了一整轮的耗时。
		// 本轮已经超过间隔时立即开始下一轮。
		if m.stillMine(gen) {
			wait := nextRoundDelay(roundStart, time.Now(), interval)
			m.state.Logger.Info(fmt.Sprintf("本轮耗时 %.1f 秒,%.1f 秒后进行下次检查...",
				time.Since(roundStart).Seconds(), wait.Seconds()), "monitor")
			deadline := time.Now().Add(wait)
			for m.stillMine(gen) {
				left := time.Until(deadline)
				if left <= 0 {
					break
				}
				if left > 200*time.Millisecond {
					left = 200 * time.Millisecond
				}
				time.Sleep(left)
			}
		}
	}
	m.state.Logger.Info("监控循环已停止", "monitor")
}

// nextRoundDelay 下一轮距现在还要等多久:以本轮开始时间为基准定点起跑,超时则不等。
func nextRoundDelay(roundStart, now time.Time, intervalSec int) time.Duration {
	wait := roundStart.Add(time.Duration(intervalSec) * time.Second).Sub(now)
	if wait < 0 {
		return 0
	}
	return wait
}

func (m *Monitor) stillInSubscriptions(sub *Subscription) bool {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	for _, s := range m.subscriptions {
		if s == sub {
			return true
		}
	}
	return false
}

// 配置改变或暂停后,旧一轮即使完成了网络请求也不能触发通知或下单。
func (m *Monitor) checkCurrent(sub *Subscription, cfg subCheckConfig) bool {
	if sub.checkParent != nil {
		sub = sub.checkParent
	}
	if !m.stillInSubscriptions(sub) {
		return false
	}
	sub.mu.Lock()
	defer sub.mu.Unlock()
	return !sub.Paused && sub.revision == cfg.Revision
}

func (m *Monitor) Start() bool {
	m.subsMu.Lock()
	// 手动启动/新增订阅也会解除全局停止记录;失败时不启动。
	if m.state.DB != nil {
		if err := m.state.DB.SetKV("monitor-manually-stopped", false); err != nil {
			m.subsMu.Unlock()
			m.state.Logger.Error("保存监控启动状态失败: "+err.Error(), "monitor")
			return false
		}
	}
	m.state.SetMonitorStopped(false)
	if m.running {
		m.subsMu.Unlock()
		m.state.Logger.Warn("监控已在运行中", "monitor")
		return false
	}
	m.running = true
	m.generation++
	gen := m.generation
	// MonitorRunning 与 checkInterval 都要在锁内取/写:
	// Start 可能与 loop 自停时调的 Stop、以及 SetCheckInterval 并发,
	// 出锁之后再写就是无同步的写-写竞争(go test -race 会报)。
	m.state.MonitorRunning = true
	interval := m.checkInterval
	m.loops.Add(1)
	m.subsMu.Unlock()
	// 重置 TG 检查时间戳,保证启动后第一轮一定 verify
	m.tgCheckMu.Lock()
	m.lastTGCheck = time.Time{}
	m.tgCheckMu.Unlock()
	go func() {
		defer m.loops.Done()
		m.monitorLoopGen(gen)
	}()
	m.state.Logger.Info(fmt.Sprintf("服务器监控已启动 (检查间隔: %d秒)", interval), "monitor")
	return true
}

// WaitStopped 等所有检查循环退出(最多 timeout),关库前调用。返回 false = 超时。
// 先 Stop 再调;循环在每个检查点看 stillMine,本轮在途的验价/入队收尾后退出。
func (m *Monitor) WaitStopped(timeout time.Duration) bool {
	return waitGroupTimeout(&m.loops, timeout)
}

func waitGroupTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Stop 只停止本轮进程,内部健康检查也会调用。
func (m *Monitor) Stop() bool {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	return m.stopLocked()
}

// StopByUser 将停止意图与循环停止串行化,避免并发启动覆盖持久状态。
func (m *Monitor) StopByUser() (bool, int, error) {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	m.state.SetMonitorStopped(true)
	stopped := m.stopLocked()
	count, pauseErr := m.state.PauseMonitorQueues(m.queueScopesLocked(true))
	if m.state.DB == nil {
		return stopped, count, errors.Join(pauseErr, fmt.Errorf("数据库不可用,不能保存停止状态"))
	}
	err := m.state.DB.SetKV("monitor-manually-stopped", true)
	return stopped, count, errors.Join(pauseErr, err)
}

// stopLocked 使循环和已取出的所有检查快照同时失效。
func (m *Monitor) stopLocked() bool {
	if !m.running {
		m.state.Logger.Warn("监控未运行", "monitor")
		return false
	}
	m.running = false
	// 让正在等库存/询价结果的旧检查失效,快速重启也不能复活。
	for _, sub := range m.subscriptions {
		sub.mu.Lock()
		sub.revision++
		sub.mu.Unlock()
	}
	m.state.MonitorRunning = false
	m.state.Logger.Info("正在停止服务器监控...", "monitor")
	return true
}

// batchOrder 监控触发的批量下单:逐个调本地 quick-order 入队。
// accountID:auto_order 账户;空时 batchOrder 不应该被调到(check.go 的 guard 已挡住),
// 这里再做一次防御性检查。
func (m *Monitor) batchOrder(planCode string, configInfo map[string]interface{}, targets []notification, quantity int, accountID string, autoPay bool, shouldContinue func() bool, guards ...func(func() error) error) {
	if accountID == "" {
		m.state.Logger.Warn("[monitor->order] 跳过自动下单: 订阅未指定 auto_order 账户", "monitor")
		return
	}
	if quantity < 1 {
		return
	}
	totalOrders := len(targets) * quantity
	m.state.Logger.Info(fmt.Sprintf("[monitor->order] 开始批量下单: %s, 配置数=1, 数据中心数=%d, 数量=%d, 总订单数=%d",
		planCode, len(targets), quantity, totalOrders), "monitor")
	m.state.Logger.Info("[monitor->order] 下单条件：仅对从无货变有货的情况下单（过滤掉持续有货的情况）", "monitor")

	options := []string{}
	if configInfo != nil {
		if opts, ok := configInfo["options"].([]string); ok {
			options = opts
		} else if optsRaw, ok := configInfo["options"].([]interface{}); ok {
			for _, o := range optsRaw {
				if s, ok := o.(string); ok {
					options = append(options, s)
				}
			}
		}
	}

	// 每个 DC 一次原子入队整批数量,避免同批第 2 台被自动任务去重挡住。
	// 直接在进程内调用 purchase.EnqueueQuickOrder，避免 HTTP 回环开销与端口依赖。
	// 真正的下单在 ProcessQueueLoop 里按 concurrentBatchSize=10 节流跑。
	type orderTask struct {
		dc string
	}
	tasks := make([]orderTask, 0, len(targets)*quantity)
	for _, n := range targets {
		tasks = append(tasks, orderTask{dc: n.dc})
	}

	var successCount, failCount int64
	var guard func(func() error) error
	if len(guards) > 0 {
		guard = guards[0]
	}
	postOne := func(t orderTask) {
		if shouldContinue != nil && !shouldContinue() {
			return
		}
		m.state.Logger.Info(fmt.Sprintf("[monitor->order] 尝试入队 %d 台: 账户=%s %s@%s, options=%v",
			quantity, accountID, planCode, t.dc, options), "monitor")

		res := purchase.EnqueueQuickOrder(m.state, purchase.QuickOrderParams{
			AccountID:          accountID,
			Quantity:           quantity,
			Force:              true,
			GuardEnqueue:       guard,
			PlanCode:           planCode,
			Datacenter:         t.dc,
			Options:            options,
			FromMonitor:        true,
			SkipDuplicateCheck: true,
			AutoPay:            autoPay,
			Duration:           durationFromConfig(configInfo),
		})

		if res.Success {
			atomic.AddInt64(&successCount, int64(quantity))
			m.state.Logger.Info(fmt.Sprintf("[monitor->order] 已入队 %d 台: 账户=%s %s@%s",
				quantity, accountID, planCode, t.dc), "monitor")
		} else {
			atomic.AddInt64(&failCount, int64(quantity))
			m.state.Logger.Warn(fmt.Sprintf("[monitor->order] %d 台入队失败: 账户=%s %s",
				quantity, accountID, res.Error), "monitor")
		}
	}

	var wg sync.WaitGroup
	for _, t := range tasks {
		wg.Add(1)
		go func(t orderTask) {
			defer wg.Done()
			postOne(t)
		}(t)
	}
	wg.Wait()

	m.state.Logger.Info(fmt.Sprintf("[monitor->order] 批量下单完成: 成功=%d, 失败=%d, 总计=%d",
		atomic.LoadInt64(&successCount), atomic.LoadInt64(&failCount), totalOrders), "monitor")
}

// AutoStartAllowed 重启遵守用户的全局停止记录,读库失败时不自动恢复。
func (m *Monitor) AutoStartAllowed() bool {
	if m.state.DB == nil {
		return false
	}
	var stopped bool
	if _, err := m.state.DB.GetKV("monitor-manually-stopped", &stopped); err != nil {
		m.state.Logger.Error("读取监控停止状态失败,不自动启动: "+err.Error(), "monitor")
		return false
	}
	return !stopped
}
