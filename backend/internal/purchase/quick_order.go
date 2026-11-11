package purchase

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/price"
	"github.com/ovh-webui/server/internal/types"
)

// quickOrderMu 串行化 quick-order 入队的逻辑,避免并发同 plan@dc 重复入队
var quickOrderMu sync.Mutex

// QuickOrderParams 快速下单入参
type QuickOrderParams struct {
	AccountID          string   `json:"account_id"` // 必填,哪个账户下单
	AccountIDCamel     string   `json:"accountId"`  // 兼容前端 camelCase
	PlanCode           string   `json:"planCode"`
	Datacenter         string   `json:"datacenter"`
	Options            []string `json:"options"`
	FromMonitor        bool     `json:"fromMonitor"`
	SkipDuplicateCheck bool     `json:"skipDuplicateCheck"`
	AutoPay            bool     `json:"autoPay"`
	Force              bool     `json:"force"`
	Quantity           int      `json:"quantity"`
	// Internal monitor generation guard; external HTTP callers cannot supply it.
	GuardEnqueue func(func() error) error `json:"-"`
	Coupon       string                   `json:"coupon"`
	// Duration 购买时长(P1M/P12M...),空 = 1 个月
	Duration string `json:"duration"`
}

// QuickOrderResult 快速下单执行结果
type QuickOrderResult struct {
	Success    bool             `json:"success"`
	Message    string           `json:"message,omitempty"`
	Error      string           `json:"error,omitempty"`
	Code       string           `json:"code,omitempty"`
	Price      *price.PriceInfo `json:"price,omitempty"`
	Options    []string         `json:"options,omitempty"`
	Item       *types.QueueItem `json:"item,omitempty"`
	HTTPStatus int              `json:"-"`
}

// EnqueueQuickOrder 执行快速下单验证与入队逻辑。
// 供 HTTP Handler (handlers.QuickOrder) 以及进程内监控 (monitor.batchOrder) 直接调用。
func EnqueueQuickOrder(state *app.State, params QuickOrderParams) *QuickOrderResult {
	params.PlanCode = strings.TrimSpace(params.PlanCode)
	params.Datacenter = strings.TrimSpace(params.Datacenter)
	if params.AccountID == "" && params.AccountIDCamel != "" {
		params.AccountID = params.AccountIDCamel
	}
	if params.PlanCode == "" || params.Datacenter == "" || strings.ContainsAny(params.PlanCode, ",，、 \t\r\n") {
		return &QuickOrderResult{
			Success:    false,
			Error:      "缺少 planCode 或 datacenter",
			Code:       "E75DA4B08",
			HTTPStatus: http.StatusOK,
		}
	}
	duration, termErr := orderterm.Normalize(params.Duration)
	if termErr != nil {
		return &QuickOrderResult{Success: false, Error: termErr.Error(), HTTPStatus: http.StatusBadRequest}
	}
	params.Duration = duration
	if params.AccountID == "" {
		return &QuickOrderResult{
			Success:    false,
			Error:      "缺少 account_id",
			Code:       "E34CBF1D4",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	if _, ok := state.FindAccount(params.AccountID); !ok {
		return &QuickOrderResult{
			Success:    false,
			Error:      "account_id 不存在",
			Code:       "E7B315B13",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	options := params.Options
	// Explicit custom plans are validated by the live cart API below. A stale
	// public catalog must not prevent that validation; monitor callers have
	// already checked availability and pricing for this exact account.
	if len(options) == 0 && !params.Force {
		availByConfig := catalog.CheckServerAvailabilityWithConfigs(state, params.PlanCode, params.AccountID)
		// cfg.Datacenters 的 key 是 OVH 原样返回的 AvailabilityDatacenterEnum(孟买是 ynm),
		// 而 params.Datacenter 是前端显示码(孟买是 mum),必须和 purchase / price 一样先转换
		dcKey := ovh.ConvertDisplayDCToAPIDC(params.Datacenter)
		// ConfigAvailability 用三个字段表达三种完全不同的"没配上":
		//   CatalogError + CatalogMissing=false → OVH 目录拉取失败(网络/429),重试有用
		//   CatalogMissing=true                 → 目录拉到了,但本子公司目录没这个 planCode,
		//                                         此时 CatalogError 里已经是完整的人话说明
		//   OptionsNote                         → 目录和 plan 都在,只是这套内存/存储在
		//                                         本子公司买不到(实测 US 35 条、EU/CA 各 18 条)
		catalogErr := ""
		catalogMissingMsg := ""
		optionsNote := ""
		dcSeen := false
		for _, cfg := range availByConfig {
			if cfg.CatalogMissing && catalogMissingMsg == "" {
				catalogMissingMsg = cfg.CatalogError
			} else if cfg.CatalogError != "" && !cfg.CatalogMissing && catalogErr == "" {
				catalogErr = cfg.CatalogError
			}
			dcStatus, ok := cfg.Datacenters[dcKey]
			if !ok || !catalog.IsAvailableForOrder(dcStatus) {
				continue
			}
			dcSeen = true
			if len(cfg.Options) > 0 {
				options = append(options, cfg.Options...)
				break
			}
			if cfg.OptionsNote != "" && optionsNote == "" {
				optionsNote = cfg.OptionsNote
			}
		}
		if len(options) == 0 {
			err := "指定机房无可定价配置（" + params.PlanCode + "@" + params.Datacenter + "）"
			switch {
			case catalogMissingMsg != "":
				err = catalogMissingMsg
			case catalogErr != "":
				err = "OVH 目录拉取失败，无法匹配可下单配置：" + catalogErr
			case optionsNote != "":
				err = optionsNote
			case len(availByConfig) == 0:
				if _, hint := catalog.ClassifyPlan(state, params.AccountID, params.PlanCode, "quick_order"); hint != "" {
					err = hint
				}
			case !dcSeen:
				err = "机型 " + params.PlanCode + " 在机房 " + params.Datacenter + " 当前无货"
			}
			state.Logger.Warn("[quick_order] "+err, "quick_order")
			return &QuickOrderResult{
				Success:    false,
				Error:      err,
				HTTPStatus: http.StatusBadRequest,
			}
		}
	}

	// 去重:防止同一 plan@dc + 同 options 的任务被重复入队(除非监控来源 + 显式跳过)。
	// 询价前先查一次(快速拒绝,不白建车),询价后在 quickOrderMu 内再查一次并入队。
	// 以前整个函数都在 quickOrderMu 里,手动下单的实时询价(建车删车 3~6 秒,超时可到 30 秒)
	// 会把这段时间里监控为每个机房发起的自动入队全部挡在锁外 —— 补货窗口往往就这几十秒。
	// 机房比较按 API 码:同一机房可能一边是显示码(mum)一边是 API 码(ynm)
	sameDC := func(a, b string) bool {
		return ovh.ConvertDisplayDCToAPIDC(a) == ovh.ConvertDisplayDCToAPIDC(b)
	}
	dupCheck := func() *QuickOrderResult {
		// 监控跳过的只是"队列里已有同配置任务"检查 —— 它在补货窗口内本来就该能重新入队。
		// 但"120 秒内同配置已成功下过单"这道闸门对监控同样生效:监控的 lastStatus 在
		// 验价 429 抖动时会走 price_check_failed → available 的来回,状态机表达不了
		// "这个窗口已经买过",唯一能表达的就是近期成功史,跳过它 = 同一窗口重复下单
		if params.FromMonitor && params.SkipDuplicateCheck {
			fp0 := fingerprint(options)
			nowTS0 := time.Now().Unix()
			state.HistoryMu.Lock()
			for i := len(state.History) - 1; i >= 0; i-- {
				h := state.History[i]
				if (h.AccountID == "" || h.AccountID == params.AccountID) && h.PlanCode == params.PlanCode && sameDC(h.Datacenter, params.Datacenter) && h.Status == "success" &&
					fingerprint(h.Options) == fp0 {
					if t, ok := types.ParseTS(h.PurchaseTime); ok && nowTS0-t.Unix() < 120 {
						state.HistoryMu.Unlock()
						state.Logger.Info("监控来源:近期已成功下过同配置订单,拒绝(防同窗口重复下单)", "quick_order")
						return &QuickOrderResult{
							Success:    false,
							Error:      "该配置刚刚已成功下单(120 秒内),不再重复下单",
							Code:       "EF4FA206C",
							HTTPStatus: http.StatusTooManyRequests,
						}
					}
				}
			}
			state.HistoryMu.Unlock()
		}
		if !(params.FromMonitor && params.SkipDuplicateCheck) {
			fp := fingerprint(options)
			state.QueueMu.Lock()
			for _, it := range state.Queue {
				if (it.AccountID == "" || it.AccountID == params.AccountID) && it.PlanCode == params.PlanCode && sameDC(it.Datacenter, params.Datacenter) &&
					(it.Status == "running" || it.Status == "pending" || it.Status == "paused") &&
					fingerprint(it.Options) == fp {
					state.QueueMu.Unlock()
					state.Logger.Info("检测到重复的队列任务（含配置），拒绝再次入队", "quick_order")
					return &QuickOrderResult{
						Success:    false,
						Error:      "已存在相同配置的购买任务，稍后再试",
						Code:       "EE77AF09E",
						HTTPStatus: http.StatusTooManyRequests,
					}
				}
			}
			state.QueueMu.Unlock()

			nowTS := time.Now().Unix()
			state.HistoryMu.Lock()
			for i := len(state.History) - 1; i >= 0; i-- {
				h := state.History[i]
				if (h.AccountID == "" || h.AccountID == params.AccountID) && h.PlanCode == params.PlanCode && sameDC(h.Datacenter, params.Datacenter) && h.Status == "success" &&
					fingerprint(h.Options) == fp {
					// PurchaseTime 是 types.NowISO() 写的,不带时区 —— 用 RFC3339Nano
					// 解必然失败,于是这道"刚刚已经买成过同款,别再买一次"的闸门
					// 一直是死代码。它拦的是重复扣款,不是显示问题。
					if t, ok := types.ParseTS(h.PurchaseTime); ok {
						if nowTS-t.Unix() < 120 {
							state.HistoryMu.Unlock()
							state.Logger.Info("检测到近期成功订单，拒绝再次入队", "quick_order")
							return &QuickOrderResult{
								Success:    false,
								Error:      "刚刚已成功下过同配置订单，稍后再试",
								Code:       "EF4FA206C",
								HTTPStatus: http.StatusTooManyRequests,
							}
						}
					}
				}
			}
			state.HistoryMu.Unlock()
		}
		return nil
	}
	if r := dupCheck(); r != nil {
		return r
	}
	if params.FromMonitor && params.SkipDuplicateCheck {
		state.Logger.Info("来自监控的批量下单，跳过重复检查", "quick_order")
	}

	var priceInfo *price.PriceInfo
	if params.FromMonitor && HasPrewarmedCart(params.AccountID, params.PlanCode, params.Datacenter, options, params.Coupon, params.Duration) {
		// 命中预热就绪车：跳过耗时建车查价，直接免检入队，由后台 worker 立即唤醒直出 Checkout！
		state.Logger.Info(fmt.Sprintf("[quick_order] %s@%s 命中预热池有效车，免检极速入队", params.PlanCode, params.Datacenter), "quick_order")
	} else if params.FromMonitor {
		// 监控批量入队路径:
		// 监控已在 check 阶段做过可用性探测，在此切忌重新发起完整的建车删车周期 (消耗 3~6s 且易踩 429 导致任务被拦截弃单)。
		// 仅尝试从短 TTL 内存缓存获取价格；若无缓存或失败，不阻塞任务直接入队抢购。
		priceResult := price.GetInternalCached(state, params.AccountID, params.PlanCode, params.Datacenter, options, params.Duration)
		if priceResult.Success && priceResult.Price != nil && !priceResult.Degraded {
			priceInfo = priceResult.Price
		} else {
			state.Logger.Info("监控来源快速下单: 跳过同步建车询价阻塞, 优先抢占入队", "quick_order")
		}
	} else {
		// 手动下单: 走新鲜的真实询价, 校验配置合法性与价格
		priceResult := price.GetInternal(state, params.AccountID, params.PlanCode, params.Datacenter, options, params.Duration)
		if !priceResult.Success {
			err := priceResult.Error
			if err == "" {
				err = "价格查询失败"
			}
			state.Logger.Warn("快速下单前价格校验失败: "+params.PlanCode+"@"+params.Datacenter+" - "+err, "quick_order")
			return &QuickOrderResult{
				Success:    false,
				Error:      "价格校验失败：" + err,
				HTTPStatus: http.StatusBadRequest,
			}
		}
		if priceResult.Degraded {
			state.Logger.Warn("快速下单前价格校验降级，拒绝入队: "+priceResult.DegradedReason, "quick_order")
			return &QuickOrderResult{
				Success:    false,
				Error:      "购物车配置不完整，暂不支持下单：" + priceResult.DegradedReason,
				HTTPStatus: http.StatusBadRequest,
			}
		}
		if priceResult.Price == nil {
			state.Logger.Warn("快速下单前价格校验失败: price字段缺失", "quick_order")
			return &QuickOrderResult{
				Success:    false,
				Error:      "价格查询返回数据格式异常：缺少price字段",
				Code:       "EBCF58A32",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		priceRaw := priceResult.Price.Prices["withoutTax"]
		if priceRaw == nil {
			priceRaw = priceResult.Price.Prices["withTax"]
		}
		if priceRaw == nil {
			state.Logger.Warn("快速下单前价格缺失或无效: "+params.PlanCode+"@"+params.Datacenter, "quick_order")
			return &QuickOrderResult{
				Success:    false,
				Error:      "该组合暂无有效价格，暂不支持下单",
				Code:       "EE94F16EB",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		if f, ok := numconv.ToFloat64(priceRaw); ok && f == 0 {
			state.Logger.Warn("快速下单前价格缺失或无效: "+params.PlanCode+"@"+params.Datacenter, "quick_order")
			return &QuickOrderResult{
				Success:    false,
				Error:      "该组合暂无有效价格，暂不支持下单",
				Code:       "EE94F16EB",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		priceInfo = priceResult.Price
	}

	now := types.NowISO()
	item := types.QueueItem{
		ID:         uuid.NewString(),
		Duration:   params.Duration,
		AccountID:  params.AccountID,
		PlanCode:   params.PlanCode,
		Datacenter: params.Datacenter,
		Options:    options,
		Status:     "running",
		RetryCount: 0,
		AutoPay:    params.AutoPay,
		Force:      params.Force,
		Coupon:     strings.TrimSpace(params.Coupon),
		// MaxRetries 封顶的是**真正提交并失败的次数**(FailureCount),无货轮次不计。
		MaxRetries:    20,
		RetryInterval: state.Config.QuickOrderRetryInterval(),
		CreatedAt:     now,
		UpdatedAt:     now,
		LastCheckTime: 0,
		QuickOrder:    true,
		Priority:      100,
	}

	// 落库与入队。监控触发的走 EnqueueAutomaticItems:用户暂停/删除过的同账户/型号/机房、
	// 已暂停的订阅或全局停止的监控,都不允许被监控重新建单。
	enqueue := state.EnqueueItems
	if params.FromMonitor {
		enqueue = state.EnqueueAutomaticItems
	}
	quantity := params.Quantity
	if quantity < 1 {
		quantity = 1
	}
	if quantity > types.MaxOrderQuantity {
		quantity = types.MaxOrderQuantity
	}
	// One atomic batch keeps the requested quantity together. Calling the
	// automatic enqueue path repeatedly would reject item two as a duplicate.
	items := make([]types.QueueItem, quantity)
	for i := range items {
		items[i] = item
		items[i].Options = append([]string(nil), options...)
		if i > 0 {
			items[i].ID = uuid.NewString()
		}
	}
	quickOrderMu.Lock()
	defer quickOrderMu.Unlock()
	if r := dupCheck(); r != nil {
		return r
	}
	commit := func() error { return enqueue(items, true) }
	var enqueueErr error
	if params.GuardEnqueue != nil {
		enqueueErr = params.GuardEnqueue(commit)
	} else {
		enqueueErr = commit()
	}
	if err := enqueueErr; err != nil {
		if app.IsRejected(err) {
			// 业务规则拒绝(重复任务 / 监控已停止或暂停 / 队列超上限…),不是存储故障
			state.Logger.Info("任务未入队: "+err.Error(), "queue")
			return &QuickOrderResult{Success: false, Error: err.Error(), Code: "ENQUEUE_REJECTED", HTTPStatus: http.StatusConflict}
		}
		state.Logger.Error("自动下单任务落库失败,已撤回: "+err.Error(), "queue")
		return &QuickOrderResult{
			Success:    false,
			Error:      "任务没能写进数据库，已撤回（避免出现重启就消失的假任务）：" + err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	state.Logger.Info("快速下单: "+params.PlanCode+" ("+params.Datacenter+") 已加入队列", "quick_order")

	return &QuickOrderResult{
		Success:    true,
		Message:    fmt.Sprintf("✅ %s (%s) 已加入购买队列，共 %d 台", params.PlanCode, params.Datacenter, quantity),
		Price:      priceInfo,
		Options:    options,
		Item:       &item,
		HTTPStatus: http.StatusOK,
	}
}

// fingerprint 排序后用 "|" 连接的 options 指纹,用于队列去重
func fingerprint(opts []string) string {
	if len(opts) == 0 {
		return ""
	}
	uniq := map[string]struct{}{}
	for _, o := range opts {
		s := strings.TrimSpace(o)
		if s != "" {
			uniq[s] = struct{}{}
		}
	}
	list := make([]string, 0, len(uniq))
	for s := range uniq {
		list = append(list, s)
	}
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j-1] > list[j]; j-- {
			list[j-1], list[j] = list[j], list[j-1]
		}
	}
	return strings.Join(list, "|")
}
