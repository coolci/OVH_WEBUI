package purchase

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// PurchaseServer 对应 Python: purchase_server
// 返回是否成功。多账户:用 item.AccountID 取对应 OVH client 和 subsidiary。
func PurchaseServer(state *app.State, item *types.QueueItem) bool {
	client, err := state.OVH.ClientFor(item.AccountID)
	if err != nil {
		state.Logger.Error("PurchaseServer: 取 OVH client 失败 ("+item.AccountID+"): "+err.Error(), "purchase")
		return false
	}

	cartID := ""
	var itemID int64

	state.Logger.Info(fmt.Sprintf("开始为 %s 在 %s 的购买流程，选项: %v",
		item.PlanCode, item.Datacenter, item.Options), "purchase")

	// 检查可用性
	var availabilities []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", item.PlanCode)
	if err := client.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &availabilities); err != nil {
		errMsg := err.Error()
		state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误: %s", item.PlanCode, errMsg), "purchase")
		recordFailure(state, item, errMsg)
		return false
	}

	apiDC := ovh.ConvertDisplayDCToAPIDC(item.Datacenter)
	foundAvailable := false
	// 记下"实际可用的那条 FQN"。FQN 格式：<planCode>.<addon1>.<addon2>...
	// 用户没显式指定 options 时，会从这个 FQN 推断 addon，让订单走"有货的那套配置"，
	// 不再退化到 OVH 默认 addon（多半是 HDD / 最小内存）。
	var availableFQN string
	for _, av := range availabilities {
		if dcsRaw, ok := av["datacenters"].([]interface{}); ok {
			for _, dcRaw := range dcsRaw {
				dc, ok := dcRaw.(map[string]interface{})
				if !ok {
					continue
				}
				dcName, _ := dc["datacenter"].(string)
				availStr, _ := dc["availability"].(string)
				if dcName == apiDC && availStr != "unavailable" && availStr != "unknown" {
					foundAvailable = true
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
		state.Logger.Info(fmt.Sprintf("服务器 %s 在数据中心 %s 当前无货", item.PlanCode, item.Datacenter), "purchase")
		return false
	}

	// 决定本次下单使用的硬件 options：
	// - 用户显式指定了 options → 直接用（fail-fast 由后面的 eco/options 处理）
	// - 用户没指定 → 从可用 FQN 推断 addon planCode，确保订单走"实际有货的那套配置"
	effectiveOptions := item.Options
	if len(effectiveOptions) == 0 && availableFQN != "" {
		parts := strings.Split(availableFQN, ".")
		if len(parts) > 1 {
			effectiveOptions = parts[1:] // 第一段是 base planCode，其余是 addon planCodes
			state.Logger.Info(fmt.Sprintf("用户未指定硬件选项，从可用 FQN %s 推断 addon: %v",
				availableFQN, effectiveOptions), "purchase")
		}
	}

	// 多账户:购物车 subsidiary 跟着账户走,不再读全局 cfg
	acc, _ := state.FindAccount(item.AccountID)
	subsidiary := acc.Zone
	if subsidiary == "" {
		subsidiary = "IE"
	}

	// 创建购物车
	state.Logger.Info(fmt.Sprintf("为区域 %s 创建购物车 (账户 %s)", subsidiary, acc.Name), "purchase")
	var cartResult map[string]interface{}
	if err := client.Post("/order/cart", map[string]interface{}{
		"ovhSubsidiary": subsidiary,
	}, &cartResult); err != nil {
		state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误: %s", item.PlanCode, err.Error()), "purchase")
		recordFailure(state, item, err.Error())
		return false
	}
	cartID, _ = cartResult["cartId"].(string)
	state.Logger.Info("购物车创建成功，ID: "+cartID, "purchase")

	// 抢购失败时清理 OVH 购物车,避免 OVH 侧堆积僵尸 cart(高频抢购累计能上千个,
	// 进而触发 OVH 限流)。checkout 成功时 cart 自动转 order,Delete 会 404,
	// 所以只在 !success 时尝试,且失败不影响主流程。
	success := false
	defer func() {
		if success || cartID == "" {
			return
		}
		if err := client.Delete("/order/cart/"+cartID, nil); err != nil {
			state.Logger.Debug(fmt.Sprintf("清理失败 cart %s: %s", cartID, err.Error()), "purchase")
		} else {
			state.Logger.Debug("已清理失败 cart "+cartID, "purchase")
		}
	}()

	// 立即绑定购物车到账户 —— 对齐 OVH 官方 PHP / Python 示例的推荐顺序：
	// cart → assign → eco → configuration → options → summary → checkout。
	// 在 add item 之前 assign，OVH 后端不会出现"cart 未绑定就 checkout"的边界错误。
	state.Logger.Info("绑定购物车 "+cartID, "purchase")
	if err := client.Post("/order/cart/"+cartID+"/assign", map[string]interface{}{}, nil); err != nil {
		errMsg := err.Error()
		state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误: %s", item.PlanCode, errMsg), "purchase")
		state.Logger.Error("错误发生时的购物车ID: "+cartID, "purchase")
		recordFailure(state, item, errMsg)
		return false
	}
	state.Logger.Info("购物车绑定成功", "purchase")

	// 添加基础商品 /eco
	state.Logger.Info(fmt.Sprintf("添加基础商品 %s 到购物车 (使用 /eco)", item.PlanCode), "purchase")
	var itemResult map[string]interface{}
	if err := client.Post("/order/cart/"+cartID+"/eco", map[string]interface{}{
		"planCode":    item.PlanCode,
		"pricingMode": "default",
		"duration":    "P1M",
		"quantity":    1,
	}, &itemResult); err != nil {
		state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误: %s", item.PlanCode, err.Error()), "purchase")
		state.Logger.Error(fmt.Sprintf("错误发生时的购物车ID: %s", cartID), "purchase")
		recordFailure(state, item, err.Error())
		return false
	}
	if n, ok := numconv.ToInt64(itemResult["itemId"]); ok {
		itemID = n
	}
	if itemID == 0 {
		errMsg := fmt.Sprintf("无法从购物车响应中解析 itemId（响应: %v）", itemResult)
		state.Logger.Error(fmt.Sprintf("购买 %s 时发生未知错误: %s", item.PlanCode, errMsg), "purchase")
		state.Logger.Error("错误发生时的购物车ID: "+cartID, "purchase")
		recordFailure(state, item, errMsg)
		return false
	}
	state.Logger.Info(fmt.Sprintf("基础商品添加成功，项目 ID: %d", itemID), "purchase")

	// 设置必需配置
	state.Logger.Info(fmt.Sprintf("为项目 %d 设置必需配置", itemID), "purchase")
	region := ovh.RegionForDC(apiDC)

	// 与 Python 一致的顺序：dedicated_datacenter → dedicated_os → (region)
	type kv struct{ label, value string }
	configurations := []kv{
		{"dedicated_datacenter", apiDC},
		{"dedicated_os", "none_64.en"},
	}
	if region != "" {
		configurations = append(configurations, kv{"region", region})
	} else {
		state.Logger.Warn(fmt.Sprintf("无法为数据中心 %s 推断区域，可能导致配置失败", strings.ToLower(apiDC)), "purchase")
		// 对应 Python: 查 requiredConfiguration 看 region 是否必填
		var required []map[string]interface{}
		if err := client.Get(fmt.Sprintf("/order/cart/%s/item/%d/requiredConfiguration", cartID, itemID), &required); err != nil {
			state.Logger.Warn(fmt.Sprintf("获取必需配置失败或区域为必需但未确定: %s", err.Error()), "purchase")
		} else {
			for _, conf := range required {
				if label, _ := conf["label"].(string); label == "region" {
					if req, _ := conf["required"].(bool); req {
						errMsg := "必需的区域配置无法确定。"
						state.Logger.Error(fmt.Sprintf("购买 %s 时发生未知错误: %s", item.PlanCode, errMsg), "purchase")
						recordFailure(state, item, errMsg)
						return false
					}
				}
			}
		}
	}
	// 激进:datacenter / os / region 三个 configuration POST 全并发。
	// 这三个 label 各自独立校验,跟 OVH 后端是 cart-level state writes,
	// 实测能跑通。failed -> 任一失败 fail-fast。
	postConfig := func(label, value string) error {
		state.Logger.Info(fmt.Sprintf("配置项目 %d: 设置必需项 %s = %s", itemID, label, value), "purchase")
		if err := client.Post(fmt.Sprintf("/order/cart/%s/item/%d/configuration", cartID, itemID),
			map[string]interface{}{"label": label, "value": value}, nil); err != nil {
			return err
		}
		state.Logger.Info(fmt.Sprintf("成功设置必需项: %s = %s", label, value), "purchase")
		return nil
	}
	type cfgResult struct {
		label string
		err   error
	}
	results := make(chan cfgResult, len(configurations))
	for _, c := range configurations {
		c := c
		go func() {
			results <- cfgResult{label: c.label, err: postConfig(c.label, c.value)}
		}()
	}
	var firstErr cfgResult
	for i := 0; i < len(configurations); i++ {
		r := <-results
		if r.err != nil && firstErr.err == nil {
			firstErr = r
		}
	}
	if firstErr.err != nil {
		errMsg := firstErr.err.Error()
		state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误(%s): %s", item.PlanCode, firstErr.label, errMsg), "purchase")
		state.Logger.Error(fmt.Sprintf("错误发生时的购物车ID: %s", cartID), "purchase")
		state.Logger.Error(fmt.Sprintf("错误发生时的基础商品ID: %d", itemID), "purchase")
		recordFailure(state, item, errMsg)
		return false
	}

	// 硬件选项处理。effectiveOptions 已经包含了：
	//   - 用户显式 options（如果有），或
	//   - 从可用 FQN 推断的 addon planCode（用户没指定时）
	if len(effectiveOptions) > 0 {
		state.Logger.Info(fmt.Sprintf("📦 处理硬件选项（%d个）: %v", len(effectiveOptions), effectiveOptions), "purchase")
		filtered := []string{}
		for _, opt := range effectiveOptions {
			if opt == "" {
				continue
			}
			lc := strings.ToLower(opt)
			skip := false
			// 过滤掉非硬件 / 许可证类（注意 "panel" 不在过滤词里：FQN 推断的 addon
			// 不会撞这词，删了避免误伤；旧版有 "panel" 是因为前端可能塞 cpanel 选项过来）
			for _, term := range []string{"windows-server", "sql-server", "cpanel-license", "plesk-",
				"-license-", "os-", "control-panel", "license", "security"} {
				if strings.Contains(lc, term) {
					skip = true
					break
				}
			}
			if skip {
				state.Logger.Info("跳过非硬件/许可证选项: "+opt, "purchase")
				continue
			}
			filtered = append(filtered, opt)
		}
		if len(filtered) > 0 {
			state.Logger.Info(fmt.Sprintf("过滤后的硬件选项计划代码: %v", filtered), "purchase")
			var availableEcoOpts []map[string]interface{}
			q := url.Values{}
			q.Set("planCode", item.PlanCode)
			if err := client.Get(fmt.Sprintf("/order/cart/%s/eco/options?%s", cartID, q.Encode()), &availableEcoOpts); err != nil {
				// 拉 eco/options 失败 → 中止订单。否则会用基础 plan 默认存储（多半是 HDD）下到错误配置
				errMsg := fmt.Sprintf("获取 Eco 硬件选项列表失败: %s（用户指定了 %d 个选项，无法验证，已取消下单避免下到错误配置）", err.Error(), len(filtered))
				state.Logger.Error(errMsg, "purchase")
				recordFailure(state, item, errMsg)
				return false
			}
			state.Logger.Info(fmt.Sprintf("找到 %d 个可用的 Eco 硬件选项。", len(availableEcoOpts)), "purchase")

			// 先全部匹配,失败直接 fail-fast(避免任何 POST 都发出去之前先卡 missing)
			type addonPayload struct {
				planCode string
				body     map[string]interface{}
			}
			var todo []addonPayload
			var missing []string
			for _, wanted := range filtered {
				matched := false
				for _, avail := range availableEcoOpts {
					availPC, _ := avail["planCode"].(string)
					if availPC != wanted {
						continue
					}
					duration := "P1M"
					if d, ok := avail["duration"].(string); ok && d != "" {
						duration = d
					}
					pricingMode := "default"
					if pm, ok := avail["pricingMode"].(string); ok && pm != "" {
						pricingMode = pm
					}
					todo = append(todo, addonPayload{
						planCode: availPC,
						body: map[string]interface{}{
							"itemId":      itemID,
							"planCode":    availPC,
							"duration":    duration,
							"pricingMode": pricingMode,
							"quantity":    1,
						},
					})
					matched = true
					break
				}
				if !matched {
					missing = append(missing, wanted)
				}
			}
			if len(missing) > 0 {
				errMsg := fmt.Sprintf("用户请求的硬件选项 %v 未在 OVH 可用 Eco 选项中找到（已取消下单避免下到错误配置）", missing)
				state.Logger.Error(errMsg, "purchase")
				recordFailure(state, item, errMsg)
				return false
			}

			// 并发 POST 各 addon。OVH 没明确说支持并发,但 addon 是 cart-append 操作,
			// 实测可行;失败仍然 fail-fast。比串行省 ~N×1s。
			state.Logger.Info(fmt.Sprintf("并发添加 %d 个 Eco 选项: %v", len(todo), filtered), "purchase")
			type addResult struct {
				planCode string
				err      error
			}
			results := make(chan addResult, len(todo))
			for _, t := range todo {
				go func(t addonPayload) {
					err := client.Post(fmt.Sprintf("/order/cart/%s/eco/options", cartID), t.body, nil)
					results <- addResult{planCode: t.planCode, err: err}
				}(t)
			}
			// 收齐所有结果(不能 break,要等所有 goroutine 退出避免漏 error 日志)
			var firstErr addResult
			for i := 0; i < len(todo); i++ {
				r := <-results
				if r.err != nil {
					if firstErr.err == nil {
						firstErr = r
					}
					state.Logger.Error(fmt.Sprintf("添加 Eco 选项 %s 失败: %s", r.planCode, r.err.Error()), "purchase")
				} else {
					state.Logger.Info(fmt.Sprintf("成功添加 Eco 选项: %s", r.planCode), "purchase")
				}
			}
			if firstErr.err != nil {
				// 关键选项添加失败 → 整单失败。不能静默继续 checkout,否则会下到错误配置。
				errMsg := fmt.Sprintf("添加 Eco 选项 %s 失败: %s（已取消下单避免下到错误配置）", firstErr.planCode, firstErr.err.Error())
				recordFailure(state, item, errMsg)
				return false
			}
			state.Logger.Info(fmt.Sprintf("共成功添加 %d 个硬件选项。", len(filtered)), "purchase")
		}
	} else {
		state.Logger.Info("⚠️ 用户未提供任何硬件选项，将使用默认配置下单", "purchase")
	}

	// 结账前再确认任务未被用户删除，避免删队后仍下单成功
	state.DeletedTaskIDsMu.Lock()
	_, deletedBeforeCheckout := state.DeletedTaskIDs[item.ID]
	state.DeletedTaskIDsMu.Unlock()
	if deletedBeforeCheckout {
		state.Logger.Info("任务 "+item.ID+" 已在结账前被删除，取消 checkout", "purchase")
		return false
	}

	// 直接结账 —— 跳过 /summary(它只是日志用的价格,2 秒开销),
	// 价格 + 过期时间下面 checkout 成功后用 /me/order 异步补,不阻塞主流程。
	state.Logger.Info("对购物车 "+cartID+" 执行结账", "purchase")
	var checkoutResult map[string]interface{}
	checkoutPayload := map[string]interface{}{
		"autoPayWithPreferredPaymentMethod": false,
		"waiveRetractationPeriod":           true,
	}
	if err := client.Post("/order/cart/"+cartID+"/checkout", checkoutPayload, &checkoutResult); err != nil {
		errMsg := err.Error()
		state.Logger.Error(fmt.Sprintf("购买 %s 时发生 OVH API 错误: %s", item.PlanCode, errMsg), "purchase")
		recordFailure(state, item, errMsg)
		return false
	}

	orderID := numconv.ToString(checkoutResult["orderId"])
	orderURL, _ := checkoutResult["url"].(string)

	// checkout 已返回订单 ID,cart 已成功转 order,标记成功阻止 defer 删除
	success = true

	// 立刻记成功 —— 价格和过期时间空着,后台异步补
	recordSuccess(state, item, orderID, orderURL, "", nil)

	// 异步补:从 /me/order/{orderID} 读 expirationDate + 价格,写回 history
	if orderID != "" {
		go backfillOrderDetail(state, client, item.ID, orderID)
	}

	state.Logger.Info(fmt.Sprintf("成功购买 %s 在 %s (订单ID: %s, URL: %s)",
		item.PlanCode, item.Datacenter, orderID, orderURL), "purchase")

	// 发送 Telegram 成功通知。TG token / chat id 仍然走全局 state.Config(Telegram 是平台级配置,跨账户共享)
	tgCfg := state.Config.Get()
	if tgCfg.TgToken != "" && tgCfg.TgChatID != "" {
		msg := fmt.Sprintf("🎉 OVH 服务器抢购成功！🎉\n\n服务器型号 (Plan Code): %s\n数据中心: %s\n订单 ID: %s\n订单链接: %s\n",
			item.PlanCode, item.Datacenter, orderID, orderURL)
		if len(item.Options) > 0 {
			msg += "自定义配置: " + strings.Join(item.Options, ", ") + "\n"
		}
		msg += "\n抢购任务ID: " + item.ID
		telegram.SendMessage(state, msg, nil)
		state.Logger.Info("已为订单 "+orderID+" 发送 Telegram 成功通知。", "purchase")
	} else {
		state.Logger.Info("未配置 Telegram Token 或 Chat ID，跳过成功通知发送。", "purchase")
	}
	return true
}

func extract(v interface{}) *float64 {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		if f, ok := numconv.ToFloat64(m["value"]); ok {
			return &f
		}
		return nil
	}
	if f, ok := numconv.ToFloat64(v); ok {
		return &f
	}
	return nil
}

func recordSuccess(state *app.State, item *types.QueueItem, orderID, orderURL, expirationTime string, priceInfo *types.PriceInfo) {
	state.HistoryMu.Lock()
	defer state.HistoryMu.Unlock()
	now := types.NowISO()

	for i := range state.History {
		if state.History[i].TaskID == item.ID {
			state.History[i].Status = "success"
			state.History[i].AccountID = item.AccountID
			state.History[i].OrderID = orderID
			state.History[i].OrderURL = orderURL
			state.History[i].ErrorMessage = nil
			state.History[i].PurchaseTime = now
			state.History[i].AttemptCount = item.RetryCount
			state.History[i].Options = item.Options
			if expirationTime != "" {
				state.History[i].ExpirationTime = expirationTime
			}
			if priceInfo != nil {
				state.History[i].Price = priceInfo
			}
			state.Logger.Info("更新抢购历史(成功) 任务ID: "+item.ID, "purchase")
			go state.SaveHistory()
			return
		}
	}

	entry := types.PurchaseHistoryEntry{
		ID:           uuid.NewString(),
		TaskID:       item.ID,
		AccountID:    item.AccountID,
		PlanCode:     item.PlanCode,
		Datacenter:   item.Datacenter,
		Options:      item.Options,
		Status:       "success",
		OrderID:      orderID,
		OrderURL:     orderURL,
		PurchaseTime: now,
		AttemptCount: item.RetryCount,
	}
	if expirationTime != "" {
		entry.ExpirationTime = expirationTime
	}
	if priceInfo != nil {
		entry.Price = priceInfo
	}
	state.History = append(state.History, entry)
	state.Logger.Info("创建抢购历史(成功) 任务ID: "+item.ID, "purchase")
	go state.SaveHistory()
}

func recordFailure(state *app.State, item *types.QueueItem, errMsg string) {
	state.HistoryMu.Lock()
	defer state.HistoryMu.Unlock()
	now := types.NowISO()

	for i := range state.History {
		if state.History[i].TaskID == item.ID {
			state.History[i].Status = "failed"
			state.History[i].AccountID = item.AccountID
			state.History[i].OrderID = ""
			state.History[i].OrderURL = ""
			em := errMsg
			state.History[i].ErrorMessage = &em
			state.History[i].PurchaseTime = now
			state.History[i].AttemptCount = item.RetryCount
			state.History[i].Options = item.Options
			state.Logger.Info("更新抢购历史(失败) 任务ID: "+item.ID, "purchase")
			go state.SaveHistory()
			return
		}
	}
	em := errMsg
	entry := types.PurchaseHistoryEntry{
		ID:           uuid.NewString(),
		TaskID:       item.ID,
		AccountID:    item.AccountID,
		PlanCode:     item.PlanCode,
		Datacenter:   item.Datacenter,
		Options:      item.Options,
		Status:       "failed",
		ErrorMessage: &em,
		PurchaseTime: now,
		AttemptCount: item.RetryCount,
	}
	state.History = append(state.History, entry)
	state.Logger.Info("创建抢购历史(失败) 任务ID: "+item.ID, "purchase")
	go state.SaveHistory()
}

// backfillOrderDetail 下单成功后异步补 history 行的 expirationTime + price。
// 不阻塞 PurchaseServer 主流程,即便这一步失败 history 也已经标 success(只是少了价格 / 过期时间)。
// 在独立 goroutine 跑,持有 OVH client 引用,只 read /me/order/{orderID}。
func backfillOrderDetail(state *app.State, client *ovhsdk.Client, taskID, orderID string) {
	var orderInfo map[string]interface{}
	if err := client.Get("/me/order/"+orderID, &orderInfo); err != nil {
		state.Logger.Warn(fmt.Sprintf("异步查询订单 %s 详情失败: %s", orderID, err.Error()), "purchase")
		return
	}

	expirationTime := ""
	if ret, ok := orderInfo["retractionDate"].(string); ok && ret != "" {
		expirationTime = ret
	} else if exp, ok := orderInfo["expirationDate"].(string); ok {
		expirationTime = exp
	}

	// /me/order 返回的价格字段:priceWithTax / priceWithoutTax / tax,
	// 每个典型形式 { value: <number-or-json.Number>, currencyCode: string, text: string }。
	// 复用 extract(),它能容 float64 / json.Number / string / int 各种来法。
	pickCurrency := func(field interface{}) string {
		m, ok := field.(map[string]interface{})
		if !ok {
			return ""
		}
		c, _ := m["currencyCode"].(string)
		return c
	}

	var priceInfo *types.PriceInfo
	withTax := extract(orderInfo["priceWithTax"])
	withoutTax := extract(orderInfo["priceWithoutTax"])
	tax := extract(orderInfo["tax"])
	currency := pickCurrency(orderInfo["priceWithTax"])
	if currency == "" {
		currency = pickCurrency(orderInfo["priceWithoutTax"])
	}
	if currency == "" {
		currency = "EUR"
	}
	if withTax != nil || withoutTax != nil {
		priceInfo = &types.PriceInfo{
			WithTax:      withTax,
			WithoutTax:   withoutTax,
			Tax:          tax,
			CurrencyCode: currency,
		}
	}

	if expirationTime == "" && priceInfo == nil {
		return
	}

	state.HistoryMu.Lock()
	defer state.HistoryMu.Unlock()
	for i := range state.History {
		if state.History[i].TaskID != taskID {
			continue
		}
		changed := false
		if expirationTime != "" && state.History[i].ExpirationTime != expirationTime {
			state.History[i].ExpirationTime = expirationTime
			changed = true
		}
		if priceInfo != nil && state.History[i].Price == nil {
			state.History[i].Price = priceInfo
			changed = true
		}
		if changed {
			state.Logger.Info(fmt.Sprintf("补全订单 %s 详情: 过期时间=%q 价格=%v",
				orderID, expirationTime, priceInfo != nil), "purchase")
			go state.SaveHistory()
		}
		return
	}
}
