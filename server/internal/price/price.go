package price

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
)

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
		return Result{Success: false, Error: ovh.Explain(err), Transient: transientErr(err)}, ""
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
	var assignErr error
	var baseErr error
	var baseFail *Result
	var availableOpts []map[string]interface{}
	var optsErr error
	// 并发请求各用一个从当前已创建 client 克隆出的隔离实例（ovh.IsolateClient），
	// 严禁在账户中途刷新/删除时回退复用共享 client 触发 go-ovh Timeout 并发写竞态。
	fork := func() (*ovhsdk.Client, error) {
		return ovh.IsolateClient(client)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		isoCli, ferr := fork()
		if ferr != nil {
			assignErr = ferr
			return
		}
		// schema 里 POST /order/cart/{cartId}/assign 只有 path 参数、没有 body
		if err := isoCli.Post("/order/cart/"+cartID+"/assign", nil, nil); err != nil {
			assignErr = err
			state.Logger.Warn("绑定购物车失败: "+err.Error(), "price")
		}
	}()
	if len(hwOptions) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			isoCli, ferr := fork()
			if ferr != nil {
				optsErr = ferr
				return
			}
			q := url.Values{}
			q.Set("planCode", planCode)
			optsErr = isoCli.Get(fmt.Sprintf("/order/cart/%s/eco/options?%s", cartID, q.Encode()), &availableOpts)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		isoCli, ferr := fork()
		if ferr != nil {
			baseErr = ferr
			return
		}
		// 月付走零额外请求的快速路径;其他周期先查 OVH 给这个型号的真实计价
		if selectedDuration != orderterm.Default {
			d, pm, found := lookupEcoPricing(state, isoCli, cartID, planCode, selectedDuration)
			if !found {
				baseFail = &Result{Success: false, Error: fmt.Sprintf("OVH 不支持 %s 按 %s 购买", planCode, orderterm.Label(selectedDuration))}
				return
			}
			baseDuration, basePricingMode = d, pm
		}
		postBase := func() error {
			itemResult = nil
			return isoCli.Post("/order/cart/"+cartID+"/eco", map[string]interface{}{
				"planCode":    planCode,
				"pricingMode": basePricingMode,
				"duration":    baseDuration,
				"quantity":    1,
			}, &itemResult)
		}
		if err := postBase(); err != nil {
			if d, pm, found := lookupEcoPricing(state, isoCli, cartID, planCode, baseDuration); found &&
				(d != baseDuration || pm != basePricingMode) {
				state.Logger.Warn(fmt.Sprintf("以 %s/%s 加购 %s 失败(%s)，改用目录计价 %s/%s 重试",
					baseDuration, basePricingMode, planCode, ovh.Explain(err), d, pm), "price")
				baseDuration, basePricingMode = d, pm
				err = postBase()
			}
			baseErr = err
		}
	}()
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
		return Result{Success: false, Error: msg, Transient: transientErr(baseErr)}, ""
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
			return Result{Success: false, Error: "获取硬件选项列表失败：" + optsErr.Error(), Transient: transientErr(optsErr)}, ""
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
		go func(i int, label, value string) {
			defer wg2.Done()
			isoCli, ferr := fork()
			if ferr != nil {
				cfgErrs[i] = ferr
				return
			}
			cfgErrs[i] = isoCli.Post(fmt.Sprintf("/order/cart/%s/item/%d/configuration", cartID, itemID),
				map[string]interface{}{"label": label, "value": value}, nil)
		}(i, cfg.label, cfg.value)
	}
	for i, ow := range optWrites {
		wg2.Add(1)
		go func(i int, body map[string]interface{}) {
			defer wg2.Done()
			isoCli, ferr := fork()
			if ferr != nil {
				optErrs[i] = ferr
				return
			}
			optErrs[i] = isoCli.Post(fmt.Sprintf("/order/cart/%s/eco/options", cartID), body, nil)
		}(i, ow.body)
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
			return Result{Success: false, Error: "无法把购物车配置到指定机房:" + ovh.Explain(err), Transient: transientErr(err)}, ""
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
			return Result{Success: false, Error: "添加硬件选项 " + ow.planCode + " 失败：" + err.Error(), Transient: transientErr(err)}, ""
		}
		addedPlanCodes = append(addedPlanCodes, ow.planCode)
	}
	if len(addedPlanCodes) > 0 {
		state.Logger.Info(fmt.Sprintf("共添加 %d 个选项: %v", len(addedPlanCodes), addedPlanCodes), "price")
	}

	var cartSummary map[string]interface{}
	if err := client.Get("/order/cart/"+cartID+"/summary", &cartSummary); err != nil {
		return Result{Success: false, Error: ovh.Explain(err), Transient: transientErr(err)}, ""
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

	splitRecurringAndSetup(priceInfo, cartSummary)

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
	// 未成功完成 /assign 绑定的匿名裸车严禁保留用于预热池快速结账（否则直冲 /checkout 必报 400）
	if keepCart && !degraded && assignErr == nil {
		kept = true
		return res, cartID
	}
	return res, ""
}
