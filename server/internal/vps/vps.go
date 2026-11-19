package vps

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/types"
)

var (
	lifecycleMu sync.Mutex
	runningMu   sync.Mutex
	running     bool
	// generation 循环代际号,每次 Start 递增。
	// 只翻 running 布尔的话,"停止后立刻启动"会让旧循环活下来:
	// 它下一个检查点看到 running 又是 true,不退出 → 两个循环并存 →
	// 同一次补货下两次单。VPS 侧界面就一个 停止/启动 按钮,点两下即可复现,
	// 而 VPS 循环退出延迟很大(每订阅一次 10 秒超时的 HTTP + 下单往返)。
	generation int64
	loopWg     *sync.WaitGroup

	// TG 健康检查节流。loop 每 5 分钟 verify 一次,失败自停。
	tgCheckMu   sync.Mutex
	lastTGCheck time.Time
)

const tgRecheckInterval = 5 * time.Minute

// 配置类失败(子公司串区 / planCode 不在该区目录)每轮都会以同样方式复现,
// 按 60s 一轮刷屏没有信息量:同一订阅同一原因只记一次,原因变了或恢复后再记。
var (
	lastCheckErrMu sync.Mutex
	lastCheckErr   = map[string]string{}

	vpsSubRevMu          sync.Mutex
	vpsSubRevisions      = map[string]uint64{}
	lastObservedSubState = map[string]vpsSubConfigSig{}
)

var errStaleSubscriptionRevision = errors.New("VPS 订阅配置已在巡检期间更新(代次已失效)")

type vpsSubConfigSig struct {
	PlanCode           string
	RawSubsidiary      string
	AutoOrderAccountID string
	Datacenters        []string
	MonitorLinux       bool
	MonitorWindows     bool
	OS                 string
}

type vpsSubBaseline struct {
	Revision           uint64
	ID                 string
	PlanCode           string
	RawSubsidiary      string
	AutoOrderAccountID string
	Datacenters        []string
	MonitorLinux       bool
	MonitorWindows     bool
	OS                 string
	ExpectedStatus     map[string]string
	Stale              bool
}

func makeSubConfigSig(s types.VPSSubscription) vpsSubConfigSig {
	dcs := make([]string, len(s.Datacenters))
	copy(dcs, s.Datacenters)
	return vpsSubConfigSig{
		PlanCode:           strings.TrimSpace(s.PlanCode),
		RawSubsidiary:      NormalizeSubsidiary(s.OvhSubsidiary),
		AutoOrderAccountID: strings.TrimSpace(s.AutoOrderAccountID),
		Datacenters:        dcs,
		MonitorLinux:       s.MonitorLinux,
		MonitorWindows:     s.MonitorWindows,
		OS:                 strings.TrimSpace(s.OS),
	}
}

func subConfigSigEqual(a, b vpsSubConfigSig) bool {
	if a.PlanCode != b.PlanCode ||
		a.RawSubsidiary != b.RawSubsidiary ||
		(a.RawSubsidiary == "" && a.AutoOrderAccountID != b.AutoOrderAccountID) ||
		a.MonitorLinux != b.MonitorLinux ||
		a.MonitorWindows != b.MonitorWindows ||
		a.OS != b.OS {
		return false
	}
	return stringSliceEqual(a.Datacenters, b.Datacenters)
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stringMapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// BumpSubscriptionRevision explicitly increments the generation counter for a VPS subscription,
// invalidating any in-flight background check round for that subscription.
func BumpSubscriptionRevision(subID string) uint64 {
	vpsSubRevMu.Lock()
	defer vpsSubRevMu.Unlock()
	vpsSubRevisions[subID]++
	return vpsSubRevisions[subID]
}

// SubscriptionRevision returns the current generation counter for a VPS subscription.
func SubscriptionRevision(subID string) uint64 {
	vpsSubRevMu.Lock()
	defer vpsSubRevMu.Unlock()
	return vpsSubRevisions[subID]
}

func captureSubBaselinesLocked(subs []types.VPSSubscription) map[string]*vpsSubBaseline {
	vpsSubRevMu.Lock()
	defer vpsSubRevMu.Unlock()
	out := make(map[string]*vpsSubBaseline, len(subs))
	for _, s := range subs {
		sig := makeSubConfigSig(s)
		if prev, ok := lastObservedSubState[s.ID]; ok && !subConfigSigEqual(prev, sig) {
			vpsSubRevisions[s.ID]++
		}
		lastObservedSubState[s.ID] = sig
		rev := vpsSubRevisions[s.ID]
		dcs := make([]string, len(s.Datacenters))
		copy(dcs, s.Datacenters)
		out[s.ID] = &vpsSubBaseline{
			Revision:           rev,
			ID:                 s.ID,
			PlanCode:           sig.PlanCode,
			RawSubsidiary:      sig.RawSubsidiary,
			AutoOrderAccountID: sig.AutoOrderAccountID,
			Datacenters:        dcs,
			MonitorLinux:       sig.MonitorLinux,
			MonitorWindows:     sig.MonitorWindows,
			OS:                 sig.OS,
			ExpectedStatus:     copyStringMap(s.LastStatus),
		}
	}
	return out
}

func baselineMatchesLiveLocked(live *types.VPSSubscription, base *vpsSubBaseline) bool {
	if live == nil || base == nil {
		return false
	}
	vpsSubRevMu.Lock()
	curRev := vpsSubRevisions[base.ID]
	vpsSubRevMu.Unlock()
	if curRev != base.Revision {
		return false
	}
	sig := makeSubConfigSig(*live)
	baseSig := vpsSubConfigSig{
		PlanCode:           base.PlanCode,
		RawSubsidiary:      base.RawSubsidiary,
		AutoOrderAccountID: base.AutoOrderAccountID,
		Datacenters:        base.Datacenters,
		MonitorLinux:       base.MonitorLinux,
		MonitorWindows:     base.MonitorWindows,
		OS:                 base.OS,
	}
	if !subConfigSigEqual(sig, baseSig) {
		return false
	}
	return stringMapEqual(live.LastStatus, base.ExpectedStatus)
}

func markBaselineStaleLocked(live *types.VPSSubscription, base *vpsSubBaseline) {
	if base == nil {
		return
	}
	base.Stale = true
	vpsSubRevMu.Lock()
	if vpsSubRevisions[base.ID] == base.Revision {
		vpsSubRevisions[base.ID]++
	}
	if live != nil {
		lastObservedSubState[base.ID] = makeSubConfigSig(*live)
	} else {
		delete(lastObservedSubState, base.ID)
	}
	vpsSubRevMu.Unlock()
}

func isBaselineCurrent(state *app.State, base *vpsSubBaseline) bool {
	if base == nil {
		return true
	}
	if base.Stale {
		return false
	}
	state.VPSSubsMu.Lock()
	defer state.VPSSubsMu.Unlock()
	for i := range state.VPSSubscriptions {
		if state.VPSSubscriptions[i].ID == base.ID {
			if baselineMatchesLiveLocked(&state.VPSSubscriptions[i], base) {
				return true
			}
			markBaselineStaleLocked(&state.VPSSubscriptions[i], base)
			return false
		}
	}
	markBaselineStaleLocked(nil, base)
	return false
}

// logCheckFailure 记录一次可用性检查失败,并做同因去重。
func logCheckFailure(state *app.State, subID, planCode, subsidiary string, err error) {
	msg := fmt.Sprintf("VPS 可用性检查失败 [%s @ %s]: %s", planCode, subsidiary, err.Error())
	lastCheckErrMu.Lock()
	same := lastCheckErr[subID] == msg
	lastCheckErr[subID] = msg
	lastCheckErrMu.Unlock()
	if same {
		return
	}
	var ce *CheckError
	if errors.As(err, &ce) && ce.Permanent() {
		// 配置问题:不会自愈,提到 Error 让用户在日志里一眼看到
		state.Logger.Error(msg+" (该订阅将持续失败,请修正子公司或套餐后重新订阅)", "vps_monitor")
		return
	}
	state.Logger.Warn(msg, "vps_monitor")
}

// clearCheckFailure 恢复正常后清掉去重记录,下次再失败仍会记一条。
func clearCheckFailure(subID string) {
	lastCheckErrMu.Lock()
	delete(lastCheckErr, subID)
	lastCheckErrMu.Unlock()
}

// checkNotifyOrStop 节流后体检所有通知通道。
// 返回 true=继续 loop。
func checkNotifyOrStop(state *app.State) bool {
	if state == nil || state.Config == nil {
		return true
	}
	tgCheckMu.Lock()
	due := time.Since(lastTGCheck) >= tgRecheckInterval
	tgCheckMu.Unlock()
	if !due {
		return true
	}
	// 与服务器监控同一口径:通知不可用时仅告警,不杀停监控循环
	ok, reason := notify.AnyAvailable(state, false) // 周期性只查配置,不发真消息(issue #2)
	tgCheckMu.Lock()
	lastTGCheck = time.Now()
	tgCheckMu.Unlock()
	if !ok {
		state.Logger.Warn("当前未配置可用通知通道("+reason+")，VPS 通知推送已静默；VPS 监控保持运行。", "vps_monitor")
		return true
	}
	return true
}

// MonitorLoop 兼容入口:按当前代际号跑。
func MonitorLoop(state *app.State) {
	runningMu.Lock()
	gen := generation
	runningMu.Unlock()
	monitorLoopGen(state, gen)
}

// stillMine 这个循环是不是还该继续跑:running 为真,且代际号还是自己那一代
func stillMine(gen int64) bool {
	runningMu.Lock()
	defer runningMu.Unlock()
	return running && generation == gen
}

func monitorLoopGen(state *app.State, gen int64) {
	state.Logger.Info("VPS监控循环已启动", "vps_monitor")
	for {
		isRunning := stillMine(gen)
		if !isRunning {
			break
		}

		// TG 失效 → 自停。checkNotifyOrStop 内部 5min 节流。
		if !checkNotifyOrStop(state) {
			break
		}

		state.VPSSubsMu.Lock()
		subs := cloneSubs(state.VPSSubscriptions)
		baselines := captureSubBaselinesLocked(subs)
		interval := state.VPSCheckInterval
		state.VPSSubsMu.Unlock()

		if len(subs) > 0 {
			state.Logger.Info(fmt.Sprintf("开始检查 %d 个VPS订阅...", len(subs)), "vps_monitor")
			prefetched := prefetchAggregated(state, subs)
			for idx := range subs {
				isRunning = stillMine(gen)
				if !isRunning {
					break
				}
				sub := &subs[idx]
				base := baselines[sub.ID]
				if !isBaselineCurrent(state, base) {
					state.Logger.Info(fmt.Sprintf("VPS 订阅 %s (%s) 在巡检期间已变更或删除(代次已推进),丢弃旧代次巡检任务", sub.ID, sub.PlanCode), "vps_monitor")
					continue
				}
				// 老订阅可能没存子公司(或存了小写)。跟着自动下单账户所在站点走,
				// 不再一律回落 IE —— 美区账户拿 IE 查的是欧洲库存,永远等不到自己能买的货。
				ovhSub := NormalizeSubsidiary(sub.OvhSubsidiary)
				if ovhSub == "" {
					ovhSub = DefaultSubsidiary(state, sub.AutoOrderAccountID)
				}
				currentData, fromAggregate := prefetched[aggregatedKey{ovhSub, sub.AutoOrderAccountID, strings.TrimSpace(sub.PlanCode)}]
				if !fromAggregate {
					var err error
					currentData, err = checkVPSDCAvailabilityOS(state, sub.PlanCode, ovhSub, sub.OS, sub.AutoOrderAccountID)
					if err != nil {
						logCheckFailure(state, sub.ID, sub.PlanCode, ovhSub, err)
						continue
					}
				}
				if !isBaselineCurrent(state, base) {
					state.Logger.Info(fmt.Sprintf("VPS 订阅 %s (%s) 在等待库存响应期间已变更(代次已推进),丢弃过时巡检结果", sub.ID, sub.PlanCode), "vps_monitor")
					continue
				}
				clearCheckFailure(sub.ID)
				dcsRaw, _ := currentData["datacenters"].([]interface{})
				if sub.LastStatus == nil {
					sub.LastStatus = map[string]string{}
				}
				lastStatus := sub.LastStatus
				monitoredDCs := sub.Datacenters

				initialAvailable := []map[string]interface{}{}
				newAvailable := []map[string]interface{}{}
				newUnavailable := []map[string]interface{}{}
				isFirstCheckOverall := len(lastStatus) == 0
				// 勾了机房却一个都没匹配上 = 这条订阅永远不会响。
				// 不报出来的话它和"这些机房目前无货"表现完全一样:
				// 界面正常、日志干净、就是永远不动。
				matchedDC := false

				for _, dcRaw := range dcsRaw {
					dc, ok := dcRaw.(map[string]interface{})
					if !ok {
						continue
					}
					code, _ := dc["code"].(string)
					name, _ := dc["datacenter"].(string)
					// 按订阅勾选的系统取对应字段。schema 的 vps.order.rule.Datacenter
					// 有 status / linuxStatus / windowsStatus 三个,三区都有 ——
					// 以前只读 status,导致界面上的「监控系统」勾选框完全是摆设:
					// 只勾 Windows 的用户在 Linux 有货时照样收到补货通知,
					// 开了自动下单还会白跑一整条建车链路,到 checkout 才失败。
					currentStatus := pickOSStatus(dc, sub.MonitorLinux, sub.MonitorWindows)
					daysI64, _ := numconv.ToInt64(dc["daysBeforeDelivery"])
					days := int(daysI64)

					if len(monitoredDCs) > 0 {
						found := false
						for _, m := range monitoredDCs {
							// 统一归一比对可用性接口返回的 code(如 eu-west-lim / ap-south-mum)与
							// datacenter 短码(如 DE / UK / YNM),解决批量订阅与目录机房订阅失配漏检。
							if MatchVPSDatacenter(m, code, name) {
								found = true
								break
							}
						}
						if !found {
							continue
						}
					}
					matchedDC = true
					stateKey := code
					if stateKey == "" {
						stateKey = name
					}
					oldStatus, hasOld := lastStatus[stateKey]
					if !hasOld && code != "" && name != "" {
						if alt, ok := lastStatus[name]; ok {
							oldStatus, hasOld = alt, true
							delete(lastStatus, name)
						}
					}
					if !hasOld {
						item := map[string]interface{}{
							"name":   name,
							"code":   code,
							"status": currentStatus,
							"days":   days,
						}
						initialAvailable = append(initialAvailable, item)
						if vpsStockAvailable(currentStatus) {
							if !isFirstCheckOverall {
								newAvailable = append(newAvailable, item)
							}
							sub.History = append(sub.History, map[string]interface{}{
								"timestamp":      time.Now().Format(time.RFC3339Nano),
								"datacenter":     name,
								"datacenterCode": code,
								"status":         currentStatus,
								"changeType":     "available",
								"oldStatus":      nil,
							})
						}
					} else {
						// 白名单判据,和独服的 ovh.IsAvailableForOrder 同口径。
						// 以前用排除法列举 out-of-stock*,OVH 若新增一个"买不了"的取值
						// (独服那边就有 comingSoon),会被判成有货 → 误报 + 误下单。
						wasUnavail := !vpsStockAvailable(oldStatus)
						isUnavail := !vpsStockAvailable(currentStatus)
						if wasUnavail && !isUnavail {
							newAvailable = append(newAvailable, map[string]interface{}{
								"name":   name,
								"code":   code,
								"status": currentStatus,
								"days":   days,
							})
							sub.History = append(sub.History, map[string]interface{}{
								"timestamp":      time.Now().Format(time.RFC3339Nano),
								"datacenter":     name,
								"datacenterCode": code,
								"status":         currentStatus,
								"changeType":     "available",
								"oldStatus":      oldStatus,
							})
						} else if !wasUnavail && isUnavail {
							newUnavailable = append(newUnavailable, map[string]interface{}{
								"name":   name,
								"code":   code,
								"status": currentStatus,
								"days":   days,
							})
							sub.History = append(sub.History, map[string]interface{}{
								"timestamp":      time.Now().Format(time.RFC3339Nano),
								"datacenter":     name,
								"datacenterCode": code,
								"status":         currentStatus,
								"changeType":     "unavailable",
								"oldStatus":      oldStatus,
							})
						}
					}
					lastStatus[stateKey] = currentStatus
				}

				if len(monitoredDCs) > 0 && !matchedDC {
					// 把 OVH 这次真正返回的机房码一并写出来,用户照着改就行。
					// 最常见的原因是机房码打错了或大小写/前缀不对
					// (比如把 US-EAST-VA 写成 va)。
					avail := make([]string, 0, len(dcsRaw))
					for _, dcRaw := range dcsRaw {
						if dc, ok := dcRaw.(map[string]interface{}); ok {
							c, _ := dc["code"].(string)
							n, _ := dc["datacenter"].(string)
							if n != "" && c != "" && !strings.EqualFold(n, c) {
								avail = append(avail, fmt.Sprintf("%s(%s)", n, c))
							} else if c != "" {
								avail = append(avail, c)
							} else if n != "" {
								avail = append(avail, n)
							}
						}
					}
					state.Logger.Warn(fmt.Sprintf(
						"VPS 订阅 %s 指定的机房 %v 一个都不在 OVH 返回的列表里,这条订阅不会有任何通知。OVH 当前返回:%v",
						sub.PlanCode, monitoredDCs, avail), "vps_monitor")
				}

				if !isBaselineCurrent(state, base) {
					state.Logger.Info(fmt.Sprintf("VPS 订阅 %s (%s) 在通知/下单前已变更(代次已推进),丢弃过时巡检结果", sub.ID, sub.PlanCode), "vps_monitor")
					continue
				}

				if isFirstCheckOverall && len(initialAvailable) > 0 && sub.NotifyAvailable {
					state.Logger.Info(fmt.Sprintf("VPS %s 初始状态检查完成，%d个数据中心", sub.PlanCode, len(initialAvailable)), "vps_monitor")
					SendSummaryNotification(state, sub.PlanCode, ovhSub, initialAvailable, "initial")
				} else {
					if len(newAvailable) > 0 && sub.NotifyAvailable {
						state.Logger.Info(fmt.Sprintf("VPS %s 补货：%d个数据中心", sub.PlanCode, len(newAvailable)), "vps_monitor")
						SendSummaryNotification(state, sub.PlanCode, ovhSub, newAvailable, "available")
					}
					// 补货了就下单。放在通知之后:先让用户知道有货,下单本身要走五六次 OVH 往返。
					// 只认"无货→有货"这一种跳变(首次检查的存量不算,见上面的 isFirstCheckOverall)。
					// 下单前先把本轮库存观察落库,否则下单后崩溃重启会把同一次补货再判一次跳变。
					if len(newAvailable) > 0 && sub.AutoOrder && strings.TrimSpace(sub.AutoOrderAccountID) != "" && stillMine(gen) {
						sub.LastStatus = lastStatus
						if err := saveObservationWithBaseline(state, sub, base); err != nil {
							state.Logger.Error("保存 VPS 库存状态失败,跳过本次自动下单: "+err.Error(), "vps_monitor")
						} else {
							toOrder := *sub
							toOrder.OvhSubsidiary = ovhSub
							autoOrderOnRestock(state, toOrder, newAvailable)
						}
					}
					if len(newUnavailable) > 0 && sub.NotifyUnavailable {
						state.Logger.Info(fmt.Sprintf("VPS %s 下架：%d个数据中心", sub.PlanCode, len(newUnavailable)), "vps_monitor")
						SendSummaryNotification(state, sub.PlanCode, ovhSub, newUnavailable, "unavailable")
					}
				}

				sub.LastStatus = lastStatus
				if len(sub.History) > 100 {
					sub.History = sub.History[len(sub.History)-100:]
				}
				// 逐个查询之间留 1 秒,避免连发打进限流;聚合结果已在手,不必等
				if !fromAggregate {
					time.Sleep(time.Second)
				}
			}
			// 按 ID + 代次基线合并写回：仅写回巡检期间未被修改/重置的当前代次订阅，防止后台旧轮次覆盖新子公司/新配置
			state.VPSSubsMu.Lock()
			byID := map[string]*types.VPSSubscription{}
			for i := range subs {
				byID[subs[i].ID] = &subs[i]
			}
			for i := range state.VPSSubscriptions {
				id := state.VPSSubscriptions[i].ID
				updated, ok := byID[id]
				if !ok {
					continue
				}
				base := baselines[id]
				if base != nil && (base.Stale || !baselineMatchesLiveLocked(&state.VPSSubscriptions[i], base)) {
					markBaselineStaleLocked(&state.VPSSubscriptions[i], base)
					continue
				}
				state.VPSSubscriptions[i].LastStatus = updated.LastStatus
				state.VPSSubscriptions[i].History = updated.History
			}
			state.VPSSubsMu.Unlock()
			_ = SaveSubscriptions(state)
		} else {
			state.Logger.Info("当前无VPS订阅，跳过检查", "vps_monitor")
		}

		isRunning = stillMine(gen)
		if isRunning {
			state.Logger.Info(fmt.Sprintf("等待 %d 秒后进行下次VPS检查...", interval), "vps_monitor")
			for i := 0; i < interval; i++ {
				isRunning = stillMine(gen)
				if !isRunning {
					break
				}
				time.Sleep(time.Second)
			}
		}
	}
	state.Logger.Info("VPS监控循环已停止", "vps_monitor")
}

// Start 启动监控
func Start(state *app.State) bool {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	runningMu.Lock()
	if running {
		runningMu.Unlock()
		return false
	}
	prevWg := loopWg
	runningMu.Unlock()

	// 等上一代循环退出:必须在 runningMu 锁外等待，
	// 否则旧循环退出前调用 stillMine 会尝试获取 runningMu，导致死锁
	if prevWg != nil {
		prevWg.Wait()
	}

	runningMu.Lock()
	if running {
		runningMu.Unlock()
		return false
	}
	running = true
	generation++
	gen := generation
	wg := &sync.WaitGroup{}
	wg.Add(1)
	loopWg = wg
	runningMu.Unlock()

	// 重置 TG 检查时间戳,保证启动后第一轮一定 verify
	tgCheckMu.Lock()
	lastTGCheck = time.Time{}
	tgCheckMu.Unlock()
	go func() {
		defer wg.Done()
		monitorLoopGen(state, gen)
	}()
	if state != nil && state.Logger != nil {
		state.Logger.Info(fmt.Sprintf("VPS监控已启动 (检查间隔: %d秒)", state.VPSCheckInterval), "vps_monitor")
	}
	return true
}

// Stop 停止监控
func Stop(state *app.State) bool {
	runningMu.Lock()
	if !running {
		runningMu.Unlock()
		return false
	}
	running = false
	runningMu.Unlock()
	if state != nil {
		state.Logger.Info("正在停止VPS监控...", "vps_monitor")
	}
	return true
}

// WaitStopped 等最近一代监控循环退出(最多 timeout),关库前调用。返回 false = 超时。
func WaitStopped(timeout time.Duration) bool {
	runningMu.Lock()
	wg := loopWg
	runningMu.Unlock()
	if wg == nil {
		return true
	}
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

// Running 返回是否在运行
func Running() bool {
	runningMu.Lock()
	defer runningMu.Unlock()
	return running
}
