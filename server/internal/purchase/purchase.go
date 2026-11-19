package purchase

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"
	"golang.org/x/sync/errgroup"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/domain/datacenter"
	"github.com/ovh-webui/server/internal/notify"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// Outcome 一次抢购尝试的结果。
//
// 以前 PurchaseServer 只返回 bool,队列处理器分不出"这轮没货,下轮再来"和
// "这台机器这个账户永远买不到"。后者(跨区 planCode / 非 Eco 机型 / 账户被删)
// 会按 retryInterval 无限重试,而且每一轮都往 history 写一条失败记录 ——
// 一个手滑选错区的任务能刷满整个抢购历史。Fatal 就是用来终止这类任务的。
type Outcome struct {
	Success           bool
	WaitingForCatalog bool
	// Fatal:确定性失败,重试多少次结果都一样。队列处理器据此把任务直接置 failed。
	// 只有"换个时间点重试也必然是同一个结果"才允许置 true ——
	// 网络 / 429 / OVH 5xx / 无货 一律不是 Fatal。
	Fatal bool
	// Reason:Fatal 时给用户看的原因(已写进 history,这里带一份给日志)
	Reason string
	// Attempted:这一轮是否真的走到了"向 OVH 提交订单"这一步。
	// 无货的轮次是 false —— 抢购的常态就是绝大多数轮次都无货,
	// 把它们计进失败次数会让任务在还没真正尝试过几次时就被判死。
	Attempted bool
	// Cancelled:任务在下单途中被用户删除,本轮主动终止。
	// 不是失败:不写 history、不计 FailureCount、不算 Attempted。
	// 结账之前的任何一步都可能落到这里;结账一旦发出就不再接受取消(见 checkout 处注释)。
	Cancelled bool
	// PauseTask:结账结果不确定(超时+对账也失败),暂停任务等人工确认。
	// 不计 FailureCount(不是确定性失败);不发 Fatal(用户确认无单后可恢复)
	PauseTask bool
	// CheckoutUncertain:结账请求已经发出(或上一轮的未决结账仍无法确认),OVH 侧是否建单未知。
	// 只有它为 true 时队列才保留 UncertainCheckout 标记,恢复运行时先对账。
	// 优惠码被拒、结账凭据没存上这类"根本没结账"的暂停只设 PauseTask,不设它 ——
	// 否则恢复时会去对一笔从未提交过的结账,把用户自己手动下的同型号订单误认成本任务的单。
	CheckoutUncertain bool
}

// cancelledOutcome 下单途中任务被删除时的返回。
// 只记日志:history 里不留痕(用户自己删的,不是失败),耗时统计也不记(不是一次完整尝试)。
func cancelledOutcome(state *app.State, item *types.QueueItem, stage string) Outcome {
	state.Logger.Info(fmt.Sprintf("任务 %s (%s @ %s) 在「%s」阶段被取消,本轮终止",
		item.ID, item.PlanCode, item.Datacenter, stage), "purchase")
	return Outcome{Cancelled: true}
}

// 多账户:用 item.AccountID 取对应 OVH client 和 subsidiary。
//
// ctx 由队列处理器给,用户删任务时会被取消(app.State.MarkTaskDeleted)。
// 结账之前的每一次 OVH 调用都挂在它上面,取消即中断;结账本身例外,见那里的注释。
func PurchaseServer(ctx context.Context, state *app.State, item *types.QueueItem) Outcome {
	// 原先只对目录监控关闭自动付款，保留原代码：
	// if strings.HasPrefix(item.ID, "catalog-watch-") {
	//     item.AutoPay = false
	// }
	// 自动付款已停用，包括旧库及旧客户端传入的 AutoPay=true。
	item.AutoPay = false
	attemptStart := time.Now() // checkout 对账用:transient 失败后按此时间查 /me/order
	// 购买时长:空值(旧任务)按 P1M;非法值是确定性失败,重试也不会变
	selectedDuration, termErr := orderterm.Normalize(item.Duration)
	if termErr != nil {
		recordFailure(state, item, termErr.Error())
		return Outcome{Fatal: true, Reason: termErr.Error()}
	}
	client, err := state.OVH.IsolatedClientFor(item.AccountID)
	if err != nil {
		// 账户不存在 / 凭据缺失 / endpoint 非法 —— 都是重试一万次也不会变的错。
		// 以前这里连 history 都不写,任务就在后台每 retryInterval 秒静默失败一次。
		errMsg := fmt.Sprintf("账户 %s 不可用: %s（重试无用，请检查账户是否还在、API 密钥是否填全）",
			item.AccountID, err.Error())
		state.Logger.Error("PurchaseServer: 取 OVH client 失败 ("+item.AccountID+"): "+err.Error(), "purchase")
		recordFailure(state, item, errMsg)
		return Outcome{Fatal: true, Reason: errMsg}
	}
	// The SDK overwrites http.Client.Timeout with its own default (180s).
	// Set the SDK timeout on this isolated workflow, including checkout settlement.
	if strings.HasPrefix(item.ID, "catalog-watch-") && (client.Timeout <= 0 || client.Timeout > 60*time.Second) {
		client.Timeout = 60 * time.Second
	}

	// 预订(移植上游 4f1c4e6 / 1205120):不再等公开销售目录收录 —— 库存会先于目录公布,
	// 而目录可能一直不更新。是否开售以真实下单接口为准;在那之前每一步被 OVH 拒都只是
	// "还没开放",按等待处理,不写失败历史、不计失败次数。
	// preorderStock 是库存匹配条件(内存/完整硬盘组合/GPU 逐字匹配,CPU 不参与),
	// preorderOrder 是下单要加的 addon。
	preorderStock, preorderOrder := item.Options, item.Options
	gateKey := ""
	if item.Preorder {
		gateKey = preorderGateKey(item, selectedDuration)
		// 同型号同配置的另一个预订任务刚被明确告知"未开放":本轮不再打 OVH(见 preorder_gate.go)
		if !stockWoken(ctx) && preorderGateClosed(gateKey, item.ID, time.Now()) {
			return Outcome{WaitingForCatalog: true, Reason: "同型号同配置的预订任务刚确认尚未开放"}
		}
		var resolveErr error
		preorderStock, preorderOrder, resolveErr = catalog.ResolvePreorderOptions(state, item.AccountID, item.PlanCode, item.Options)
		if resolveErr != nil {
			return preorderWait(state, item, resolveErr.Error())
		}
	}

	cartID := ""
	var itemID int64
	var couponWarning string

	// 每一轮都打点。抢购的胜负常常只差几百毫秒,而"差在哪"没有数字就只能靠猜。
	tl := newTimeline()
	timingKey := TimingKey(item.PlanCode, item.Datacenter)

	if item.UncertainCheckout {
		accCheck, _ := state.FindAccount(item.AccountID)
		attemptStart := time.Now().Add(-10 * time.Minute)
		if tParse, ok := types.ParseTS(item.UncertainTime); ok {
			attemptStart = tParse.Add(-2 * time.Minute)
		}
		// 对账与认领在同一把(账户+型号)锁里完成,见 reconcileAndClaim
		oidCheck, orderURLCheck, reconcileErr := reconcileAndClaim(state, client, accCheck, item, attemptStart, "")
		if reconcileErr != nil {
			reason := "上一轮结账结果仍不确定，对账失败，暂停以避免重复下单: " + reconcileErr.Error()
			state.Logger.Warn(reason, "purchase")
			return Outcome{PauseTask: true, CheckoutUncertain: true, Reason: reason}
		}
		if oidCheck != "" {
			state.Logger.Warn(fmt.Sprintf("抢购前对账发现上一轮未决结账已在 OVH 成功建单(#%s),按成功处理,不再重复下单", oidCheck), "purchase")
			item.UncertainCheckout = false
			return handleReconciledOrder(state, client, accCheck.Endpoint, item, oidCheck, orderURLCheck, tl, timingKey, "")
		}
		item.UncertainCheckout = false
	}

	state.Logger.Info(fmt.Sprintf("开始为 %s 在 %s 的购买流程，选项: %v",
		item.PlanCode, item.Datacenter, item.Options), "purchase")

	// 检查可用性
	var availabilities []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", item.PlanCode)
	if err := client.GetWithContext(ctx, "/dedicated/server/datacenter/availabilities?"+q.Encode(), &availabilities); err != nil {
		if ctx.Err() != nil {
			return cancelledOutcome(state, item, "查库存")
		}
		tl.mark("查库存")
		recordTiming(timingKey, tl, "failed")
		errMsg := err.Error()
		state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误: %s", item.PlanCode, errMsg), "purchase")
		recordFailure(state, item, ovh.Explain(err))
		return attemptOutcome(err)
	}
	tl.mark("查库存")

	// 空数组 ≠ 暂时没货。OVH 三个站点的机型目录彼此独立,拿别的站点的 planCode 查
	// /dedicated/server/datacenter/availabilities 同样是 HTTP 200 + [](实测:
	// 24rise01-v1 在 US 站点 n=0、24rise01-v1-us 在 EU/CA 站点 n=0),
	// 和"这台机器暂时没货"长得一模一样。不区分的话跨区任务会按重试间隔永远刷下去,
	// 日志里只有一句"当前无货",用户永远看不出是自己选错了区的机型。
	// classifyPlan 给出确定性结论时直接判 Fatal:这类任务重试到天荒地老也不会变。
	if len(availabilities) == 0 {
		if verdict, msg := catalog.ClassifyPlan(state, item.AccountID, item.PlanCode, "purchase"); msg != "" {
			// Force 允许未收录的自定义型号继续等待库存,不绕过明确跨区或非 Eco。
			if item.Force && catalog.CanForceCustomPlan(verdict) {
				state.Logger.Warn(fmt.Sprintf("Force 跳过未收录判定,继续按无货重试: %s", msg), "purchase")
			} else {
				state.Logger.Error(fmt.Sprintf("%s（判定 %d）", msg, verdict), "purchase")
				recordFailure(state, item, msg)
				return Outcome{Fatal: true, Reason: msg}
			}
		}
	}

	apiDC := ovh.ConvertDisplayDCToAPIDC(item.Datacenter)

	// dedicated.DatacenterAvailability 的粒度是 fqn（一整套硬件组合），不是 planCode：
	// "32G+HDD 有货" 不等于 "用户要的 64G+NVMe 有货"。用户指定了硬件选项时，
	// 只在 FQN 段集合覆盖了这些选项的条目里判有货。
	wantedHW := fqnRelevantOptions(availabilities, item.Options, item.PlanCode)
	candidates := availabilities
	if item.Preorder {
		// 预订保存的是原始 FQN 硬件段;某段这轮没出现也不能忽略它、改买其他配置。
		// 而且只认型号完全相同的 FQN:同系列的另一个型号(…-v2 / 区域变体)不是同一台机器。
		wantedHW = preorderStock
		candidates = nil
		for _, av := range availabilities {
			fqn, _ := av["fqn"].(string)
			if fqnIsPlan(fqn, item.PlanCode) {
				candidates = append(candidates, av)
			}
		}
	}
	if len(wantedHW) > 0 {
		matchedAvs := []map[string]interface{}{}
		for _, av := range candidates {
			fqn, _ := av["fqn"].(string)
			matches := fqnCoversOptions(fqn, wantedHW, item.PlanCode)
			if item.Preorder {
				matches = preorderFQNCoversOptions(fqn, wantedHW, item.PlanCode)
			}
			if matches {
				matchedAvs = append(matchedAvs, av)
			}
		}
		if len(matchedAvs) > 0 {
			candidates = matchedAvs
		} else {
			// wantedHW 里每一项都在本次 availabilities 的 FQN 段里出现过（否则会被
			// fqnRelevantOptions 剔掉），却没有任何一条 FQN 同时覆盖它们 ——
			// 说明 OVH 根本不供这套组合，直接判无货。
			// 这里退回全量判定就会重演"32G+HDD 有货 → 把 64G+NVMe 的单也下出去"。
			state.Logger.Info(fmt.Sprintf("服务器 %s 没有任何配置组合同时包含所选硬件 %v，视为该配置无货",
				item.PlanCode, wantedHW), "purchase")
			recordTiming(timingKey, tl, "unavailable")
			return Outcome{}
		}
	}

	foundAvailable := false
	// 记下"实际可用的那条 FQN"。FQN 格式：<planCode>.<addon1>.<addon2>...
	// 用户没显式指定 options 时，会从这个 FQN 推断 addon，让订单走"有货的那套配置"，
	// 不再退化到 OVH 默认 addon（多半是 HDD / 最小内存）。
	var availableFQN string
	for _, av := range candidates {
		if dcsRaw, ok := av["datacenters"].([]interface{}); ok {
			for _, dcRaw := range dcsRaw {
				dc, ok := dcRaw.(map[string]interface{})
				if !ok {
					continue
				}
				dcName, _ := dc["datacenter"].(string)
				availStr, _ := dc["availability"].(string)
				// 白名单判定：只有 \d+H 家族算有货。comingSoon 是"即将上线尚未开售"，
				// 以前被当成有货，会为永远下不了单的机型跑完整个下单流程、白刷 OVH 限流额度。
				if (dcName == apiDC || datacenter.SameZoneOrAlias(dcName, item.Datacenter)) && catalog.IsAvailableForOrder(availStr) {
					foundAvailable = true
					if dcName != "" {
						apiDC = dcName
					}
					if fqn, ok := av["fqn"].(string); ok {
						availableFQN = fqn
					}
					break
				}
			}
		}
		if foundAvailable {
			break
		}
	}
	if !foundAvailable {
		recordTiming(timingKey, tl, "unavailable")
		state.Logger.Info(fmt.Sprintf("服务器 %s 在数据中心 %s 当前无货 (%s)", item.PlanCode, item.Datacenter, tl.String()), "purchase")
		return Outcome{}
	}

	// 决定本次下单使用的硬件 options：
	// - 用户显式指定了 options → 直接用（fail-fast 由后面的 eco/options 处理）
	// - 用户没指定 → 从可用 FQN 推断 addon planCode，确保订单走"实际有货的那套配置"
	effectiveOptions := item.Options
	if item.Preorder {
		effectiveOptions = preorderConfigurationOptions(availableFQN, item.PlanCode, preorderStock, preorderOrder)
	} else if len(effectiveOptions) == 0 && availableFQN != "" {
		if inferred := optionsFromFQN(availableFQN, item.PlanCode); len(inferred) > 0 {
			effectiveOptions = inferred
			state.Logger.Info(fmt.Sprintf("用户未指定硬件选项，从可用 FQN %s 推断 addon: %v",
				availableFQN, effectiveOptions), "purchase")
		}
	}

	// 多账户:购物车 subsidiary 跟着账户走,不再读全局 cfg
	acc, _ := state.FindAccount(item.AccountID)
	subsidiary := orderSubsidiary(state, acc, "purchase")

	// 云下单:进度卡片切到"正在提交"(未绑卡片时直接返回)
	telegram.NotifyTaskProgress(state, item, "submitting", nil)

	// 抢购失败时清理 OVH 购物车,避免 OVH 侧堆积僵尸 cart(高频抢购累计能上千个,
	// 进而触发 OVH 限流)。checkout 成功时 cart 自动转 order,Delete 会 404,
	// 所以只在 !success 时尝试,且失败不影响主流程。
	success := false
	defer func() {
		if success || cartID == "" {
			return
		}
		// 故意不带 ctx:取消路径也要走到这里清 cart。挂上 ctx 的话,
		// 用户一删任务,清理请求自己先被取消,僵尸 cart 就留在 OVH 那边了。
		if err := client.Delete("/order/cart/"+cartID, nil); err != nil {
			state.Logger.Debug(fmt.Sprintf("清理失败 cart %s: %s", cartID, err.Error()), "purchase")
		} else {
			state.Logger.Debug("已清理失败 cart "+cartID, "purchase")
		}
		ovh.ReleaseCart(cartID) // 删失败也撤销登记,交给遗留车清理兜底
	}()

	// ⚡ 检查预装车池：若后台已提前组装好满配就绪车，直接跳过装车阶段直达 checkout，装车耗时归零！
	var prewarmed *PrewarmedCart
	if !item.Preorder && ctx.Value(skipPrewarmKey{}) == nil {
		prewarmed = TakePrewarmedCart(item.AccountID, item.PlanCode, item.Datacenter, effectiveOptions, item.Coupon, selectedDuration)
	}
	// 预订不用预热车:结账前要逐项核对购物车里的实际配置,预热车拿不到基础商品 itemId
	var orderedOptions []string
	if prewarmed != nil {
		cartID = prewarmed.CartID
		state.Logger.Info(fmt.Sprintf("⚡ 命中预装车池！直接使用预热就绪购物车 %s，装车耗时 0 秒！", cartID), "purchase")
		tl.mark("预装车就绪")
	} else {

		// 创建购物车
		state.Logger.Info(fmt.Sprintf("为区域 %s 创建购物车 (账户 %s)", subsidiary, acc.Name), "purchase")
		var cartResult map[string]interface{}
		if err := client.PostWithContext(ctx, "/order/cart", ovh.CartCreateBody(subsidiary), &cartResult); err != nil {
			if ctx.Err() != nil {
				return cancelledOutcome(state, item, "建购物车")
			}
			state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误: %s", item.PlanCode, err.Error()), "purchase")
			recordFailure(state, item, ovh.Explain(err))
			return attemptOutcome(err)
		}
		cartID, _ = cartResult["cartId"].(string)
		ovh.TrackCart(cartID)
		tl.mark("建购物车")
		state.Logger.Info("购物车创建成功，ID: "+cartID, "purchase")

		// 并发执行：绑定购物车 (assign) + 添加基础商品 (eco)
		// OVH API 的 /assign 与 /eco 只依赖 cartId，二者并发可节省 1 次网络 RTT (~450ms)
		state.Logger.Info(fmt.Sprintf("并发执行绑定购物车与加购基础商品 %s (使用 /assign 与 /eco)", item.PlanCode), "purchase")
		// 月付走零额外请求的快速路径;其他周期在 eco 协程里先查真实 duration/pricingMode。
		baseDuration, basePricingMode := selectedDuration, "default"
		var itemResult map[string]interface{}
		var assignErr error
		var postBaseErr error
		termUnavailable := false

		postBase := func(callCtx context.Context, cli *ovhsdk.Client) error {
			return cli.PostWithContext(callCtx, "/order/cart/"+cartID+"/eco", map[string]interface{}{
				"planCode":    item.PlanCode,
				"pricingMode": basePricingMode,
				"duration":    baseDuration,
				"quantity":    1,
			}, &itemResult)
		}

		// 硬件选项列表与 assign / 加购并发预取(只依赖 cartId;缓存命中时零往返)。
		// 以前要等加购完成才去拉,冷启动时串行多一次往返。用独立 client:
		// go-ovh 每次请求都会写 http.Client.Timeout,共享实例并发调用是数据竞争。
		type ecoOptsResult struct {
			opts []map[string]interface{}
			err  error
		}
		var ecoOptsCh chan ecoOptsResult
		var ecoOptsConsumed bool
		var cancelPrefetch context.CancelFunc
		if len(effectiveOptions) > 0 {
			ecoOptsCh = make(chan ecoOptsResult, 1)
			var prefetchCtx context.Context
			prefetchCtx, cancelPrefetch = context.WithCancel(ctx)
			defer func() {
				cancelPrefetch()
				if !ecoOptsConsumed {
					<-ecoOptsCh
				}
			}()
			prefetchClient, perr := state.OVH.IsolatedClientFor(item.AccountID)
			if perr != nil {
				prefetchClient = nil
			}
			go func() {
				if prefetchClient == nil {
					ecoOptsCh <- ecoOptsResult{err: perr}
					return
				}
				if item.Preorder {
					// 预订不用缓存:开售前拉到的选项列表里还没有新配置,
					// 拿它判断会让开售后的第一次尝试误判"所选配置未开放",白白浪费一轮
					invalidateEcoOptions(subsidiary, item.PlanCode)
				}
				opts, err := getOrFetchEcoOptions(prefetchCtx, prefetchClient, cartID, subsidiary, item.PlanCode)
				ecoOptsCh <- ecoOptsResult{opts: opts, err: err}
			}()
		}

		g2, ctx2 := errgroup.WithContext(ctx)

		// 1. assign
		g2.Go(func() error {
			isoClient, ierr := ovh.IsolateClient(client)
			if ierr != nil {
				assignErr = ierr
				return ierr
			}
			state.Logger.Info("绑定购物车 "+cartID, "purchase")
			if err := isoClient.PostWithContext(ctx2, "/order/cart/"+cartID+"/assign", nil, nil); err != nil {
				assignErr = err
				return err
			}
			state.Logger.Info("购物车绑定成功", "purchase")
			return nil
		})

		// 2. eco
		g2.Go(func() error {
			isoClient, ierr := ovh.IsolateClient(client)
			if ierr != nil {
				postBaseErr = ierr
				return ierr
			}
			state.Logger.Info(fmt.Sprintf("添加基础商品 %s 到购物车 (使用 /eco)", item.PlanCode), "purchase")
			if selectedDuration != orderterm.Default {
				d, pm, found := lookupEcoPricing(state, isoClient, cartID, item.PlanCode, selectedDuration)
				if !found {
					termUnavailable = true
					postBaseErr = fmt.Errorf("OVH 不支持 %s 按 %s 购买,请修改任务的购买时长后重试", item.PlanCode, orderterm.Label(selectedDuration))
					return postBaseErr
				}
				baseDuration, basePricingMode = d, pm
			}
			if err := postBase(ctx2, isoClient); err != nil {
				if ctx2.Err() != nil {
					postBaseErr = err
					return err
				}
				if d, pm, found := lookupEcoPricing(state, isoClient, cartID, item.PlanCode, selectedDuration); found &&
					(d != baseDuration || pm != basePricingMode) {
					state.Logger.Warn(fmt.Sprintf("以 %s/%s 加购 %s 失败(%s)，改用目录计价 %s/%s 重试",
						baseDuration, basePricingMode, item.PlanCode, err.Error(), d, pm), "purchase")
					baseDuration, basePricingMode = d, pm
					err = postBase(ctx2, isoClient)
				}
				if err != nil {
					postBaseErr = err
					return err
				}
			}
			return nil
		})

		if err := g2.Wait(); err != nil {
			if ctx.Err() != nil {
				return cancelledOutcome(state, item, "建车加购")
			}
			// 必须排在 assignErr 之前:eco 协程判定周期不可买后,errgroup 会取消并行的
			// assign,那个 "context canceled" 不是真正的失败原因 —— 先看它就会把确定性失败
			// 当成可重试错误,任务按 retryInterval 永远重试。
			if item.Preorder && (termUnavailable || (postBaseErr != nil && listingNotReady(postBaseErr))) {
				// 开售前 OVH 不接受这个型号/周期:继续等,不算失败;同型号同配置的其它任务短时间内不必再试
				closePreorderGate(gateKey, item.ID)
				// 未开售时拉到的选项列表不可信,别留给同型号的其它任务
				invalidateEcoOptions(subsidiary, item.PlanCode)
				return preorderWait(state, item, ovh.Explain(postBaseErr))
			}
			if termUnavailable {
				// 目录里没有这个周期:重试多少轮都一样,直接判死,别反复建车
				state.Logger.Error(postBaseErr.Error(), "purchase")
				recordFailure(state, item, postBaseErr.Error())
				return Outcome{Fatal: true, Reason: postBaseErr.Error()}
			}
			if assignErr != nil {
				errMsg := assignErr.Error()
				state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误(assign): %s", item.PlanCode, errMsg), "purchase")
				state.Logger.Error("错误发生时的购物车ID: "+cartID, "purchase")
				recordFailure(state, item, ovh.Explain(assignErr))
				return attemptOutcome(assignErr)
			}
			if postBaseErr != nil {
				state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误(eco): %s", item.PlanCode, postBaseErr.Error()), "purchase")
				state.Logger.Error(fmt.Sprintf("错误发生时的购物车ID: %s", cartID), "purchase")
				recordFailure(state, item, ovh.Explain(postBaseErr))
				return attemptOutcome(postBaseErr)
			}
			state.Logger.Error(fmt.Sprintf("购买 %s 时发生未知错误: %s", item.PlanCode, err.Error()), "purchase")
			recordFailure(state, item, ovh.Explain(err))
			return attemptOutcome(err)
		}

		if n, ok := numconv.ToInt64(itemResult["itemId"]); ok {
			itemID = n
		}
		if itemID == 0 {
			errMsg := fmt.Sprintf("无法从购物车响应中解析 itemId（响应: %v）", itemResult)
			state.Logger.Error(fmt.Sprintf("购买 %s 时发生未知错误: %s", item.PlanCode, errMsg), "purchase")
			state.Logger.Error("错误发生时的购物车ID: "+cartID, "purchase")
			recordFailure(state, item, errMsg)
			return Outcome{Attempted: true}
		}
		tl.mark("绑定并加购")
		state.Logger.Info(fmt.Sprintf("基础商品添加成功，项目 ID: %d", itemID), "purchase")

		// 设置必需配置
		state.Logger.Info(fmt.Sprintf("为项目 %d 设置必需配置", itemID), "purchase")
		// region 必须按 (子公司, planCode) 从目录取:US 子公司所有机型都是 united_states,
		// 欧区只有 canada/europe(亚太机型归 canada)。老的静态机房映射会发出目录里
		// 根本不存在的 "usa"/"apac",美区每一单都卡在这里。
		region, regionSrc := catalog.ResolveRegion(state, item.AccountID, item.PlanCode, apiDC)
		if regionSrc == "fallback" {
			// catalog.ResolveRegion 的兜底分支拿的是账户原始 zone,zone 为空时(美区账户很常见,
			// 建号时不填就是空)会算成 europe/canada,而美区目录里 143 个 plan 的 region
			// 只有 united_states 一个取值 —— 目录一拉不动,美区每一单都会提交本区不存在的 region。
			// 这里用已经归一化过的 subsidiary 重算一次。
			// 必须走 catalog.FallbackRegion 而不是 ovh.RegionForDCInSubsidiary:
			// 后者只认 gra/bhs 这种短机房码,而可用性接口会返回 ca-east-tor-a / eu-west-par-a
			// 这类长码,直接问它会拿到空串,反而把已经算对的 region 覆盖掉。
			if r := catalog.FallbackRegion(apiDC, subsidiary); r != region {
				state.Logger.Warn(fmt.Sprintf("目录兜底给出的 region %q 与子公司 %s 不符，改用 %q",
					region, subsidiary, r), "purchase")
				region, regionSrc = r, "fallback(subsidiary-corrected)"
			}
		}
		if region == "" {
			// 目录拉不动 + 静态兜底也认不出机房时的最后一招:问购物车自己。
			// requiredConfiguration 的 schema 只声明 label/required/type,但三区实测都会返回
			// allowedValues,而且这份值是这辆车(= 这个子公司 + 这个 planCode)算出来的,
			// 天然是本区正确值:US 车返回 ["united_states"],EU/CA 车返回 ["canada","europe"]。
			// 老实现只用它判"required 吗"然后整单取消,等于守着答案不看。
			state.Logger.Warn(fmt.Sprintf("无法为数据中心 %s 推断区域，改问购物车的 requiredConfiguration",
				strings.ToLower(apiDC)), "purchase")
			var required []map[string]interface{}
			if err := client.GetWithContext(ctx, fmt.Sprintf("/order/cart/%s/item/%d/requiredConfiguration", cartID, itemID), &required); err != nil {
				if ctx.Err() != nil {
					return cancelledOutcome(state, item, "查必需配置")
				}
				state.Logger.Warn("获取必需配置失败: "+err.Error(), "purchase")
			} else {
				allowed, mandatory := regionAllowedValues(required)
				if v := pickAllowedRegion(allowed, catalog.FallbackRegion(apiDC, subsidiary)); v != "" {
					region, regionSrc = v, "requiredConfiguration"
				} else if mandatory {
					errMsg := fmt.Sprintf("必需的区域配置无法确定：子公司 %s 的目录拉不到，购物车也没给出 region 的合法取值。", subsidiary)
					state.Logger.Error(fmt.Sprintf("购买 %s 时发生未知错误: %s", item.PlanCode, errMsg), "purchase")
					recordFailure(state, item, errMsg)
					return Outcome{Attempted: true}
				}
			}
		}
		if region != "" {
			state.Logger.Info(fmt.Sprintf("区域配置: %s = %s(来源: %s)", apiDC, region, regionSrc), "purchase")
		}

		// 顺序与参数：dedicated_datacenter → dedicated_os → (region)
		type kv struct{ label, value string }
		configurations := []kv{
			{"dedicated_datacenter", apiDC},
			{"dedicated_os", "none_64.en"},
		}
		if region != "" {
			configurations = append(configurations, kv{"region", region})
		}

		// 硬件选项准备。effectiveOptions 已经包含了：
		//   - 用户显式 options（如果有），或
		//   - 从可用 FQN 推断的 addon planCode（用户没指定时）
		type addonPayload struct {
			planCode string
			body     map[string]interface{}
		}
		var todo []addonPayload
		if len(effectiveOptions) > 0 {
			state.Logger.Info(fmt.Sprintf("📦 处理硬件选项（%d个）: %v", len(effectiveOptions), effectiveOptions), "purchase")
			filtered := filterHardwareOptions(state, effectiveOptions, true)
			if len(filtered) > 0 {
				state.Logger.Info(fmt.Sprintf("过滤后的硬件选项计划代码: %v", filtered), "purchase")
				// 优先读取内存缓存，消除抢购链路上 GET /eco/options 的网络阻塞
				var availableEcoOpts []map[string]interface{}
				var err error
				if ecoOptsCh != nil {
					r := <-ecoOptsCh
					ecoOptsConsumed = true
					availableEcoOpts, err = r.opts, r.err
				} else {
					availableEcoOpts, err = getOrFetchEcoOptions(ctx, client, cartID, subsidiary, item.PlanCode)
				}
				if err != nil {
					if ctx.Err() != nil {
						return cancelledOutcome(state, item, "查硬件选项")
					}
					if item.Preorder && listingNotReady(err) {
						return preorderWait(state, item, ovh.Explain(err))
					}
					// 拉 eco/options 失败 → 中止订单。否则会用基础 plan 默认存储（多半是 HDD）下到错误配置
					errMsg := fmt.Sprintf("获取 Eco 硬件选项列表失败: %s（用户指定了 %d 个选项，无法验证，已取消下单避免下到错误配置）", ovh.Explain(err), len(filtered))
					state.Logger.Error(errMsg, "purchase")
					recordFailure(state, item, errMsg)
					return attemptOutcome(err)
				}
				state.Logger.Info(fmt.Sprintf("找到 %d 个可用的 Eco 硬件选项。", len(availableEcoOpts)), "purchase")

				var missing []string
				for _, wanted := range filtered {
					// 分档匹配(matchEcoOption,与 price.go 同一口径):用户从前端选来的是完整
					// planCode(第①档直接命中);用户没选配置时 filtered 是从 FQN 推来的短前缀,
					// 精确匹配永远打不中,必须靠前缀/标准化档兜底,否则会被误判成"选项不存在"整单取消。
					matchedOpt, matchedPC, tier := catalog.MatchEcoOption(availableEcoOpts, wanted)
					if item.Preorder {
						// 预订只接受原始规格(加本型号后缀),不按标准化结果换成另一套硬件
						matchedOpt, matchedPC, tier = catalog.MatchPreorderEcoOption(availableEcoOpts, wanted)
					}
					if matchedOpt == nil {
						missing = append(missing, wanted)
						continue
					}
					if matchedPC != wanted {
						state.Logger.Info(fmt.Sprintf("硬件选项 %s 按「%s」匹配到 Eco 选项 %s", wanted, tier, matchedPC), "purchase")
					}
					duration, pricingMode, valid := pickPricing(matchedOpt["prices"], baseDuration, basePricingMode)
					if !valid {
						errMsg := fmt.Sprintf("硬件选项 %s 不支持购买时长 %s,已取消下单", matchedPC, orderterm.Label(baseDuration))
						if item.Preorder {
							invalidateEcoOptions(subsidiary, item.PlanCode)
							return preorderWait(state, item, fmt.Sprintf("硬件选项 %s 暂未开放 %s 计价", matchedPC, orderterm.Label(baseDuration)))
						}
						state.Logger.Error(errMsg, "purchase")
						recordFailure(state, item, errMsg)
						return Outcome{Fatal: true, Reason: errMsg}
					}
					todo = append(todo, addonPayload{
						planCode: matchedPC,
						body: map[string]interface{}{
							"itemId":      itemID,
							"planCode":    matchedPC,
							"duration":    duration,
							"pricingMode": pricingMode,
							"quantity":    1,
						},
					})
				}
				if len(missing) > 0 && item.Preorder {
					closePreorderGate(gateKey, item.ID)
					invalidateEcoOptions(subsidiary, item.PlanCode)
					return preorderWait(state, item, fmt.Sprintf("等待所选配置开放: %v", missing))
				}
				if len(missing) > 0 {
					errMsg := fmt.Sprintf("用户请求的硬件选项 %v 未在 OVH 可用 Eco 选项中找到（已取消下单避免下到错误配置）", missing)
					state.Logger.Error(errMsg, "purchase")
					recordFailure(state, item, errMsg)
					// 用户显式指定的选项对不上该子公司目录 = 确定性失败:eco/options 是按
					// (子公司, planCode) 算出来的固定集合,下一轮重试还是同一份,只会再刷一条失败 history。
					// 但"用户没指定、从 FQN 推来的"那批不算:下一轮有货的可能是另一条 FQN,
					// 推出来的段就不一样了,判死会把本来能抢到的任务掐掉。
					if len(item.Options) > 0 {
						return Outcome{Fatal: true, Reason: errMsg}
					}
					return Outcome{}
				}
			}
		} else {
			state.Logger.Info("⚠️ 用户未提供任何硬件选项，将使用默认配置下单", "purchase")
		}

		// ⚡ 并发下发必需配置与硬件选项：
		// 将 dedicated_datacenter、dedicated_os、region 以及所有 options 并发写入。
		// 单车配置装车耗时从原先的 4~6 次串行 RTT 压缩为 1 次 RTT (~450ms)
		state.Logger.Info(fmt.Sprintf("并发下发必需配置(%d项)与硬件选项(%d项)", len(configurations), len(todo)), "purchase")
		g3, ctx3 := errgroup.WithContext(ctx)

		for _, cfg := range configurations {
			c := cfg
			g3.Go(func() error {
				isoClient, ierr := ovh.IsolateClient(client)
				if ierr != nil {
					return ierr
				}
				state.Logger.Info(fmt.Sprintf("配置项目 %d: 设置必需项 %s = %s", itemID, c.label, c.value), "purchase")
				if err := isoClient.PostWithContext(ctx3, fmt.Sprintf("/order/cart/%s/item/%d/configuration", cartID, itemID),
					map[string]interface{}{"label": c.label, "value": c.value}, nil); err != nil {
					return fmt.Errorf("config %s error: %w", c.label, err)
				}
				state.Logger.Info(fmt.Sprintf("成功设置必需项: %s = %s", c.label, c.value), "purchase")
				return nil
			})
		}

		for _, t := range todo {
			opt := t
			g3.Go(func() error {
				isoClient, ierr := ovh.IsolateClient(client)
				if ierr != nil {
					return ierr
				}
				state.Logger.Info(fmt.Sprintf("添加 Eco 选项: %s", opt.planCode), "purchase")
				if err := isoClient.PostWithContext(ctx3, fmt.Sprintf("/order/cart/%s/eco/options", cartID), opt.body, nil); err != nil {
					return fmt.Errorf("option %s error: %w", opt.planCode, err)
				}
				state.Logger.Info(fmt.Sprintf("成功添加 Eco 选项: %s", opt.planCode), "purchase")
				return nil
			})
		}

		if err := g3.Wait(); err != nil {
			if ctx.Err() != nil {
				return cancelledOutcome(state, item, "配置与选件写入")
			}
			if item.Preorder && listingNotReady(err) {
				return preorderWait(state, item, ovh.Explain(err))
			}
			errMsg := err.Error()
			state.Logger.Error(fmt.Sprintf("购买 %s 时并发装车写入失败: %s", item.PlanCode, errMsg), "purchase")
			state.Logger.Error("错误发生时的购物车ID: "+cartID, "purchase")
			state.Logger.Error(fmt.Sprintf("错误发生时的基础商品ID: %d", itemID), "purchase")
			recordFailure(state, item, ovh.Explain(err))
			return attemptOutcome(err)
		}
		tl.mark("必需配置与选件")
		for _, t := range todo {
			orderedOptions = append(orderedOptions, t.planCode)
		}

		couponWarning = ""
		if coupon := strings.TrimSpace(item.Coupon); coupon != "" {
			err := client.PostWithContext(ctx, "/order/cart/"+cartID+"/coupon", map[string]interface{}{"coupon": coupon}, nil)
			if ctx.Err() != nil {
				return cancelledOutcome(state, item, "应用优惠码")
			}
			tl.mark("应用优惠码")
			if err != nil {
				var apiErr *ovhsdk.APIError
				if IsTransient(err) || !errors.As(err, &apiErr) || apiErr.Code < 400 || apiErr.Code >= 500 {
					recordTiming(timingKey, tl, "failed")
					recordFailure(state, item, "应用优惠码暂时失败: "+ovh.Explain(err))
					return Outcome{}
				}
				// 明确被拒绝才回退;网络故障不能当作优惠码无效。
				if item.AutoPay {
					errMsg := fmt.Sprintf("优惠码被 OVH 拒绝，为防止按原价扣款已暂停下单。原因: %s", ovh.Explain(err))
					state.Logger.Warn(errMsg, "purchase")
					recordFailure(state, item, errMsg)
					notify.Broadcast(state, fmt.Sprintf("⚠️ 抢购 %s@%s 优惠码被拒绝\n\n为防自动按原价扣款，任务已暂停。\n原因：%s", item.PlanCode, item.Datacenter, ovh.Explain(err)), nil)
					return Outcome{Attempted: false, PauseTask: true, Reason: errMsg}
				}
				couponWarning = "优惠码被 OVH 拒绝，本次未使用优惠码，价格不包含该码优惠。原因: " + ovh.Explain(err)
				state.Logger.Warn(couponWarning, "purchase")
			} else {
				state.Logger.Info("购物车优惠码已应用", "purchase")
			}
		}

		// 预订:结账前逐项核对 OVH 实际存下的购物车 —— 型号、每个选项、全部购买周期。
		// 放在最后一次改车(优惠码)之后;开售前后 OVH 可能悄悄补上默认配件或换掉选项,
		// 不核对就会下到另一套硬件。
		if item.Preorder {
			expected := append([]string{item.PlanCode}, orderedOptions...)
			if err := orderterm.VerifyCart(ctx, client, cartID, itemID, selectedDuration, expected...); err != nil {
				if ctx.Err() != nil {
					return cancelledOutcome(state, item, "核对购物车")
				}
				return preorderWait(state, item, err.Error())
			}
			tl.mark("核对购物车")
		}
	}
	// 直接结账 —— 跳过 /summary(它只是日志用的价格,2 秒开销),
	// 价格 + 过期时间下面 checkout 成功后用 /me/order 异步补,不阻塞主流程。
	// 结账前最后一次复核。这是整条链路里唯一真正花钱的一步,也是取消能起作用的
	// 最后一个点 —— 从这里往下,结账请求一旦发出就**不再接受取消**:
	// 请求已经到了 OVH、我们这边却中途断开,订单可能已经生成而我们不知道,
	// 既没 history 也没通知,那比"多下了一单"更糟。
	if ctx.Err() != nil || state.IsTaskDeleted(item.ID) || state.IsTaskPaused(item.ID) {
		return cancelledOutcome(state, item, "结账前复核")
	}
	state.Logger.Info("对购物车 "+cartID+" 执行结账", "purchase")
	var checkoutResult map[string]interface{}
	checkoutPayload := map[string]interface{}{
		// 用户在任务上显式打开了"自动付款"才为 true(schema 描述:
		// "order will be automatically paid with preferred payment method",
		// 需要 OVH 账户已设置默认支付方式)。默认 false:不替用户扣钱。
		// 自动付款已停用，保留原设置：
		// "autoPayWithPreferredPaymentMethod": item.AutoPay,
		"autoPayWithPreferredPaymentMethod": false,
	}
	// 不发 waiveRetractationPeriod。
	//
	// 这个字段的意思是"放弃 14 天无理由撤回权"(官方定义:order will be processed
	// with waiving retractation period)。它在 schema 里是 required:false,
	// 以前这里写死 true —— 每一单都替用户把这个权利交出去,而旁边的 autoPay
	// 明明是跟着用户开关走的,说明当时对"不能替用户做花钱决定"是有意识的。
	//
	// 不传 = 不主动弃权。这不影响拿到机器的速度:checkout 只是创建订单
	// (进 notPaid),真正开通要等付款,卡点从来不是这个字段。
	// 而且方向上是可恢复的 —— 真需要弃权还有 POST /me/order/{id}/waiveRetraction
	// 可以事后补;反过来结账时一旦 true,权利当场消失,没有任何接口能拿回来。

	if err := saveCheckoutJournalForItem(ctx, item, cartID); err != nil {
		return Outcome{PauseTask: true, Reason: "结账凭据保存失败，未提交订单：" + err.Error()}
	}
	checkoutErr := client.PostWithContext(context.WithoutCancel(ctx), "/order/cart/"+cartID+"/checkout", checkoutPayload, &checkoutResult)
	if checkoutErr != nil && prewarmed != nil && unassignedCartError(checkoutErr) {
		state.Logger.Warn(fmt.Sprintf("预热购物车 %s 结账提示尚未绑定账户，自动补发 /assign 后重试结账", cartID), "purchase")
		if assignRetryErr := client.PostWithContext(context.WithoutCancel(ctx), "/order/cart/"+cartID+"/assign", nil, nil); assignRetryErr == nil {
			checkoutResult = nil
			checkoutErr = client.PostWithContext(context.WithoutCancel(ctx), "/order/cart/"+cartID+"/checkout", checkoutPayload, &checkoutResult)
		}
	}
	if err := checkoutErr; err != nil {
		// POST /item/{id}/configuration 对取值不做任何校验 —— 实测在 EU 车上把
		// dedicated_datacenter 设成 "hil"、region 设成 "usa" 都会 200 返回配置项 id,
		// 直到 summary/checkout 才报 "<fqn> is not available in hil"。
		// 也就是说"配置全设成功"根本不代表值合法,错的机房/区域只能在这里现原形,
		// 必须把它翻成用户看得懂的话,否则历史里只有一句英文 OVH 报错。
		// 用 Explain 而不是 err.Error():这条是用户在「购买历史」里唯一能看到的说明。
		// Explain 把原文原样嵌在后面,所以下面那句 Contains 判断照样成立。
		errMsg := ovh.Explain(err)
		if strings.Contains(errMsg, "is not available in") {
			errMsg = fmt.Sprintf("%s（机房 %s 不在子公司 %s 的 %s 目录里；OVH 三站目录独立，同一机型在不同区可选的机房不同）",
				errMsg, apiDC, subsidiary, item.PlanCode)
		}
		tl.mark("下单")
		recordTiming(timingKey, tl, "failed")
		state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误: %s (%s)", item.PlanCode, errMsg, tl.String()), "purchase")
		if couponWarning != "" {
			errMsg += "；" + couponWarning
		}

		// 用的是预热车,而 OVH 明确说这辆车已经不存在 / 已过期 / 未绑定:订单肯定没建(确定性的 4xx),
		// 问题只在车本身。本轮立即不用预热车重走一遍(只重走一次,不会循环)。
		if prewarmed != nil && !strings.HasPrefix(item.ID, "catalog-watch-") && (staleCartError(err) || unassignedCartError(err)) {
			state.Logger.Warn(fmt.Sprintf("预热购物车 %s 在 OVH 侧不可用(%s),本轮立即重新装车下单", cartID, errMsg), "purchase")
			ovh.ReleaseCart(cartID)
			return PurchaseServer(context.WithValue(ctx, skipPrewarmKey{}, true), state, item)
		}

		// checkout 超时/传输层失败 ≠ 下单失败:请求可能已经到达 OVH 且订单已建,
		// 只是我方没等到响应。这时任务会重试 → 新车再 checkout → **买两台**
		// (autoPay 开着就是重复扣款)。所以 transient 失败先按官方 GET /me/order 对账
		// (long[] + date.from,见 ReconcileRecentOrder),命中就按成功处理。
		var checkoutAPIError *ovhsdk.APIError
		definiteRejection := errors.As(err, &checkoutAPIError) && checkoutAPIError.Code >= 400 && checkoutAPIError.Code < 500 && checkoutAPIError.Code != 408 && checkoutAPIError.Code != 429
		if IsTransient(err) || (strings.HasPrefix(item.ID, "catalog-watch-") && !definiteRejection) {
			// Concurrent catalog jobs may share a planCode. A fuzzy recent-order
			// match cannot prove which configuration/location submitted that order.
			// Preserve uncertainty rather than attributing another worker's order.
			if strings.HasPrefix(item.ID, "catalog-watch-") {
				success = true // Retain the original cart for read-only automatic reconciliation.
				return Outcome{PauseTask: true, CheckoutUncertain: true, Reason: "结账响应不确定，后台将自动核对原购物车；确认前不重复提交"}
			}
			oid, orderURL, recErr := reconcileAndClaim(state, client, acc, item, attemptStart, couponWarning)
			if recErr != nil {
				state.Logger.Warn("checkout 对账查询失败(按未建单处理): "+recErr.Error(), "purchase")
			} else if oid != "" {
				state.Logger.Warn(fmt.Sprintf("checkout 报 transient 失败但对账发现订单已建(#%s),按成功处理,不再重试", oid), "purchase")
				success = true
				ovh.ReleaseCart(cartID)
				return handleReconciledOrder(state, client, acc.Endpoint, item, oid, orderURL, tl, timingKey, couponWarning)
			}
			// 对账也未果(429/超时/明细读失败):OVH 侧订单状态未知。
			// 此处不调用 recordFailure(避免在未决状态下写入假失败记录烧掉重试预算),
			// 而是标记暂停任务并记录 UncertainCheckout, 等待人工确认或在下次恢复运行时优先对账。
			state.Logger.Warn("checkout transient 失败且对账未能确认,暂停任务等待人工确认(防重复下单)", "purchase")
			notify.Broadcast(state, fmt.Sprintf(
				"⚠️ 抢购 %s@%s 结账超时,无法确认订单是否已创建\n\n"+
					"任务已暂停。请到 OVH 订单页查看是否已有一笔未付订单:\n"+
					"- 有:等付款即可,任务不需要恢复\n"+
					"- 没有:在队列页把任务恢复运行", item.PlanCode, item.Datacenter), nil)
			success = true // 结账结果未决时禁止立即 DELETE 原购物车,释放进程内引用(cart_janitor 只删非只读车)
			ovh.ReleaseCart(cartID)
			return Outcome{Attempted: false, PauseTask: true, CheckoutUncertain: true, Reason: "结账超时且对账未能确认是否已建单"}
		}
		if item.Preorder && listingNotReady(err) {
			// 明确拒绝(400/404/409),不是超时:订单肯定没建。开售前结账被拒属于"还没开放"
			state.Logger.Info("预订下单接口暂未接受所选配置，继续等待: "+errMsg, "purchase")
			return Outcome{WaitingForCatalog: true, Reason: errMsg}
		}
		recordFailure(state, item, errMsg)
		// checkout 这一步最要紧:补货瞬间大家都在下单,429 是常态。
		// 把它记成一次"真正的失败尝试"会让任务在唯一有货的那一分钟里自己判死。
		return attemptOutcome(err)
	}
	tl.mark("下单")
	recordTiming(timingKey, tl, "ordered")

	orderID := numconv.ToString(checkoutResult["orderId"])
	orderURL, _ := checkoutResult["url"].(string)
	if strings.HasPrefix(item.ID, "catalog-watch-") && (orderID == "" || orderID == "0") {
		success = true
		return Outcome{PauseTask: true, CheckoutUncertain: true, Reason: "结账未返回有效订单编号，后台自动核对"}
	}

	// checkout 已返回订单 ID,cart 已成功转 order,标记成功阻止 defer 删除并立即释放活跃购物车注册表
	success = true
	ovh.ReleaseCart(cartID)
	openPreorderGate(gateKey)

	// Persist the returned identity before history/notifications; a restart can
	// query this exact order without relying on cart retention upstream.
	if err := saveCheckoutJournalForItem(ctx, item, cartID, orderID); err != nil {
		state.Logger.Error("订单已创建但确认凭据保存失败: "+err.Error(), "purchase")
	}
	// 立刻记成功 —— 价格和过期时间空着,后台异步补。
	// 预订记下实际下单的 addon(库存段已映射成销售选项),历史里看到的就是真实买到的配置。
	orderedItem := item
	if item.Preorder && len(orderedOptions) > 0 {
		copied := *item
		copied.Options = append([]string(nil), orderedOptions...)
		orderedItem = &copied
	}
	recordSuccess(state, orderedItem, orderID, orderURL, "", nil, couponWarning)
	recordTimingToHistory(state, item.ID, tl)
	state.Logger.Info("[耗时] "+item.PlanCode+"@"+item.Datacenter+" 抢购成功 "+tl.String(), "purchase")

	// 异步补:从 /me/order/{orderID} 读 expirationDate + 价格,写回 history
	if orderID != "" {
		startBackfillOrderDetail(state, client, item.ID, orderID)
	}

	state.Logger.Info(fmt.Sprintf("成功购买 %s 在 %s (订单ID: %s, URL: %s)",
		item.PlanCode, item.Datacenter, orderID, orderURL), "purchase")

	// 成功通知走统一通知层(Telegram + Webhook 至少一条可达就发)。
	// 以前被 TG 配置判断包住:Webhook-only 用户收不到"抢到了但没付款",
	// 订单逾期作废 —— 这是整条链路里最不能丢的一条消息(issue #2)。
	payURL := ovh.ManagerOrderURL(acc.Endpoint, orderID)
	// 云下单:绑定了进度卡片的任务把卡片改成"锁单成功"+支付按钮;统一通知照发,不能丢
	extra := map[string]string{
		"orderId":  orderID,
		"orderUrl": payURL,
	}
	if tl != nil {
		extra["timing"] = tl.FormatMetrics()
	}
	telegram.NotifyTaskProgress(state, item, "success", extra)
	message := BuildOrderSuccessMessage(orderedItem, orderID, payURL, tl)
	if couponWarning != "" {
		message += "\n\n⚠️ " + couponWarning
	}
	if notify.Broadcast(state, message, nil) > 0 {
		state.Logger.Info("已为订单 "+orderID+" 发送订单成功通知。", "purchase")
	} else {
		state.Logger.Warn("订单 "+orderID+" 已创建,但成功通知未送达任何通道。", "purchase")
	}
	return Outcome{Success: true}
}
