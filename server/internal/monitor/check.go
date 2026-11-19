package monitor

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/catalog"
)

// notification 单次状态变化通知（内部）
type notification struct {
	// notify 这条跳变要不要发通知。与"要不要下单"分开:
	// 下单只看跳变本身,通知才受 NotifyAvailable/NotifyUnavailable 控制。
	notify bool
	dc     string
	status string
	// rawStatus OVH 原样返回的可用性值(1H-low / 24H / 72H …)。
	// status 是归一化后的 available/unavailable/price_check_failed,
	// 丢掉原值就没法在通知里告诉用户"多久能交付、库存高还是低" ——
	// 而这正是决定要不要立刻下单的信息。
	rawStatus        string
	oldStatus        string
	hasOld           bool
	statusKey        string
	changeType       string
	priceCheckFailed bool
	priceCheckError  string
	configTraceID    string
	traceID          string
	detectedTime     string
	durationText     string
}

func (n notification) oldStatusJSON() interface{} {
	if !n.hasOld {
		return nil
	}
	return n.oldStatus
}

// 并发:同一条订阅同一时刻只会有一个 goroutine 在跑这里(loop 每轮一个,轮间 wg.Wait),
// 但 HTTP 侧随时可能读同一条订阅。所以本函数对 sub 的所有读写都走 types.go 里的带锁方法,
// 中间态(lastStatus)先在本地副本上算,算完一次性写回。
func (m *Monitor) checkAccountAvailabilityChange(sub *Subscription, traceID string) {
	planCode := sub.PlanCode // 创建后不变,可以裸读

	// 订阅配置在本轮开始时取一份快照,后面统一用它 ——
	// 边跑边读 sub.X 既有竞争,也会让一轮检查横跨用户改配置的时刻。
	cfg := sub.checkConfig()
	if cfg.Paused || !m.checkCurrent(sub, cfg) {
		return
	}

	// 查询账户必须按 planCode 所属大区选,不能一律落默认账户:
	// 三个站点的库存视图独立,查错站点是 HTTP 200 + 空数组而不是报错(见 resolveQueryAccount 上的实测)。
	choice := m.resolveQueryAccount(planCode, cfg.AutoOrderAccountID)

	// beginCheck 写入本轮诊断信息并返回上一轮的错误原因,用来给日志去重:
	// 检查每 5 秒跑一轮,同一条区域错配如果每轮都 Warn 一次,一天能刷出一万七千条一模一样的日志。
	prevErr := sub.beginCheck(m.nowBeijing().Format(time.RFC3339Nano),
		choice.accountID, choice.region, choice.subsidiary)

	if choice.degradeReason != "" && choice.degradeReason != prevErr {
		m.state.Logger.Warn(fmt.Sprintf("[monitor/region] 订阅 %s: %s", planCode, choice.degradeReason), "monitor")
	}
	if choice.degradeReason != "" {
		// 选账户这一步就出了问题(账户被删 / 判不出归属),不管后面查不查得到数据,
		// 都必须让用户在 region_issues 里看见 —— 以前只在"查出来是空"时才写进 LastCheckError,
		// 于是"订阅指定的下单账户已被删除、现在拿默认账户在查"这种事,
		// 只要默认账户碰巧有数据就完全无声无息。
		sub.setCheckError(choice.degradeReason)
	}
	if choice.definitive && choice.accountID == "" {
		// 可用性接口已经逐区确认过:已配置账户覆盖的大区里根本没有这个机型的记录。
		// 再打 OVH 也只会拿到空数组,每 5 秒白跑一次。原因已写进订阅状态(前端 region_issues 可见),
		// 不是静默跳过;而且这个结论 10 分钟后会自动重探,机型上架后能自愈。
		return
	}
	// 监控用选中账户的 subsidiary 拉 catalog,这样跨子公司 multi-account
	// 触发 auto-order 时,options 匹配能命中目标账户独有的项。
	currentAvailability := catalog.CheckServerAvailabilityWithConfigs(m.state, planCode, choice.accountID)
	if !m.checkCurrent(sub, cfg) {
		return
	}
	if len(currentAvailability) == 0 {
		// 区分两种"空":选到了区域正确的账户还是空 → OVH 侧问题;
		// 用的账户其实在别的大区 → 区域配错,必须说清楚,不能只丢一句"无法获取"。
		reason := choice.degradeReason
		if reason == "" {
			reason = m.explainEmptyAvailability(planCode, choice)
		}
		sub.setCheckError(reason)
		if reason != prevErr {
			m.state.Logger.Warn(fmt.Sprintf("无法获取 %s 的可用性信息: %s", planCode, reason), "monitor")
		}
		return
	}

	// 库存可能在开售前就显示有货,而销售目录还没收录这个 Eco 型号(上游 issue #12)。
	// 给订阅留一条可见诊断;这类配置只有购物车验价通过才会通知/下单(见下方 CatalogMissing 分支)。
	// 必须在"已恢复"之前判断,否则每轮"设诊断→清诊断"来回刷日志。
	catalogWarning := ""
	for _, configData := range currentAvailability {
		if configData.AwaitingCatalog(planCode) {
			catalogWarning = fmt.Sprintf("当前账户的销售目录尚未收录机型 %s；库存可用性不代表已开售,"+
				"需购物车验价通过后才会发送有货通知或自动下单。请确认账户地区及产品系列", planCode)
			break
		}
	}
	// 走到这里说明本轮真正拿到了库存 —— 现在才允许说"已恢复"(issue #2:
	// 以前在查询前就报恢复,持续失败的型号每轮"恢复→又失败"刷屏)。
	if catalogWarning != "" {
		sub.setCheckError(catalogWarning)
		if catalogWarning != prevErr {
			m.state.Logger.Warn(catalogWarning, "monitor")
		}
	} else if prevErr != "" {
		sub.clearCheckError()
		m.state.Logger.Info(fmt.Sprintf("[monitor/region] 订阅 %s 的区域问题已恢复,改用 %s 站点(子公司 %s)查询",
			planCode, choice.region, choice.subsidiary), "monitor")
	}

	hasCatalogMissing := false
	allConfigsInCatalog := len(currentAvailability) > 0
	for _, cd := range currentAvailability {
		if cd.CatalogMissing {
			hasCatalogMissing = true
			allConfigsInCatalog = false
		}
	}
	m.observePlanCatalogStatus(choice.accountID, choice.subsidiary, planCode, allConfigsInCatalog, hasCatalogMissing)

	// 状态先在副本上推进,循环结束一次性写回:HTTP 侧读到的要么是上轮的完整状态、
	// 要么是本轮的完整状态,不会是推进到一半的中间态。
	lastStatus := sub.statusSnapshot()
	before := copyStatus(lastStatus)
	// committed 已经据此下单 / 发通知的基线。本轮因配置变更、暂停、停止而提前返回时,
	// 整轮的 lastStatus 不写回(旧配置的半成品不能生效),但这部分必须写回,
	// 否则下一轮把同一次补货再判一次"无货→有货":重复通知,再下一单。
	committed := map[string]string{}
	bail := func() {
		if len(committed) > 0 {
			sub.commitStatus(committed)
			m.dirty.Store(true)
		}
	}
	monitoredDCs := cfg.Datacenters

	m.state.Logger.Info(fmt.Sprintf("订阅 %s - 监控数据中心: %v", planCode, monitoredDCs), "monitor")
	m.state.Logger.Info(fmt.Sprintf("订阅 %s - 当前发现 %d 个配置组合", planCode, len(currentAvailability)), "monitor")

	for configKey, configData := range currentAvailability {
		if !m.checkCurrent(sub, cfg) {
			bail()
			return
		}
		// 订阅指定了配置就只盯那一套。
		//
		// 这个循环下面的每一件事都是**按配置逐套**做的:发一条通知、触发一次
		// 自动下单。所以不筛的话,"自动抢 1 台"在三套配置同时补货时会下三次单
		// (还要再乘以机房数)。用户想要的往往是"只盯 64G + 2x480SSD 那套",
		// 在有这个字段之前他没有任何办法表达。
		// 空 = 全部配置,保持一直以来的行为。
		matchedOptions := configData.Options
		if configData.CatalogMissing {
			matchedOptions = catalog.CustomInventoryOptions(planCode, configData.Options, cfg.Options)
		}
		if !configMatchesFilter(cfg.Options, matchedOptions) {
			m.state.Logger.Debug(fmt.Sprintf("订阅 %s 跳过配置 %s(与订阅指定的配置不符)",
				planCode, configKey), "monitor")
			continue
		}
		effectiveOptions := monitoringOptions(matchedOptions, cfg.Options)
		memory := configData.Memory
		storage := configData.Storage
		configDisplay := memory + " + " + storage
		if configData.SystemStorage != "" && configData.SystemStorage != "N/A" {
			configDisplay += " + " + configData.SystemStorage
		}
		if configData.GPU != "" && configData.GPU != "N/A" {
			configDisplay += " + " + configData.GPU
		}

		configTraceID := uuid.NewString()
		m.state.Logger.Info(fmt.Sprintf("检查配置: %s [config-trace:%s]", configDisplay, configTraceID), "monitor")

		configInfo := map[string]interface{}{
			"memory":  memory,
			"storage": storage,
			"display": configDisplay,
			"options": effectiveOptions,
		}
		if configData.SystemStorage != "" && configData.SystemStorage != "N/A" {
			configInfo["system_storage"] = configData.SystemStorage
		}
		if configData.GPU != "" && configData.GPU != "N/A" {
			configInfo["gpu"] = configData.GPU
		}
		// 询价必须走上面选中的那个账户:验价账户和下单账户不一致时,
		// endpoint 与 ovhSubsidiary 都会错(A 账户买得到 B 账户买不到,反之亦然)。
		// 订阅指定了 auto_order 账户时 choice.accountID 就是它,两者天然一致;
		// 没指定时用的是区域正确的那个账户 —— 以前这里传空串会落到默认账户,
		// 美区机型就会拿欧区购物车去验价,币种和可售性全错。
		// 账户 ID 只加在传给询价/通知的副本上,不污染写进订阅历史的 configInfo。
		if sub.checkParent != nil {
			configInfo["account_id"] = choice.accountID
		}
		// 购买时长随配置走:验价、预热车命中、入队都按订阅选的周期
		if cfg.Duration != "" {
			configInfo["duration"] = cfg.Duration
		}
		priceCfg := copyMap(configInfo)
		priceCfg["account_id"] = choice.accountID
		priceCfg["subsidiary"] = choice.subsidiary
		priceCfg["catalog_missing"] = configData.CatalogMissing
		// 验价车留作预热车时要知道下一步谁来用它:自动下单(不带优惠码)还是 TG 一键下单按钮(带默认优惠码)
		priceCfg["auto_order"] = cfg.AutoOrder

		type dcStatus struct {
			status    string
			statusKey string
			oldStatus string
			hasOld    bool
			orderable bool
		}
		dcStatusMap := map[string]dcStatus{}
		priceCheckTasks := []string{}
		priceCheckSkipped := map[string]bool{}
		for dc, status := range configData.Datacenters {
			if len(monitoredDCs) > 0 && !matchesMonitoredDC(monitoredDCs, dc) {
				continue
			}
			statusKey := dc + "|" + configKey
			old, hasOld := lastStatus[statusKey]
			// 白名单判定:只有 \d+H 家族算有货。以前 `!= "unavailable"` 把 comingSoon
			// (即将上线、尚未开售)和 unknown 都当成有货,会对永远下不了单的机型
			// 反复验价、反复发"有货但价格校验失败"的骚扰通知。
			orderable := catalog.IsAvailableForOrder(status)
			dcStatusMap[dc] = dcStatus{status: status, statusKey: statusKey, oldStatus: old, hasOld: hasOld, orderable: orderable}
			if NeedsPriceCheck(orderable, hasOld, old) {
				if !m.hasFreshPrewarmedCartForConfig(planCode, dc, priceCfg) &&
					m.isPlanInCooldown(choice.accountID, choice.subsidiary, planCode) {
					priceCheckSkipped[dc] = true
					continue
				}
				priceCheckTasks = append(priceCheckTasks, dc)
			}
		}
		sort.Strings(priceCheckTasks)
		if len(priceCheckSkipped) > 0 {
			m.logPlanCooldownSkip(choice.accountID, choice.subsidiary, planCode, "监控验价")
		}

		// 新出现的有货:先于验价 / 通知,立刻唤醒同型号、同机房的队列任务,
		// 不让它们等满各自的重试间隔(默认 60 秒)。任务醒来会自己再查一次库存,不会误下单。
		queryEndpoint := ""
		if acc, ok := m.state.FindAccount(choice.accountID); ok {
			queryEndpoint = acc.Endpoint
		}
		for dc, ds := range dcStatusMap {
			if NeedsPriceCheck(ds.orderable, ds.hasOld, ds.oldStatus) {
				if n := m.state.WakeQueueForStock(planCode, dc, queryEndpoint); n > 0 {
					m.state.Logger.Info(fmt.Sprintf("[monitor->queue] %s@%s 有货,已唤醒 %d 个同型号队列任务", planCode, dc, n), "monitor")
				}
			}
		}

		// 并发价格校验
		priceCheckResults := map[string][3]interface{}{}
		if len(priceCheckTasks) > 0 {
			var pcMu sync.Mutex
			var roundHit429 bool
			runPriceVerify := func(dc string) bool {
				ok, errMsg, transient := m.verifyPriceWithTransient(planCode, dc, priceCfg)
				pcMu.Lock()
				priceCheckResults[dc] = [3]interface{}{ok, errMsg, transient}
				if !ok && isRateLimit429Error(errMsg) {
					roundHit429 = true
				}
				pcMu.Unlock()
				if ok {
					m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 价格校验通过 [config-trace:%s]",
						planCode, dc, configDisplay, configTraceID), "monitor")
				} else {
					kind, friendly := ClassifyPriceCheckError(errMsg)
					if kind == ErrorKindPreSale {
						m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 底层库存已释出，等待商城开放加车（%s） [config-trace:%s]",
							planCode, dc, configDisplay, friendly, configTraceID), "monitor")
					} else if kind == ErrorKindRegionBlocked {
						m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 跨区配置受限: %s [config-trace:%s]",
							planCode, dc, configDisplay, errMsg, configTraceID), "monitor")
					} else {
						m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 价格校验未通过，原因: %s [config-trace:%s]",
							planCode, dc, configDisplay, errMsg, configTraceID), "monitor")
					}
				}
				return ok
			}

			// 命中新鲜预热车的 DC 直接放行；需要真实建车探针的 DC 对目录未收录车型先单点试探 1 个机房，
			// 首探未通过则跳过其余机房，避免同轮内 N 配置 × M 机房齐发触发 429。
			prewarmedTasks := make([]string, 0, len(priceCheckTasks))
			liveTasks := make([]string, 0, len(priceCheckTasks))
			for _, dc := range priceCheckTasks {
				if m.hasFreshPrewarmedCartForConfig(planCode, dc, priceCfg) {
					prewarmedTasks = append(prewarmedTasks, dc)
				} else {
					liveTasks = append(liveTasks, dc)
				}
			}

			concurrentTasks := make([]string, 0, len(priceCheckTasks))
			concurrentTasks = append(concurrentTasks, prewarmedTasks...)
			if len(liveTasks) > 0 && (configData.CatalogMissing || m.hasPendingUpstream504(choice.accountID, choice.subsidiary, planCode)) {
				if runPriceVerify(liveTasks[0]) {
					concurrentTasks = append(concurrentTasks, liveTasks[1:]...)
				} else if configData.CatalogMissing || m.isPlanInCooldown(choice.accountID, choice.subsidiary, planCode) {
					for _, skippedDC := range liveTasks[1:] {
						priceCheckSkipped[skippedDC] = true
					}
				} else {
					concurrentTasks = append(concurrentTasks, liveTasks[1:]...)
				}
			} else {
				concurrentTasks = append(concurrentTasks, liveTasks...)
			}

			if len(concurrentTasks) > 0 {
				var wg sync.WaitGroup
				workers := len(concurrentTasks)
				if workers > 10 {
					workers = 10
				}
				sem := make(chan struct{}, workers)
				for _, dc := range concurrentTasks {
					wg.Add(1)
					sem <- struct{}{}
					go func(dc string) {
						defer wg.Done()
						defer func() { <-sem }()
						pcMu.Lock()
						abort429 := roundHit429
						pcMu.Unlock()
						if abort429 {
							pcMu.Lock()
							priceCheckSkipped[dc] = true
							pcMu.Unlock()
							return
						}
						if !m.hasFreshPrewarmedCartForConfig(planCode, dc, priceCfg) &&
							m.isPlanInCooldown(choice.accountID, choice.subsidiary, planCode) {
							pcMu.Lock()
							priceCheckSkipped[dc] = true
							pcMu.Unlock()
							m.logPlanCooldownSkip(choice.accountID, choice.subsidiary, planCode, "监控验价")
							return
						}
						runPriceVerify(dc)
					}(dc)
				}
				wg.Wait()
			}
		}

		notifications := []notification{}
		var priceUnverified []string
		unverifiedDCs := map[string]bool{}

		for dc, ds := range dcStatusMap {
			// 状态机(以及落进 lastStatus 的值)只认 available / unavailable /
			// price_check_failed 三种规范化状态,不可下单的原始值(unavailable /
			// unknown / comingSoon)统一折成 unavailable
			actualStatus := "unavailable"
			priceCheckFailed := false
			priceCheckError := ""

			if ds.orderable {
				if ds.hasOld && ds.oldStatus == "available" {
					// 持续在售：此前已在上架时完成验价放行，持续有货期间直接维持 available，
					// 不每轮重复调用购物车验价，防止在库存被买走下架瞬间因加车失败报 500 误告警
					actualStatus = "available"
				} else if v, ok := priceCheckResults[dc]; ok {
					okBool, _ := v[0].(bool)
					errStr, _ := v[1].(string)
					transient, _ := v[2].(bool)
					if !okBool && configData.CatalogMissing {
						// 销售目录尚未收录(常见于预上架:库存先于目录公布)。购物车也验不过 = 还不能买,
						// 按无货处理:不发"验价失败"提醒、不下单。以前记成 price_check_failed,
						// 库存每次抖动(无货→验价失败)都会再发一条,刷屏(上游 issue #12)。
						// 基准留在 unavailable,上架后购物车验价通过仍会触发一次正常补货与自动下单。
						actualStatus = "unavailable"
						m.state.Logger.Debug(fmt.Sprintf("%s@%s [%s] 销售目录未收录且购物车验价未通过,按无货处理: %s",
							planCode, dc, configDisplay, errStr), "monitor")
					} else if !okBool && transient {
						// 暂时性失败（超时、网络、408/429/5xx）不代表配置不可买：照常按有货放行，
						// 交由抢购任务在建车/结账阶段完整核对型号、配置、周期与数量，避免补货窗口丢单。
						actualStatus = "available"
						unverifiedDCs[dc] = true
						priceUnverified = append(priceUnverified, errStr)
						m.state.Logger.Warn(fmt.Sprintf("%s@%s [%s] 价格校验暂时未完成(%s)，按有货处理，下单前由抢购任务完整核对",
							planCode, dc, configDisplay, errStr), "monitor")
					} else if !okBool {
						actualStatus = "price_check_failed"
						priceCheckFailed = true
						priceCheckError = errStr
						kind, friendly := ClassifyPriceCheckError(errStr)
						if kind == ErrorKindPreSale {
							m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 可用性检测到库存，进入预售等待蹲守状态（%s）",
								planCode, dc, configDisplay, friendly), "monitor")
						} else if kind == ErrorKindRegionBlocked {
							m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 跨区配置受限: %s，标记为price_check_failed",
								planCode, dc, configDisplay, errStr), "monitor")
						} else {
							m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 可用性显示有货但价格校验未通过，原因: %s，标记为price_check_failed",
								planCode, dc, configDisplay, errStr), "monitor")
						}
					} else {
						actualStatus = "available"
						m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 可用性有货且价格校验通过，确认有货",
							planCode, dc, configDisplay), "monitor")
					}
				} else if priceCheckSkipped[dc] {
					if configData.CatalogMissing {
						actualStatus = "unavailable"
					} else if ds.hasOld && ds.oldStatus == "price_check_failed" {
						actualStatus = "price_check_failed"
					} else {
						actualStatus = "unavailable"
					}
				} else {
					actualStatus = "price_check_failed"
					priceCheckFailed = true
					priceCheckError = "价格校验未执行"
				}
			}

			// 状态跳变的判定必须和"要不要发通知"分开。
			//
			// 以前 9 个分支的 statusChanged 全被 if cfg.NotifyAvailable/NotifyUnavailable 包着,
			// 而下单链路是 statusChanged → notifications → availables → orderTargets → batchOrder。
			// 于是关掉「有货时提醒」就等于关掉自动下单 —— 而界面上这是三个独立复选框,
			// 用户为了不被 TG 刷屏关掉提醒,得到一个看着在跑、永远不下单的监控,没有任何线索。
			// (VPS 侧本来就是分开的两个 if,写法是对的,这里对齐它。)
			statusChanged := false
			changeType := ""

			switch {
			case !ds.hasOld:
				switch actualStatus {
				case "price_check_failed":
					m.state.Logger.Info(fmt.Sprintf("首次检查: %s@%s [%s] 可用性有货但价格校验失败",
						planCode, dc, configDisplay), "monitor")
					statusChanged, changeType = true, "price_check_failed"
				case "unavailable":
					m.state.Logger.Info(fmt.Sprintf("首次检查: %s@%s [%s] 无货", planCode, dc, configDisplay), "monitor")
					statusChanged, changeType = true, "unavailable"
				default:
					m.state.Logger.Info(fmt.Sprintf("首次检查: %s@%s [%s] 有货（价格校验通过）",
						planCode, dc, configDisplay), "monitor")
					statusChanged, changeType = true, "available"
				}
			case ds.oldStatus == "unavailable" && actualStatus == "available":
				statusChanged, changeType = true, "available"
				m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从无货变有货（价格校验通过）",
					planCode, dc, configDisplay), "monitor")
			case ds.oldStatus == "unavailable" && actualStatus == "price_check_failed":
				statusChanged, changeType = true, "price_check_failed"
				kind, _ := ClassifyPriceCheckError(priceCheckError)
				if kind == ErrorKindPreSale {
					m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 捕获预售信号：底层库存已释出，开始持续蹲守商城放行",
						planCode, dc, configDisplay), "monitor")
				} else {
					m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从无货变可用性有货但价格校验未通过",
						planCode, dc, configDisplay), "monitor")
				}
			case ds.oldStatus == "price_check_failed" && actualStatus == "available":
				statusChanged, changeType = true, "available"
				m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从价格校验失败变有货（价格校验通过）",
					planCode, dc, configDisplay), "monitor")
			case ds.oldStatus == "price_check_failed" && actualStatus == "unavailable":
				// 从验价失败恢复为无货：说明此前只是瞬时幽灵库存闪现，从未真正成功上架过，
				// 静默重置状态为 unavailable，不发送下架通知（避免“从未上架却收到下架通知”的骚扰）
				m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 幽灵库存闪现结束，从价格校验失败恢复为无货（静默）",
					planCode, dc, configDisplay), "monitor")
			case ds.oldStatus == "available" && actualStatus == "unavailable":
				statusChanged, changeType = true, "unavailable"
				m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从有货变无货", planCode, dc, configDisplay), "monitor")
			case ds.oldStatus == "available" && actualStatus == "price_check_failed":
				statusChanged, changeType = true, "price_check_failed"
				m.state.Logger.Info(fmt.Sprintf("%s@%s [%s] 从有货变可用性有货但价格校验失败",
					planCode, dc, configDisplay), "monitor")
			}

			// 通知开关只决定"发不发消息",不再影响跳变本身。
			// changeType 是 unavailable 的看 NotifyUnavailable,其余(available /
			// price_check_failed,都属于"有货了")看 NotifyAvailable。
			notifyThis := false
			if statusChanged {
				if changeType == "unavailable" {
					notifyThis = cfg.NotifyUnavailable
				} else {
					notifyThis = cfg.NotifyAvailable
				}
			}

			if statusChanged {
				detectedTime := m.nowBeijing().Format(time.RFC3339Nano)
				n := notification{
					dc:               dc,
					status:           actualStatus,
					rawStatus:        ds.status,
					oldStatus:        ds.oldStatus,
					hasOld:           ds.hasOld,
					statusKey:        ds.statusKey,
					changeType:       changeType,
					priceCheckFailed: priceCheckFailed,
					priceCheckError:  priceCheckError,
					configTraceID:    configTraceID,
					traceID:          traceID,
					detectedTime:     detectedTime,
					notify:           notifyThis,
				}
				if changeType == "available" && ds.oldStatus == "unavailable" {
					n.durationText = m.calcDuration(sub, dc, configDisplay, []string{"unavailable", "price_check_failed"})
				}
				notifications = append(notifications, n)
			}

			lastStatus[ds.statusKey] = actualStatus
		}

		// sideEffects 本配置是否已经下单或发出通知;commitConfig 把本配置各机房的新基线记为已生效
		sideEffects := false
		commitConfig := func() {
			for _, ds := range dcStatusMap {
				committed[ds.statusKey] = lastStatus[ds.statusKey]
			}
			sub.commitStatus(committed)
		}

		// 价格查询（同一配置只查一次）
		var priceText string
		var priceFetchError string
		hasAvail := false
		for _, n := range notifications {
			if n.changeType == "available" {
				hasAvail = true
				break
			}
		}
		if hasAvail {
			firstDC := ""
			for _, n := range notifications {
				// 跳过本轮验价刚刚暂时性失败（超时/429/5xx）的机房：
				// 1. 该机房询价未命中缓存，立刻重查只会再撞一次 OVH 建车/限流甚至阻塞通知 30s；
				// 2. 下方 resolvePlanPriceInfo 会用公开目录缓存补齐参考价，通知也会附带未完成原因。
				if n.changeType == "available" && n.status != "unavailable" && !unverifiedDCs[n.dc] {
					firstDC = n.dc
					break
				}
			}
			if firstDC != "" {
				priceText, priceFetchError = m.getPriceWithTimeout(planCode, firstDC, priceCfg, 30*time.Second)
				if priceText != "" {
					m.state.Logger.Debug(fmt.Sprintf("配置 %s 价格获取成功: %s，将在所有通知中复用", configDisplay, priceText), "monitor")
				} else {
					m.state.Logger.Warn(fmt.Sprintf("配置 %s 价格获取失败，通知中不包含价格信息", configDisplay), "monitor")
					if priceFetchError == "" {
						priceFetchError = "价格接口未返回结果"
					}
				}
			}
		}

		if !m.checkCurrent(sub, cfg) {
			bail()
			return
		}
		// 分类。通知三兄弟只收 notify=true 的 —— 通知开关只管发不发消息;
		// 下单走下面独立的一遍,不受通知开关影响。
		var availables, unavailables, priceFailed []notification
		for _, n := range notifications {
			if !n.notify {
				// 关闭消息提醒仍保存状态变化,最近补货时间不能依赖通知开关。
				sub.appendHistory([]HistoryEntry{{Timestamp: n.detectedTime, Datacenter: n.dc, Status: n.status, ChangeType: n.changeType, OldStatus: n.oldStatusJSON(), Config: copyMap(configInfo)}}, maxHistorySize)
				continue
			}
			switch n.changeType {
			case "available":
				availables = append(availables, n)
			case "unavailable":
				unavailables = append(unavailables, n)
			case "price_check_failed":
				priceFailed = append(priceFailed, n)
			}
		}

		// 自动下单:从全量 notifications 里挑,不看 notify 标记。
		//
		// oldStatus 的白名单里必须带上 price_check_failed:价格校验把
		// 本地 HTTP 异常 / 30 秒超时 / OVH 429 全算失败,而补货那一刻正是
		// OVH 最容易抖的时候。以前只认 "unavailable",于是
		// unavailable → price_check_failed(抖一下) → available 这条路上,
		// 最后那步跳变会被过滤掉 —— 通知发了「🎉 上架」,一单没下,
		// 而且此后 lastStatus 停在 available 再无跳变,整个补货窗口都不下单。
		orderTargets := []notification{}
		for _, n := range notifications {
			if n.changeType != "available" || n.priceCheckFailed {
				continue
			}
			if !n.hasOld || n.oldStatus == "unavailable" || n.oldStatus == "price_check_failed" {
				orderTargets = append(orderTargets, n)
			}
		}
		// 触发条件:订阅勾了 AutoOrder + 指定了某个账户。
		// 没账户(AutoOrderAccountID="") → 只发可用通知,不下单(用户明确决定)。
		if len(orderTargets) > 0 && cfg.AutoOrder {
			switch {
			case cfg.AutoOrderAccountID == "":
				m.state.Logger.Info(fmt.Sprintf("[monitor] %s 触发 auto_order 但未指定账户,只通知不下单", planCode), "monitor")
			case choice.accountID != cfg.AutoOrderAccountID:
				// 查询/验价账户与下单账户不是同一个 —— 只可能发生在下单账户已被删除、
				// 本轮退回默认账户查询的时候。这种单子照发出去,买到的会是"另一个账户看到的价格
				// 和可售性",甚至直接被 quick-order 以账户不存在拒掉。宁可不下单,只通知。
				m.state.Logger.Warn(fmt.Sprintf(
					"[monitor] %s 跳过自动下单:验价用的是账户 %q,而订阅指定的下单账户是 %q,两者不一致(下单账户可能已被删除)",
					planCode, choice.accountID, cfg.AutoOrderAccountID), "monitor")
			default:
				m.batchOrder(planCode, configInfo, orderTargets, cfg.Quantity, cfg.AutoOrderAccountID, cfg.AutoPay,
					func() bool { return m.checkCurrent(sub, cfg) },
					func(enqueue func() error) error { return m.guardCheckEnqueue(sub, cfg, enqueue) })
				// 已经按这批跳变下过单(或尝试过入队):基线立即写回,
				// 之后哪怕本轮因配置变更提前返回,下一轮也不会把同一次补货再下一单
				sideEffects = true
				commitConfig()
			}
		}

		// 发送有货通知
		if len(availables) > 0 {
			m.state.Logger.Info(fmt.Sprintf("准备发送汇总提醒: %s [%s] - %d个机房有货",
				planCode, configDisplay, len(availables)), "monitor")
			configInfoWithPrice := copyMap(priceCfg)
			priceInfo := m.resolvePlanPriceInfo(planCode, choice.accountID, effectiveOptions)
			if priceText != "" {
				configInfoWithPrice["cached_price"] = priceText
			} else if priceInfo.MonthlyText != "" {
				configInfoWithPrice["cached_price"] = priceInfo.MonthlyText
			}
			// 安装费与首月总计从公开目录算(已缓存 120 秒,不占账户配额)。
			// 不走询价接口:那个要真的建购物车再删,一次好几秒 ——
			// 而补货通知的全部价值就在于"有货那一刻立刻发出去"。
			if priceInfo.InstallText != "" {
				configInfoWithPrice["install_price"] = priceInfo.InstallText
			}
			if priceInfo.FirstMonthText != "" {
				configInfoWithPrice["first_month_price"] = priceInfo.FirstMonthText
			}
			availDCs := make([]map[string]interface{}, 0, len(availables))
			for _, n := range availables {
				dcInfo := map[string]interface{}{"dc": n.dc, "status": n.status}
				if n.rawStatus != "" {
					dcInfo["raw_status"] = n.rawStatus
				}
				if n.durationText != "" {
					dcInfo["duration_text"] = n.durationText
				}
				if n.detectedTime != "" {
					dcInfo["detected_time"] = n.detectedTime
				}
				if unverifiedDCs[n.dc] {
					dcInfo["price_unverified"] = true
				}
				availDCs = append(availDCs, dcInfo)
			}
			configTraceForNotif := ""
			if len(availables) > 0 {
				configTraceForNotif = availables[0].configTraceID
			}
			errIfNoPrice := ""
			if priceText == "" {
				errIfNoPrice = priceFetchError
			}
			if len(priceUnverified) > 0 {
				note := "价格校验未完成（" + priceUnverified[0] + "），已照常交给抢购任务，下单前会完整核对购物车"
				if errIfNoPrice != "" {
					errIfNoPrice += "；" + note
				} else {
					errIfNoPrice = note
				}
			}
			if !m.checkCurrent(sub, cfg) {
				if sideEffects {
					commitConfig()
				}
				bail()
				return
			}
			m.SendAvailabilityAlertGrouped(planCode, availDCs, configInfoWithPrice, cfg.ServerName,
				errIfNoPrice, traceID, configTraceForNotif, choice.accountID)
			sideEffects = true

			entries := make([]HistoryEntry, 0, len(availables))
			for _, n := range availables {
				entries = append(entries, HistoryEntry{
					Timestamp:  m.nowBeijing().Format(time.RFC3339Nano),
					Datacenter: n.dc,
					Status:     n.status,
					ChangeType: n.changeType,
					OldStatus:  n.oldStatusJSON(),
					Config:     configInfo,
				})
			}
			sub.appendHistory(entries, maxHistorySize)
		}

		// 价格校验失败通知
		for _, n := range priceFailed {
			m.state.Logger.Info(fmt.Sprintf("准备发送价格校验失败提醒: %s@%s [%s] - 可用性有货但价格校验失败",
				planCode, n.dc, configDisplay), "monitor")
			priceTextFailed := m.resolvePlanPriceInfo(planCode, accountIDFromConfig(priceCfg), optionsFromConfig(priceCfg)).MonthlyText
			configInfoFailed := copyMap(priceCfg)
			if priceTextFailed != "" {
				configInfoFailed["cached_price"] = priceTextFailed
			}
			configInfoFailed["price_check_error"] = n.priceCheckError
			if !m.checkCurrent(sub, cfg) {
				if sideEffects {
					commitConfig()
				}
				bail()
				return
			}
			m.SendAvailabilityAlert(planCode, n.dc, "unavailable", "price_check_failed",
				configInfoFailed, cfg.ServerName, "", n.priceCheckError, traceID, n.configTraceID, n.detectedTime, choice.accountID)
			sideEffects = true
			sub.appendHistory([]HistoryEntry{{
				Timestamp:  m.nowBeijing().Format(time.RFC3339Nano),
				Datacenter: n.dc,
				Status:     "price_check_failed",
				ChangeType: "price_check_failed",
				OldStatus:  n.oldStatusJSON(),
				Config:     configInfo,
			}}, maxHistorySize)
		}

		// 下架聚合通知
		if len(unavailables) > 0 {
			m.state.Logger.Info(fmt.Sprintf("准备发送聚合下架提醒: %s [%s] - %d个机房",
				planCode, configDisplay, len(unavailables)), "monitor")
			unavailDCs := make([]map[string]interface{}, 0, len(unavailables))
			for _, n := range unavailables {
				dcInfo := map[string]interface{}{"dc": n.dc, "status": n.status}
				isBecame := n.changeType == "unavailable" && n.hasOld && n.oldStatus != "unavailable"
				if isBecame {
					if d := m.calcDuration(sub, n.dc, configDisplay, []string{"available"}); d != "" {
						dcInfo["duration_text"] = d
					}
				}
				unavailDCs = append(unavailDCs, dcInfo)
			}
			configTraceForNotif := ""
			if len(unavailables) > 0 {
				configTraceForNotif = unavailables[0].configTraceID
			}
			if !m.checkCurrent(sub, cfg) {
				if sideEffects {
					commitConfig()
				}
				bail()
				return
			}
			m.SendUnavailableAlertGrouped(planCode, unavailDCs, configInfo, cfg.ServerName,
				traceID, configTraceForNotif)
			entries := make([]HistoryEntry, 0, len(unavailables))
			for _, n := range unavailables {
				entries = append(entries, HistoryEntry{
					Timestamp:  m.nowBeijing().Format(time.RFC3339Nano),
					Datacenter: n.dc,
					Status:     n.status,
					ChangeType: n.changeType,
					OldStatus:  n.oldStatusJSON(),
					Config:     configInfo,
				})
			}
			sub.appendHistory(entries, maxHistorySize)
		}
		// 这套配置的通知和下单都处理完了,基线即刻生效(即使后面的配置提前返回)
		commitConfig()
	}

	// 这里以前会给"本轮没监控的机房"也写一条 lastStatus,而且写的是 OVH 原始可用性值
	// (24H / 1H-low / comingSoon)。这两点都有害:
	//   - 原始值不在状态机认识的 available / unavailable / price_check_failed 里,
	//     后续所有分支都不命中,状态变化会被静默吞掉;
	//   - 这些机房根本没做过价格校验,凭空写一条状态,等于让用户之后把它加进监控时
	//     "首次检查"分支失效,当次有货既不通知也不触发 auto-order。
	// 监控范围内的机房在上面的主循环里已经写过规范化状态了,所以这里不再补写。
	if !m.checkCurrent(sub, cfg) {
		bail()
		return
	}
	sub.replaceLastStatus(lastStatus)
	// 状态真的动过才标脏,循环末尾统一落一次库。不标的话 LastStatus 只活在内存里,
	// 重启后当次有货会被当成"初始存量"吞掉(既不通知也不自动下单)。
	// 但也不能每轮无条件标脏:那等于每 5 秒整表重写一次订阅(连同每条 100 条历史)。
	if !statusEqual(before, lastStatus) {
		m.dirty.Store(true)
	}
}
