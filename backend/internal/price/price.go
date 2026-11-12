package price

import (
	"fmt"
	"net/url"
	"strings"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
	"sort"
	"sync"
	"time"
)

// Result 询价结果
type Result struct {
	Success    bool     `json:"success"`
	PlanCode   string   `json:"planCode,omitempty"`
	Datacenter string   `json:"datacenter,omitempty"`
	Options    []string `json:"options,omitempty"`
	// Degraded:价格算出来了,但购物车没有完全配成用户要的样子(某个非机房必填配置没设上)。
	// 价格仍可展示,但不能拿它当"可以下单"的闸门 —— purchase.go 对同样的配置是 fail-fast 的。
	Degraded       bool                   `json:"degraded,omitempty"`
	DegradedReason string                 `json:"degradedReason,omitempty"`
	Price          *PriceInfo             `json:"price"`
	Error          string                 `json:"error,omitempty"`
	Raw            map[string]interface{} `json:"-"`
}

type PriceInfo struct {
	// Duration 本次报价对应的购买时长(P1M/P12M...)
	Duration    string                   `json:"duration,omitempty"`
	PricingMode string                   `json:"pricingMode"`
	Prices      map[string]interface{}   `json:"prices"`
	Items       []map[string]interface{} `json:"items"`
}

// internalPriceCache 短 TTL 询价结果缓存。
// 背景:监控每轮对 orderable 机房全量验价(每个 7-9 个串行 OVH 调用),
// 持续有货的订阅按 5 秒一轮就能把账户打进 429 —— 而 429 一来验价失败,
// 自动下单被闸门挡住,等于"监控自己把下单通道刷死"。
// 询价结果(同一配置同一机房)在 20 秒内不会实质变化,复用它即可把
// 每分钟的完整购物车周期降一个数量级。入队时刻的二次验价(monitor 刚验过
// 一遍、batchOrder 又逐条验)同样命中这里,补货瞬间的突发消失。
var internalPriceCache = struct {
	sync.Mutex
	m map[string]cacheEntry
}{m: make(map[string]cacheEntry)}

type cacheEntry struct {
	at     time.Time
	result Result
}

const priceCacheTTL = 20 * time.Second

func priceCacheKey(accountID, planCode, datacenter string, options []string) string {
	sort.Strings(options)
	return accountID + "|" + planCode + "@" + datacenter + "|" + strings.Join(options, ",")
}

// GetInternalCached 带短 TTL 缓存的 GetInternal(监控轮询/批量入队路径用;
// 用户显式点击的询价仍走无缓存的 GetInternal,保证手查永远新鲜)
func GetInternalCached(state *app.State, accountID, planCode, datacenter string, options []string, duration string) Result {
	r, _ := getInternalCached(state, accountID, planCode, datacenter, options, duration, false)
	return r
}

// GetInternalCachedKeepCart 同 GetInternalCached,但询价成功(且未降级)时不删购物车,
// 把配好的车号一并返回,调用方可直接拿去当预热车结账 —— 监控发现补货时要先验价,
// 这辆车本来就是按下单的配置装好、绑定过的,删掉再重装一辆纯属浪费抢购时间。
// 命中缓存时没有新车,cartID 为空;调用方不用这辆车时必须自己删掉。
func GetInternalCachedKeepCart(state *app.State, accountID, planCode, datacenter string, options []string, duration string) (Result, string) {
	return getInternalCached(state, accountID, planCode, datacenter, options, duration, true)
}

func getInternalCached(state *app.State, accountID, planCode, datacenter string, options []string, duration string, keepCart bool) (Result, string) {
	normalized, err := orderterm.Normalize(duration)
	if err != nil {
		return Result{Success: false, Error: err.Error()}, ""
	}
	// 购买时长必须进缓存键:月付和年付是两张不同的报价,混用会把年付价当月付价显示
	key := normalized + "|" + priceCacheKey(accountID, planCode, datacenter, append([]string{}, options...))
	internalPriceCache.Lock()
	if e, ok := internalPriceCache.m[key]; ok && time.Since(e.at) < priceCacheTTL {
		internalPriceCache.Unlock()
		return e.result, ""
	}
	internalPriceCache.Unlock()
	r, cartID := getInternal(state, accountID, planCode, datacenter, options, normalized, keepCart)
	internalPriceCache.Lock()
	// 防无限增长:超 512 条先清过期
	if len(internalPriceCache.m) > 512 {
		for k, e := range internalPriceCache.m {
			if time.Since(e.at) >= priceCacheTTL {
				delete(internalPriceCache.m, k)
			}
		}
	}
	internalPriceCache.m[key] = cacheEntry{at: time.Now(), result: r}
	internalPriceCache.Unlock()
	return r, cartID
}

// GetInternal 询价。accountID 决定用哪个账户调 OVH(空 = 默认账户),
// 以及购物车走哪个 subsidiary(账户的 zone)。多账户必须区分。
// duration 是购买时长(空 = P1M),报价严格按这个周期出,不会换成别的周期。
func GetInternal(state *app.State, accountID, planCode, datacenter string, options []string, duration string) Result {
	r, _ := getInternal(state, accountID, planCode, datacenter, options, duration, false)
	return r
}

// getInternal keepCart=true 且询价成功、未降级时保留购物车并返回车号,否则车号为空、车已(异步)删除。
func getInternal(state *app.State, accountID, planCode, datacenter string, options []string, duration string, keepCart bool) (Result, string) {
	selectedDuration, termErr := orderterm.Normalize(duration)
	if termErr != nil {
		return Result{Success: false, Error: termErr.Error()}, ""
	}
	if options == nil {
		options = []string{}
	}
	apiDC := ovh.ConvertDisplayDCToAPIDC(datacenter)

	client, err := state.OVH.IsolatedClientFor(accountID)
	if err != nil {
		return Result{Success: false, Error: "未配置OVH API密钥: " + ovh.Explain(err)}, ""
	}
	acc, _ := state.FindAccount(accountID)
	subsidiary := orderSubsidiary(state, acc)

	state.Logger.Info(fmt.Sprintf("查询 %s 的配置价格，数据中心: %s (原始: %s), 选项: %v",
		planCode, apiDC, datacenter, options), "price")

	cartID := ""

	kept := false
	// 删车不阻塞返回:询价结果已经拿到了,删车那一次往返没必要算进验价耗时。
	// defer 也兜住中间步骤 panic(map 断言 / 空指针等)时的 cart 泄漏。
	defer func() {
		if cartID == "" || kept {
			return
		}
		id := cartID
		go func() {
			_ = client.Delete("/order/cart/"+id, nil)
			ovh.ReleaseCart(id) // 删失败也撤销登记,交给遗留车清理兜底
		}()
	}()

	// 1. 创建购物车
	var cartResult map[string]interface{}
	if err := client.Post("/order/cart", ovh.CartCreateBody(subsidiary), &cartResult); err != nil {
		return Result{Success: false, Error: ovh.Explain(err)}, ""
	}
	cartID, _ = cartResult["cartId"].(string)
	ovh.TrackCart(cartID)
	state.Logger.Debug("购物车创建成功，ID: "+cartID, "price")

	// 2. 并发:加购基础商品 ‖ 绑定购物车 ‖ 拉 Eco 选项列表 —— 三者都只依赖 cartId。
	// 以前整条询价全串行(十来次往返、约 4 秒),而监控发现补货后要先验价才发通知、才自动下单,
	// 这段时间直接算在抢购延迟里。
	hwOptions := filterHardwareOptions(state, options)
	baseDuration, basePricingMode := selectedDuration, "default"
	var itemResult map[string]interface{}
	var baseErr error
	var baseFail *Result
	var availableOpts []map[string]interface{}
	var optsErr error
	// 并发请求各用一个隔离 client:go-ovh 每次请求都会写 http.Client.Timeout,
	// 共享同一实例并发调用就是数据竞争。有了 /auth/time 时差缓存,多建几个不多花网络往返。
	fork := func() *ovhsdk.Client {
		if c, err := state.OVH.IsolatedClientFor(accountID); err == nil {
			return c
		}
		return client
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func(client *ovhsdk.Client) {
		defer wg.Done()
		// schema 里 POST /order/cart/{cartId}/assign 只有 path 参数、没有 body
		if err := client.Post("/order/cart/"+cartID+"/assign", nil, nil); err != nil {
			state.Logger.Warn("绑定购物车失败（可能不需要）: "+err.Error(), "price")
		}
	}(fork())
	if len(hwOptions) > 0 {
		wg.Add(1)
		go func(client *ovhsdk.Client) {
			defer wg.Done()
			q := url.Values{}
			q.Set("planCode", planCode)
			optsErr = client.Get(fmt.Sprintf("/order/cart/%s/eco/options?%s", cartID, q.Encode()), &availableOpts)
		}(fork())
	}
	wg.Add(1)
	go func(client *ovhsdk.Client) {
		defer wg.Done()
		// 月付走零额外请求的快速路径;其他周期先查 OVH 给这个型号的真实计价
		if selectedDuration != orderterm.Default {
			d, pm, found := lookupEcoPricing(state, client, cartID, planCode, selectedDuration)
			if !found {
				baseFail = &Result{Success: false, Error: fmt.Sprintf("OVH 不支持 %s 按 %s 购买", planCode, orderterm.Label(selectedDuration))}
				return
			}
			baseDuration, basePricingMode = d, pm
		}
		postBase := func() error {
			itemResult = nil
			return client.Post("/order/cart/"+cartID+"/eco", map[string]interface{}{
				"planCode":    planCode,
				"pricingMode": basePricingMode,
				"duration":    baseDuration,
				"quantity":    1,
			}, &itemResult)
		}
		if err := postBase(); err != nil {
			if d, pm, found := lookupEcoPricing(state, client, cartID, planCode, baseDuration); found &&
				(d != baseDuration || pm != basePricingMode) {
				state.Logger.Warn(fmt.Sprintf("以 %s/%s 加购 %s 失败(%s)，改用目录计价 %s/%s 重试",
					baseDuration, basePricingMode, planCode, ovh.Explain(err), d, pm), "price")
				baseDuration, basePricingMode = d, pm
				err = postBase()
			}
			baseErr = err
		}
	}(fork())
	wg.Wait()
	if baseFail != nil {
		return *baseFail, ""
	}
	if baseErr != nil {
		msg := ovh.Explain(baseErr)
		if strings.Contains(msg, "is not available in") {
			state.Logger.Warn("配置在指定数据中心不可用: "+msg, "price")
			return Result{Success: false, Error: "该配置在指定数据中心不可用"}, ""
		}
		if strings.Contains(msg, "Plan code not found") {
			state.Logger.Warn(fmt.Sprintf("planCode %s 不在子公司 %s 的目录里: %s", planCode, subsidiary, msg), "price")
			return Result{Success: false, Error: fmt.Sprintf(
				"机型 %s 不在该账户所属子公司 %s（%s 区）的目录里：OVH 的 EU / US / CA 三个站点目录互不相通，"+
					"美区机型带 -us / -eu / -ca 后缀，欧区和加区不带。请改用本区的 planCode，或换一个同区账户询价。",
				planCode, subsidiary, ovh.SubsidiaryRegion(subsidiary))}, ""
		}
		return Result{Success: false, Error: msg}, ""
	}
	itemID, _ := numconv.ToInt64(itemResult["itemId"])
	if itemID == 0 {
		return Result{Success: false, Error: fmt.Sprintf("无法从购物车响应中解析 itemId（响应: %v）", itemResult)}, ""
	}
	state.Logger.Debug(fmt.Sprintf("基础商品添加成功，项目 ID: %d", itemID), "price")

	region, regionSrc := catalog.ResolveRegion(state, accountID, planCode, apiDC)
	if regionSrc == "fallback" {
		// ResolveRegion 的兜底分支用的是账户原始 zone,zone 为空时(美区账户很常见)
		// 会算成 europe/canada,而美区目录里 143 个 plan 的 region 只有 united_states。
		// 用已归一化的 subsidiary 重算,和 purchase.go 保持同一口径。
		// 走 catalog.FallbackRegion:它会先把 ca-east-tor-a 这类长机房码归一成城市码,
		// 而 ovh.RegionForDCInSubsidiary 只认短码、遇到长码返回空串。
		if r := catalog.FallbackRegion(apiDC, subsidiary); r != region {
			state.Logger.Warn(fmt.Sprintf("目录兜底给出的 region %q 与子公司 %s 不符，改用 %q",
				region, subsidiary, r), "price")
			region, regionSrc = r, "fallback(subsidiary-corrected)"
		}
	}
	degraded := false
	degradedReason := ""
	if region == "" {
		// region 在三区目录里都是 isMandatory:true(IE/CA/US 三份公开目录逐个 plan 核对过),
		// 所以"推不出 region"不是"这个 plan 不需要 region",而是配不全车。
		// 先问购物车自己 —— requiredConfiguration 三区实测都带 allowedValues,
		// 是按本车子公司算出来的本区正确值(US → ["united_states"],EU/CA → ["canada","europe"])。
		var required []map[string]interface{}
		if err := client.Get(fmt.Sprintf("/order/cart/%s/item/%d/requiredConfiguration", cartID, itemID), &required); err != nil {
			state.Logger.Warn("获取必需配置失败: "+err.Error(), "price")
		} else {
			allowed, mandatory := regionAllowedValues(required)
			if v := pickAllowedRegion(allowed, catalog.FallbackRegion(apiDC, subsidiary)); v != "" {
				region, regionSrc = v, "requiredConfiguration"
			} else if mandatory {
				// 标 degraded 而不是硬失败:价格照样算得出来,但 purchase.go 对同样的情况是
				// fail-fast,quick-order 的闸门看 Degraded 就会拒绝入队,两边口径一致。
				degraded = true
				degradedReason = "必填配置 region 无法确定：子公司 " + subsidiary + " 的目录拉不到，购物车也没给出合法取值"
				state.Logger.Warn(degradedReason, "price")
			}
		}
	}
	if region == "" {
		state.Logger.Info(fmt.Sprintf("%s@%s 无需 region 配置(来源: %s)", planCode, apiDC, regionSrc), "price")
	} else {
		state.Logger.Debug(fmt.Sprintf("%s@%s 的 region = %s(来源: %s)", planCode, apiDC, region, regionSrc), "price")
	}
	type kv struct{ label, value string }
	configurations := []kv{
		{"dedicated_datacenter", apiDC},
		{"dedicated_os", "none_64.en"},
	}
	if region != "" {
		configurations = append(configurations, kv{"region", region})
	}

	// 3. 硬件选项在本地匹配好(选项列表上一步已并发拉回)
	type optWrite struct {
		planCode string
		body     map[string]interface{}
	}
	var optWrites []optWrite
	if len(hwOptions) > 0 {
		if optsErr != nil {
			state.Logger.Error("获取 Eco 选项列表失败: "+optsErr.Error(), "price")
			return Result{Success: false, Error: "获取硬件选项列表失败：" + optsErr.Error()}, ""
		}
		state.Logger.Debug(fmt.Sprintf("找到 %d 个可用选项", len(availableOpts)), "price")
		for _, wanted := range hwOptions {
			matched, matchedPC, tier := catalog.MatchEcoOption(availableOpts, wanted)
			if matched != nil && matchedPC != wanted {
				state.Logger.Info("硬件选项 "+wanted+" 按「"+tier+"」匹配到 Eco 选项 "+matchedPC, "price")
			}
			if matched == nil {
				state.Logger.Error("硬件选项 "+wanted+" 不在 OVH 可用选项列表中", "price")
				return Result{Success: false, Error: "硬件选项 " + wanted + " 在该机房不可订购，无法定价"}, ""
			}
			duration, pricingMode, valid := pickPricing(matched["prices"], baseDuration, basePricingMode)
			if !valid {
				return Result{Success: false, Error: fmt.Sprintf("硬件选项 %s 不支持购买时长 %s", matchedPC, orderterm.Label(baseDuration))}, ""
			}
			optWrites = append(optWrites, optWrite{planCode: matchedPC, body: map[string]interface{}{
				"itemId":      itemID,
				"planCode":    matchedPC,
				"duration":    duration,
				"pricingMode": pricingMode,
				"quantity":    1,
			}})
		}
	}

	// 4. 并发写入必需配置与硬件选项(一次往返),结果仍按原先口径判定:
	// 机房写失败 → 本次询价作废;其他必需配置失败 → 降级;选项失败 → 作废
	cfgErrs := make([]error, len(configurations))
	optErrs := make([]error, len(optWrites))
	var wg2 sync.WaitGroup
	for i, cfg := range configurations {
		wg2.Add(1)
		go func(client *ovhsdk.Client, i int, label, value string) {
			defer wg2.Done()
			cfgErrs[i] = client.Post(fmt.Sprintf("/order/cart/%s/item/%d/configuration", cartID, itemID),
				map[string]interface{}{"label": label, "value": value}, nil)
		}(fork(), i, cfg.label, cfg.value)
	}
	for i, ow := range optWrites {
		wg2.Add(1)
		go func(client *ovhsdk.Client, i int, body map[string]interface{}) {
			defer wg2.Done()
			optErrs[i] = client.Post(fmt.Sprintf("/order/cart/%s/eco/options", cartID), body, nil)
		}(fork(), i, ow.body)
	}
	wg2.Wait()
	for i, cfg := range configurations {
		err := cfgErrs[i]
		if err == nil {
			state.Logger.Debug(fmt.Sprintf("设置配置: %s = %s", cfg.label, cfg.value), "price")
			continue
		}
		if cfg.label == "dedicated_datacenter" {
			state.Logger.Error(fmt.Sprintf("设置机房 %s 失败: %s，本次询价作废", cfg.value, err.Error()), "price")
			return Result{Success: false, Error: "无法把购物车配置到指定机房:" + ovh.Explain(err)}, ""
		}
		state.Logger.Warn(fmt.Sprintf("设置配置 %s 失败: %s", cfg.label, err.Error()), "price")
		degraded = true
		if degradedReason != "" {
			degradedReason += "；"
		}
		degradedReason += "必填配置 " + cfg.label + " 设置失败：" + err.Error()
	}
	addedPlanCodes := []string{}
	for i, ow := range optWrites {
		if err := optErrs[i]; err != nil {
			state.Logger.Error(fmt.Sprintf("添加选项 %s 失败: %s", ow.planCode, err.Error()), "price")
			return Result{Success: false, Error: "添加硬件选项 " + ow.planCode + " 失败：" + err.Error()}, ""
		}
		addedPlanCodes = append(addedPlanCodes, ow.planCode)
	}
	if len(addedPlanCodes) > 0 {
		state.Logger.Info(fmt.Sprintf("共添加 %d 个选项: %v", len(addedPlanCodes), addedPlanCodes), "price")
	}

	var cartSummary map[string]interface{}
	if err := client.Get("/order/cart/"+cartID+"/summary", &cartSummary); err != nil {
		return Result{Success: false, Error: ovh.Explain(err)}, ""
	}

	priceInfo := &PriceInfo{
		Duration:    baseDuration,
		PricingMode: basePricingMode,
		Prices: map[string]interface{}{
			"withTax":      nil,
			"withoutTax":   nil,
			"tax":          nil,
			"currencyCode": nil,
		},
		Items: []map[string]interface{}{},
	}

	// 从 summary 提取价格
	if cartSummary != nil {
		if pricesField, ok := cartSummary["prices"].(map[string]interface{}); ok {
			withTaxVal, withTaxCurrency := extractPriceField(pricesField["withTax"])
			withoutTaxVal, _ := extractPriceField(pricesField["withoutTax"])
			taxVal, _ := extractPriceField(pricesField["tax"])

			// 币种只存在于每个 order.Price 对象里(withTax / withoutTax / tax 各带一份),
			// order.OrderPrices **没有**顶层 currencyCode —— 这里以前会去读那个字段,
			// 它在三个区的 schema 里都不存在,恒为 nil,是一句基于错误假设的死代码。
			// 依次从三个价格对象里取,哪个有就用哪个。
			currency := withTaxCurrency
			if currency == "" {
				_, currency = extractPriceField(pricesField["withoutTax"])
			}
			if currency == "" {
				_, currency = extractPriceField(pricesField["tax"])
			}
			// 以前这里兜底写死 "EUR"。币种是跟子公司走的,不是跟站点走的:
			// 实测公开目录 locale.currencyCode —— IE=EUR / CA=QC=CAD / US=WE=WS=USD /
			// SG=SGD / AU=AUD。给一张美元报价盖上 EUR 会直接误导花多少钱,
			// 宁可留空让上层显示"未知"。
			if currency == "" {
				state.Logger.Warn("summary 未返回 currencyCode，币种留空（不再默认 EUR：子公司 "+
					subsidiary+" 未必是欧元计价）", "price")
			}

			priceInfo.Prices["withTax"] = withTaxVal
			priceInfo.Prices["withoutTax"] = withoutTaxVal
			priceInfo.Prices["tax"] = taxVal
			priceInfo.Prices["currencyCode"] = currency
		}
	}

	// 每个商品的价格：order.Order.details[] 里每条是 order.OrderDetail
	// (description / cartItemID / quantity / totalPrice / unitPrice，后两个是 order.Price)。
	// 注意 order.OrderDetail 没有含税/不含税拆分，税只在订单级 prices 上给，所以这里不再造
	// withTax/withoutTax 字段，避免把不含税单价当含税价展示。
	if cartSummary != nil {
		if detailsRaw, ok := cartSummary["details"].([]interface{}); ok {
			for _, detailRaw := range detailsRaw {
				d, ok := detailRaw.(map[string]interface{})
				if !ok {
					continue
				}
				totalVal, currency := extractPriceField(d["totalPrice"])
				unitVal, unitCurrency := extractPriceField(d["unitPrice"])
				if currency == "" {
					currency = unitCurrency
				}
				// 同上:不再默认 EUR,取不到就留空
				priceInfo.Items = append(priceInfo.Items, map[string]interface{}{
					"itemId":       d["cartItemID"],
					"description":  d["description"],
					"quantity":     d["quantity"],
					"totalPrice":   totalVal,
					"unitPrice":    unitVal,
					"currencyCode": currency,
				})
			}
		}
	}

	withoutTaxStr := fmt.Sprintf("%v", priceInfo.Prices["withoutTax"])
	withTaxStr := fmt.Sprintf("%v", priceInfo.Prices["withTax"])
	currencyStr := fmt.Sprintf("%v", priceInfo.Prices["currencyCode"])
	state.Logger.Info(fmt.Sprintf("价格查询成功: 总价未税=%s, 含税=%s %s", withoutTaxStr, withTaxStr, currencyStr), "price")

	if degraded {
		state.Logger.Warn("询价结果降级（购物车未完全配置成功）: "+degradedReason, "price")
	}
	res := Result{
		Success:        true,
		PlanCode:       planCode,
		Datacenter:     datacenter,
		Options:        addedPlanCodes,
		Degraded:       degraded,
		DegradedReason: degradedReason,
		Price:          priceInfo,
	}
	if keepCart && !degraded {
		kept = true
		return res, cartID
	}
	return res, ""
}

func orderSubsidiary(state *app.State, acc types.OVHAccount) string {
	// 大写化 / zone 为空时按 endpoint 推,统一走 catalog.SubsidiaryOfAccount(全项目唯一口径),
	// 这里只在它之上再加一道"跨大区"校验
	sub := catalog.SubsidiaryOfAccount(acc)
	fallback := ovh.DefaultSubsidiaryForEndpoint(acc.Endpoint)
	if !ovh.KnownSubsidiary(sub) || ovh.SubsidiaryRegion(sub) != ovh.EndpointRegion(acc.Endpoint) {
		state.Logger.Warn(fmt.Sprintf("账户 %s 的子公司 %q 不属于 endpoint %s 所在的 %s 区（OVH 会以 invalid ovhSubsidiary 拒绝建车），本次改用 %s",
			acc.Name, acc.Zone, acc.Endpoint, ovh.EndpointRegion(acc.Endpoint), fallback), "price")
		return fallback
	}
	return sub
}

// regionAllowedValues / pickAllowedRegion 与 purchase.go 的同名函数保持同一份逻辑。
//
// schema 里 order.cart.ConfigurationRequirements 只声明 label/required/type,但三区实测
// GET /order/cart/{cartId}/item/{itemId}/requiredConfiguration 都会带 allowedValues,
// 而且这份值是按"这辆车的子公司 + 这个 planCode"算出来的,天然是本区正确值:
// US 车 → ["united_states"],EU/CA 车 → ["canada","europe"]。
func regionAllowedValues(required []map[string]interface{}) ([]string, bool) {
	for _, conf := range required {
		if label, _ := conf["label"].(string); label != "region" {
			continue
		}
		mandatory, _ := conf["required"].(bool)
		raw, _ := conf["allowedValues"].([]interface{})
		vals := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, _ := v.(string); s != "" {
				vals = append(vals, s)
			}
		}
		return vals, mandatory
	}
	return nil, false
}

func pickAllowedRegion(allowed []string, prefer string) string {
	if len(allowed) == 0 {
		return ""
	}
	for _, a := range allowed {
		if strings.EqualFold(a, prefer) {
			return a
		}
	}
	return allowed[0]
}

// pickPricing 从 order.cart.GenericProductPricing[] 里挑一条计价填 duration / pricingMode。
//
// 优先级:duration 必须与主商品一致 > 同 duration 内优先 rental+renew(按周期续费那条)
// > 实在没有同 duration 才退而求其次;prices 为空或结构异常时回退 P1M/default。
// 为什么 duration 一致排第一:addon 的 prices[] 常见 P1M/P12M/P24M 三条都是 rental+renew,
// 先按 rental+renew 挑等于取数组里碰巧的第一条、跟主商品 duration 无关;
// 一旦挑中与主商品不同的 duration,OVH 直接拒这个 addon,而这里是 fail-fast
// (整次询价 success:false),代价远大于"少续了一档"。
// 与 purchase.go 的 pickPricing 必须保持同一份逻辑。
func pickPricing(pricesRaw interface{}, duration, preferredMode string) (string, string, bool) {
	// 只在同一购买周期内挑计价(上游 orderterm.Pick):绝不换成别的周期,
	// 否则月付主机会挂上年付配件,或整单被悄悄换成年付。
	if d, pm, ok := orderterm.Pick(pricesRaw, duration, preferredMode); ok {
		return d, pm, true
	}
	// 选项根本没给价格表:没有依据判断,按主商品周期原样发出,由 OVH 校验。
	// 给了价格表却没有这个周期 = 确定买不到,返回 false 让调用方取消。
	if list, _ := pricesRaw.([]interface{}); len(list) == 0 {
		if preferredMode == "" {
			preferredMode = "default"
		}
		return duration, preferredMode, true
	}
	return "", "", false
}

// filterHardwareOptions 剔除许可证 / OS / 控制面板类选项,只留真正决定机器配置的 addon。
// 口径与 purchase.go 的同名函数一致 —— 两处必须同进同退,否则会出现
// "询价说这个组合不可定价、抢购却能下单"(或反之)的分裂。
// 这里复制一份而不是 import purchase:price 是 purchase 的下游(quick_order 先询价再入队),
// 反向依赖会成环。
func filterHardwareOptions(state *app.State, opts []string) []string {
	filtered := []string{}
	for _, opt := range opts {
		if opt == "" {
			continue
		}
		lc := strings.ToLower(opt)
		skip := false
		for _, term := range []string{"windows-server", "sql-server", "cpanel-license", "plesk-",
			"-license-", "os-", "control-panel", "license", "security"} {
			if strings.Contains(lc, term) {
				skip = true
				break
			}
		}
		if skip {
			state.Logger.Debug("询价跳过非硬件/许可证选项: "+opt, "price")
			continue
		}
		filtered = append(filtered, opt)
	}
	return filtered
}

func hasCapacity(raw interface{}, want string) bool {
	list, _ := raw.([]interface{})
	for _, c := range list {
		if s, _ := c.(string); s == want {
			return true
		}
	}
	return false
}

// lookupEcoPricing 查 GET /order/cart/{cartId}/eco(order.cart.GenericProductDefinition[])
// 里某个 planCode 的真实可用计价。只在按 P1M/default 加购被 OVH 拒之后才调,
// 这样正常的月付机型一次多余请求都不会发。
func lookupEcoPricing(state *app.State, client *ovhsdk.Client, cartID, planCode, preferDuration string) (string, string, bool) {
	var defs []map[string]interface{}
	if err := client.Get("/order/cart/"+cartID+"/eco", &defs); err != nil {
		state.Logger.Warn("查询 Eco 计价失败: "+err.Error(), "price")
		return "", "", false
	}
	for _, def := range defs {
		if pc, _ := def["planCode"].(string); pc != planCode {
			continue
		}
		// 只认同一周期下的其他计价模式;没有该周期就是买不到,不能换周期重试
		d, pm, ok := orderterm.Pick(def["prices"], preferDuration, "")
		return d, pm, ok
	}
	state.Logger.Debug("在 Eco 目录里未找到 planCode: "+planCode, "price")
	return "", "", false
}

// extractPriceField 兼容字典形式（{value, currencyCode}）与直接值，统一转 float64
// OVH SDK 用 UseNumber，数字是 json.Number，必须经过 numconv 才能拿到正确值
func extractPriceField(v interface{}) (interface{}, string) {
	if v == nil {
		return nil, ""
	}
	if m, ok := v.(map[string]interface{}); ok {
		currency, _ := m["currencyCode"].(string)
		if f, ok := numconv.ToFloat64(m["value"]); ok {
			return f, currency
		}
		return nil, currency
	}
	if f, ok := numconv.ToFloat64(v); ok {
		return f, ""
	}
	return nil, ""
}
