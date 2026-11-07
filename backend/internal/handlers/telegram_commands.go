package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/price"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// dispatchTelegramCommand 处理 /buy /stock 等斜杠命令，返回回复文案。
func dispatchTelegramCommand(state *app.State, mon *monitor.Monitor, cmd *telegram.BotCommand) string {
	if cmd == nil {
		return telegram.HelpMessage()
	}
	switch cmd.Name {
	case "start", "help":
		return telegram.HelpMessage()
	case "watch", "w":
		return watchText(state, mon, cmd.Args)
	case "unwatch", "uw":
		return unwatchText(state, mon, cmd.Args)
	case "stock":
		return cmdStock(state, cmd.Args, mon)
	case "queue", "buy":
		return cmdBuyOrQueue(state, mon, cmd.Args, cmd.Name)
	case "tasks":
		return cmdTasksText(state)
	case "accounts":
		return cmdAccountsText(state)
	case "subs", "subscriptions":
		return cmdSubs(state, mon)
	case "cancel", "c":
		return cmdCancel(state, cmd.Args)
	case "monitor":
		return cmdMonitor(state, mon, cmd.Args)
	case "price":
		return cmdPrice(state, cmd.Args, mon)
	case "interval", "iv":
		return intervalText(state, cmd.Args)
	case "coupon", "cp":
		return cmdCoupon(state, cmd.Args)
	default:
		return "❌ 未知命令: /" + cmd.Name + "\n\n" + telegram.HelpMessage()
	}
}

func cmdStock(state *app.State, args []string, monOpt ...*monitor.Monitor) string {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		return "💡 查询库存格式: /stock <planCode> [@account]\n例: /stock 24ska01\n例: /stock 24ska01 @us"
	}
	planCode := strings.TrimSpace(args[0])
	if !state.HasAnyAccount() {
		return "❌ 尚未配置任何 OVH 账户，请先在 Web 控制台添加"
	}
	var mon *monitor.Monitor
	if len(monOpt) > 0 {
		mon = monOpt[0]
	}
	accountID := telegram.DefaultAccountID(state)
	var explicitAccount *types.OVHAccount
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "@") && len(a) > 1 {
			accs, err := resolveAccountRef(state, mon, a[1:], planCode)
			if err == "" && len(accs) > 0 {
				accountID = accs[0].ID
				explicitAccount = &accs[0]
				break
			}
		}
	}
	if explicitAccount == nil && mon != nil {
		ra := resolveOrderAccount(state, mon, planCode)
		if ra.Account.ID != "" {
			accountID = ra.Account.ID
		}
	}
	avail := catalog.CheckServerAvailabilityWithConfigs(state, planCode, accountID)
	if len(avail) == 0 {
		return fmt.Sprintf("❌ 未查询到型号 %s 的库存信息（型号可能不存在或当前区域无上架）", planCode)
	}

	// 汇总各机房：任一配置有货即视为该 DC 有货
	dcStatus := map[string]string{} // dc → best status
	configLines := []string{}
	for _, cfg := range avail {
		availDCs := []string{}
		for dc, st := range cfg.Datacenters {
			if st == "" || st == "unavailable" || st == "unknown" {
				continue
			}
			availDCs = append(availDCs, telegram.DisplayDC(dc)+"("+st+")")
			// 记录全局机房状态
			if prev, ok := dcStatus[dc]; !ok || prev == "unavailable" {
				dcStatus[dc] = st
			}
		}
		hwLabel := telegram.HumanizeHardware(cfg.Memory, cfg.Storage)
		if len(availDCs) == 0 {
			configLines = append(configLines, fmt.Sprintf("  • %s  [🔴 缺货]", hwLabel))
		} else {
			configLines = append(configLines, fmt.Sprintf("  • %s  [🟢 %s]", hwLabel, strings.Join(availDCs, ", ")))
		}
	}

	inStock := []string{}
	outStock := []string{}
	for dc, st := range dcStatus {
		if st != "" && st != "unavailable" && st != "unknown" {
			inStock = append(inStock, telegram.DisplayDCFull(dc)+" ("+st+")")
		} else {
			outStock = append(outStock, telegram.DisplayDC(dc))
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📦 全球库存穿透查询 · %s\n", planCode))
	b.WriteString(telegram.CardDivider + "\n")
	if len(inStock) > 0 {
		b.WriteString(fmt.Sprintf("🟢 现货节点 (%d 个机房有库存):\n", len(inStock)))
		for _, s := range inStock {
			b.WriteString("  • " + s + "\n")
		}
	} else {
		b.WriteString("🔴 现货状态: 当前全区所有数据中心缺货\n")
		b.WriteString("💡 支持全自动挂机排队，官方放货毫秒抢占！\n")
	}
	if len(configLines) > 0 {
		b.WriteString("\n⚙️ 硬件规格与库存概况:\n")
		// 最多展示 6 行，避免 Telegram 消息过长
		limit := 6
		for i, line := range configLines {
			if i >= limit {
				b.WriteString(fmt.Sprintf("  …另有 %d 条硬件规格省略\n", len(configLines)-limit))
				break
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("⚡ 极速下单命令: /buy " + planCode + " <机房代码>")
	return b.String()
}

// startTelegramOrder 多套配置时先弹出点选,避免 /buy 型号 机房 直接拿随机第一套。
// 返回 true = 已经弹出选择器,调用方不要再 ProcessOrder。
func startTelegramOrder(state *app.State, mon *monitor.Monitor, chatID interface{}, cmdName string, info *telegram.OrderInfo) bool {
	if info == nil || info.PlanCode == "" || info.AccountRef != "" {
		return false
	}
	mode := "b"
	if cmdName == "queue" {
		mode = "q"
	}
	if len(info.Options) > 0 {
		return false
	}
	if info.Datacenter == "" {
		sess := getPickerSession(chatID, 0, mode, info.PlanCode)
		configs := enumerateWizardConfigs(state, info.PlanCode, "")
		if len(configs) == 1 {
			sess.Options = append([]string{}, configs[0].Options...)
			sess.ConfigLabel = configs[0].Label
		}
		showDCPicker(state, chatID, 0, mode, info.PlanCode, false)
		return true
	}
	return false
}

func cmdBuyOrQueue(state *app.State, mon *monitor.Monitor, args []string, cmdName string) string {
	if len(args) < 1 {
		return "用法: /" + cmdName + " <planCode> [datacenter] [quantity] [options]\n例: /" + cmdName + " 24ska01 gra"
	}
	return cmdBuyOrQueueInfo(state, mon, telegram.ParseOrderArgs(args), cmdName)
}

func cmdBuyOrQueueInfo(state *app.State, mon *monitor.Monitor, info *telegram.OrderInfo, cmdName string) string {
	if info == nil || info.PlanCode == "" {
		return "❌ 无法解析参数\n用法: /" + cmdName + " <planCode> [datacenter] [quantity] [options]"
	}
	// 拒绝把子命令当 planCode
	if strings.HasPrefix(info.PlanCode, "/") {
		return "❌ 型号无效: " + info.PlanCode
	}

	state.Logger.Info(fmt.Sprintf("Telegram /%s: planCode=%s dc=%s qty=%d opts=%v ref=%s",
		cmdName, info.PlanCode, info.Datacenter, info.Quantity, info.Options, info.AccountRef), "telegram")

	if info.AccountRef != "" {
		accs, err := resolveAccountRef(state, mon, info.AccountRef, info.PlanCode)
		if err != "" {
			return "❌ " + err
		}
		return runOrder(state, info, accs)
	}

	resolved := resolveOrderAccount(state, mon, info.PlanCode)
	if resolved.Reason != "" {
		return "❌ " + resolved.Reason
	}
	acc := resolved.Account
	result := telegram.ProcessOrder(state, acc.ID, info.PlanCode, info.Datacenter, info.Quantity, info.Options, info.Coupon)
	if result.Success {
		dcText := "全区所有可用机房"
		if info.Datacenter != "" {
			dcText = telegram.DisplayDCFull(info.Datacenter)
		}
		optsText := "任意硬件配置 (自动优选)"
		if len(info.Options) > 0 {
			optsText = telegram.HumanizeOptionCodes(info.Options)
		}
		title := "⚡ 抢购任务已建立入队！"
		if cmdName == "buy" {
			title = "🚀 快速下单已提交并进入队列！"
		}
		accExplain := explainAccountChoice(resolved, info.PlanCode)
		accLine := ""
		if accExplain != "" {
			accLine = "👤 执行账户: " + accExplain + "\n"
		}
		couponLine := ""
		if info.Coupon != "" {
			couponLine = "🎫 优惠代码: " + info.Coupon + " (OVH 结账时自动抵扣)\n"
		}
		return fmt.Sprintf("%s\n%s\n📦 目标型号: %s\n📍 目标机房: %s\n🔢 下单数量: %d 台\n⚙️ 硬件规格: %s\n%s%s⏱️ 轮询周期: %d 秒 / 轮 (极速引擎)\n%s\n🎯 状态: 后台挂机监听中，检测到上架即毫秒抢占并锁单支付！",
			title, telegram.CardDivider, info.PlanCode, dcText, info.Quantity, optsText, accLine, couponLine, state.Config.RetryInterval(), telegram.CardDivider)
	}
	return fmt.Sprintf("❌ 任务创建失败\n%s\n⚠️ 原因: %s", telegram.CardDivider, result.Message)
}

func cmdMonitor(state *app.State, mon *monitor.Monitor, args []string) string {
	if mon == nil {
		return "❌ 监控模块不可用"
	}
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		return "用法: /monitor <planCode> [datacenter...]\n例: /monitor 24ska01\n例: /monitor 24ska01 gra rbx"
	}
	// 监控依赖 TG 通知自身
	if ok, reason := telegram.VerifyConfig(state); !ok {
		return "❌ Telegram 配置无效: " + reason
	}
	planCode := strings.TrimSpace(args[0])
	dcs := []string{}
	for _, a := range args[1:] {
		a = strings.ToLower(strings.TrimSpace(a))
		if a != "" {
			dcs = append(dcs, a)
		}
	}

	var serverName string
	state.ServerPlansMu.RLock()
	for _, s := range state.ServerPlans {
		if s.PlanCode == planCode {
			serverName = s.Name
			break
		}
	}
	state.ServerPlansMu.RUnlock()

	mon.AddSubscription(planCode, dcs, true, false, serverName, nil, nil, false, 0, "", false, nil)
	mon.SaveToDB()
	if !mon.Running() {
		mon.Start()
		state.Logger.Info("Telegram /monitor 添加订阅后自动启动监控", "telegram")
	}

	dcText := "全球全部机房"
	if len(dcs) > 0 {
		up := make([]string, len(dcs))
		for i, d := range dcs {
			up[i] = telegram.DisplayDCFull(d)
		}
		dcText = strings.Join(up, "\n  • ")
	}
	namePart := planCode
	if serverName != "" {
		namePart = planCode + " (" + serverName + ")"
	}
	return fmt.Sprintf("✅ 补货监控已成功建立！\n%s\n📦 目标型号: %s\n📍 监控机房:\n  • %s\n⚙️ 监控规格: 全部硬件规格组合\n%s\n📡 状态: 全天候毫秒级监听中，官方一旦上架即刻推送警报并支持一键秒抢！",
		telegram.CardDivider, namePart, dcText, telegram.CardDivider)
}

func cmdPrice(state *app.State, args []string, monOpt ...*monitor.Monitor) string {
	if len(args) < 2 {
		return "用法: /price <planCode> <datacenter> [@account]\n例: /price 24ska01 gra"
	}
	planCode := strings.TrimSpace(args[0])
	dc := strings.ToLower(strings.TrimSpace(args[1]))
	if planCode == "" || dc == "" {
		return "用法: /price <planCode> <datacenter>\n例: /price 24ska01 gra"
	}
	if !state.HasAnyAccount() {
		return "❌ 未配置任何 OVH 账户"
	}
	var mon *monitor.Monitor
	if len(monOpt) > 0 {
		mon = monOpt[0]
	}
	accountID := telegram.DefaultAccountID(state)
	var explicitAccount *types.OVHAccount
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "@") && len(a) > 1 {
			accs, err := resolveAccountRef(state, mon, a[1:], planCode)
			if err == "" && len(accs) > 0 {
				accountID = accs[0].ID
				explicitAccount = &accs[0]
				break
			}
		}
	}
	if explicitAccount == nil && mon != nil {
		ra := resolveOrderAccount(state, mon, planCode)
		if ra.Account.ID != "" {
			accountID = ra.Account.ID
		}
	}

	validDCs, _ := validAndInStockDCs(state, planCode, accountID)
	if len(validDCs) > 0 {
		dcSupported := false
		for _, vd := range validDCs {
			if vd == dc {
				dcSupported = true
				break
			}
		}
		if !dcSupported {
			var up []string
			for _, v := range validDCs {
				up = append(up, telegram.DisplayDC(v))
			}
			return fmt.Sprintf("❌ 无法在 %s 查询价格\n\n型号 %s 在该机房未提供（属于区域专用型号）。\n\n该型号支持的数据中心:\n%s\n\n💡 建议命令: /price %s %s",
				telegram.DisplayDC(dc), planCode, strings.Join(up, "、"), planCode, validDCs[0])
		}
	}

	// 尝试取该机房任一有货配置的 options 再询价
	options := []string{}
	avail := catalog.CheckServerAvailabilityWithConfigs(state, planCode, accountID)
	for _, cfg := range avail {
		if st, ok := cfg.Datacenters[dc]; ok && st != "" && st != "unavailable" && st != "unknown" {
			if len(cfg.Options) > 0 {
				options = append([]string{}, cfg.Options...)
				break
			}
		}
	}
	// 无货时仍尝试用第一套配置询价（OVH 常允许对无货组合询价）
	if len(options) == 0 {
		for _, cfg := range avail {
			if len(cfg.Options) > 0 {
				options = append([]string{}, cfg.Options...)
				break
			}
		}
	}

	result := price.GetInternal(state, accountID, planCode, dc, options, "")
	if !result.Success {
		err := result.Error
		if err == "" {
			err = "询价失败"
		}
		return "❌ 价格查询失败\n\n" + err
	}

	var withTax, withoutTax interface{}
	currency := ""
	if result.Price != nil && result.Price.Prices != nil {
		withTax = result.Price.Prices["withTax"]
		withoutTax = result.Price.Prices["withoutTax"]
		if c, ok := result.Price.Prices["currencyCode"].(string); ok && c != "" {
			currency = c
		}
	}
	if currency == "" {
		currency = "EUR"
	}
	// 尝试从 items 里找货币
	optsText := "任意/匹配硬件规格"
	if len(options) > 0 {
		optsText = telegram.HumanizeOptionCodes(options)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("💰 官方结算价格核算 · %s\n", planCode))
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("📍 结算节点: " + telegram.DisplayDCFull(dc) + "\n")
	b.WriteString("⚙️ 计费规格: " + optsText + "\n")
	p, pErr := catalog.PriceForOptions(state, accountID, planCode, options)
	if pErr == nil && p.Monthly > 0 {
		b.WriteString(fmt.Sprintf("💵 官方月付续费: %.2f %s / 月\n", p.Monthly, p.Currency))
		if p.Install > 0 {
			b.WriteString(fmt.Sprintf("📦 一次性安装费: %.2f %s\n", p.Install, p.Currency))
			b.WriteString(fmt.Sprintf("🧾 首月订单总计: %.2f %s（含安装费）\n", p.Monthly+p.Install, p.Currency))
		}
	} else {
		if withoutTax != nil {
			b.WriteString(fmt.Sprintf("💵 官方月付续费: %v %s / 月\n", withoutTax, currency))
		} else if withTax != nil {
			b.WriteString(fmt.Sprintf("💵 官方月付续费: %v %s / 月\n", withTax, currency))
		}
	}
	if pErr != nil && withTax == nil && withoutTax == nil {
		b.WriteString("ℹ️（官方实时账单接口未返回具体金额，请在 WebUI 查看）\n")
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString(fmt.Sprintf("⚡ 极速下单: /buy %s %s", planCode, dc))
	return b.String()
}

// intervalText /interval 看或改新建任务的默认重试间隔。
// 只改默认值:已经在跑的任务各自带着自己的间隔,要改单个任务去网页「抢购队列」里点秒数。
func intervalText(state *app.State, args []string) string {
	cur := state.Config.RetryInterval()
	quick := state.Config.QuickOrderRetryInterval()
	if len(args) == 0 {
		return fmt.Sprintf("⏱️ 轮询重试周期参数\n%s\n⚡ 新任务默认重试间隔: %d 秒\n📡 监控自动下单间隔: %d 秒\n%s\n💡 修改默认值: /interval <秒> (%d ~ %d)\n提示: 已在运行中任务的间隔可在 WebUI「抢购队列」中实时调整。",
			telegram.CardDivider, cur, quick, telegram.CardDivider, types.MinRetryInterval, types.MaxRetryInterval)
	}
	n, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil || n < types.MinRetryInterval || n > types.MaxRetryInterval {
		return fmt.Sprintf("⚠️ 间隔需为 %d ~ %d 之间的整数秒，例如: /interval 30",
			types.MinRetryInterval, types.MaxRetryInterval)
	}
	cfg := state.Config.Get()
	cfg.DefaultRetryInterval = n
	if err := state.Config.Set(cfg); err != nil {
		return "❌ 保存失败: " + err.Error()
	}
	state.Logger.Info(fmt.Sprintf("Telegram 把默认重试间隔从 %d 改为 %d 秒", cur, n), "telegram")
	return fmt.Sprintf("✅ 默认重试间隔已更新！\n%s\n⏱️ 当前设定: %d 秒 / 轮 (原设定: %d 秒)\n💡 该设置将应用于后续所有新建的抢购任务。", telegram.CardDivider, n, cur)
}

func cmdTasksText(state *app.State) string {
	snapshot := state.QueueSnapshot()
	active := make([]types.QueueItem, 0)
	for _, item := range snapshot {
		if item.Status == "running" || item.Status == "pending" {
			active = append(active, item)
		}
	}
	if len(active) == 0 {
		return "📋 当前抢购任务队列\n" + telegram.CardDivider + "\n当前没有任何进行中的抢购任务。\n\n💡 发送 /buy <型号> <机房> 即可快速入队抢购！"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📋 当前抢购任务队列 (共 %d 个活跃任务)\n", len(active)))
	b.WriteString(telegram.CardDivider + "\n")
	for i, it := range active {
		shortID := it.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		accTag := it.AccountID
		if acc, ok := state.FindAccount(it.AccountID); ok {
			accTag = acc.Name
			if acc.Zone != "" {
				accTag += " (" + strings.ToUpper(acc.Zone) + ")"
			}
		}
		dcText := telegram.DisplayDC(it.Datacenter)
		statusDesc := "🟢 正在重试抢购"
		if it.Status == "pending" {
			statusDesc = "🟡 排队等待"
		}
		b.WriteString(fmt.Sprintf("%d. [%s] 📦 %s @ %s\n   %s | 👤 %s | 🔄 %d次 | ⏱️ %ds\n",
			i+1, shortID, it.PlanCode, dcText, statusDesc, accTag, it.RetryCount, it.RetryInterval))
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("💡 取消任务: /cancel <ID前缀> | 取消全部: /cancel all")
	return b.String()
}

func cmdAccountsText(state *app.State) string {
	all := listAccounts(state)
	if len(all) == 0 {
		return "❌ 尚未配置任何 OVH 账户，请先在 Web 控制台「设置 → OVH 账户」添加。"
	}
	active, _ := telegram.ActiveAccount(state)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("👤 已配置的 OVH 账户矩阵 (共 %d 个)\n", len(all)))
	b.WriteString(telegram.CardDivider + "\n")
	for i, a := range all {
		mark := ""
		if a.ID == active.ID {
			mark = " 🌟 [当前默认]"
		}
		zone := strings.ToUpper(strings.TrimSpace(a.Zone))
		if zone == "" {
			zone = "EU"
		}
		b.WriteString(fmt.Sprintf("%d. %s (%s) — %s%s\n   别名: @%d 或 @%s\n",
			i+1, a.Name, zone, a.Endpoint, mark, i+1, strings.ToLower(zone)))
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("💡 下单指定账户: /buy <型号> <机房> @<别名>\n   多账户齐开抢: /buy <型号> <机房> @all")
	return b.String()
}

func cmdSubs(state *app.State, mon *monitor.Monitor) string {
	if mon == nil {
		return "❌ 监控模块不可用"
	}
	subs := mon.Snapshot()
	if len(subs) == 0 {
		return "📡 当前没有任何监控订阅。\n\n💡 发送 /watch <型号> [机房] 即可添加监控！"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📡 当前监控订阅列表 (共 %d 项)\n", len(subs)))
	b.WriteString(telegram.CardDivider + "\n")
	for i, s := range subs {
		dcText := "全部机房"
		if len(s.Datacenters) > 0 {
			dcText = strings.ToUpper(strings.Join(s.Datacenters, ", "))
		}
		orderText := "只发通知"
		if s.AutoOrder {
			orderText = fmt.Sprintf("补货自动抢 %d 台", s.Quantity)
		}
		statusText := "🟢 运行中"
		if s.Paused {
			statusText = "⏸️ 已暂停"
		}
		b.WriteString(fmt.Sprintf("%d. 📦 %s [%s]\n   📍 机房: %s\n   ⚡ 模式: %s\n",
			i+1, s.PlanCode, statusText, dcText, orderText))
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("💡 取消监控: /unwatch <型号>")
	return b.String()
}

func cmdCancel(state *app.State, args []string) string {
	return cancelText(state, args)
}

// cancelText /cancel <ID前缀|all>:真正删除任务(取消进行中的下单 + 自动建单停止记录 + 落库)。
//
// 以前这里持有 QueueMu 再调 SaveQueue(内部又锁 QueueMu)—— 第一次成功取消就把整条队列死锁;
// 而且只是把状态改成队列处理器不认识的 "cancelled",进行中的下单不会被中断,监控还会重新建单。
func cancelText(state *app.State, args []string) string {
	if len(args) == 0 {
		return "用法: /cancel <任务ID前缀 或 all>\n例: /cancel all (取消全部任务)\n例: /cancel a1b2c3d4"
	}
	target := strings.TrimSpace(args[0])
	active := func(it types.QueueItem) bool { return it.Status == "running" || it.Status == "pending" }

	var match func(types.QueueItem) bool
	if strings.EqualFold(target, "all") {
		match = active
	} else {
		id, ok := state.ResolveQueueID(target)
		if !ok {
			return fmt.Sprintf("未找到前缀为 %s 的唯一抢购任务(前缀不存在或匹配到多个)。发 /tasks 查看当前任务列表。", target)
		}
		match = func(it types.QueueItem) bool { return it.ID == id && active(it) }
	}

	removed, err := state.RemoveQueueItemsWhere(match)
	if len(removed) == 0 {
		if strings.EqualFold(target, "all") {
			return "当前没有任何运行中或排队中的任务可取消。"
		}
		return fmt.Sprintf("任务 %s 当前不在运行或排队中，无需取消。", target)
	}
	for i := range removed {
		telegram.NotifyTaskProgress(state, &removed[i], "cancelled", map[string]string{"reason": "已在 Telegram 中取消此任务"})
	}
	if err != nil {
		state.Logger.Error("Telegram 取消任务后保存队列失败: "+err.Error(), "telegram")
		return fmt.Sprintf("⚠️ 已从运行中的队列移除 %d 个任务，但没能写进数据库，重启后会重新出现：\n%s", len(removed), err.Error())
	}
	state.Logger.Info(fmt.Sprintf("Telegram 取消了 %d 个抢购任务", len(removed)), "telegram")
	return fmt.Sprintf("🛑 已成功取消 %d 个抢购任务。", len(removed))
}

// handleTelegramText 统一处理 webhook 普通文本：斜杠命令 / free-form 下单 / 帮助。
func handleTelegramText(state *app.State, mon *monitor.Monitor, text string, chatID, userID interface{}, messageID float64) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	// Chat ID 或 User ID 白名单
	authorized := telegram.IsAuthorizedActor(state, chatID, userID)

	// 1) 斜杠命令或中文快捷指令
	cmd := telegram.ParseBotCommand(text)
	if cmd == nil {
		trimmed := strings.TrimSpace(text)
		lower := strings.ToLower(trimmed)
		switch {
		case lower == "⚡ 快速下单" || lower == "快速下单" || lower == "买" || lower == "抢" || lower == "下单":
			cmd = &telegram.BotCommand{Name: "buy"}
		case lower == "📋 抢购任务" || lower == "抢购任务" || lower == "任务" || lower == "队列" || lower == "任务列表":
			cmd = &telegram.BotCommand{Name: "tasks"}
		case lower == "🎫 优惠码" || lower == "优惠码" || lower == "coupon":
			cmd = &telegram.BotCommand{Name: "coupon"}
		case lower == "👤 账户矩阵" || lower == "账户矩阵" || lower == "账户" || lower == "账号" || lower == "切换账户":
			cmd = &telegram.BotCommand{Name: "accounts"}
		case lower == "📦 查询库存" || lower == "查询库存" || lower == "库存" || lower == "查库存":
			cmd = &telegram.BotCommand{Name: "stock"}
		case lower == "🎛️ 控制中心" || lower == "控制中心" || lower == "控制台" || lower == "主菜单" || lower == "中枢":
			cmd = &telegram.BotCommand{Name: "start"}
		case lower == "监控" || lower == "监控列表":
			cmd = &telegram.BotCommand{Name: "monitor"}
		case lower == "价格" || lower == "查价格":
			cmd = &telegram.BotCommand{Name: "price"}
		case lower == "订阅" || lower == "订阅列表":
			cmd = &telegram.BotCommand{Name: "subs"}
		case lower == "取消" || strings.HasPrefix(lower, "取消 "):
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "cancel", Args: fields[1:]}
			} else {
				cmd = &telegram.BotCommand{Name: "cancel"}
			}
		case strings.HasPrefix(lower, "查库存 ") || strings.HasPrefix(lower, "库存 ") || strings.HasPrefix(lower, "查 "):
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "stock", Args: fields[1:]}
			}
		case strings.HasPrefix(lower, "监控 "):
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "monitor", Args: fields[1:]}
			}
		case strings.HasPrefix(lower, "查价格 ") || strings.HasPrefix(lower, "价格 "):
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "price", Args: fields[1:]}
			}
		case strings.HasPrefix(lower, "买 ") || strings.HasPrefix(lower, "抢 ") || strings.HasPrefix(lower, "下单 "):
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "buy", Args: fields[1:]}
			}
		case strings.HasPrefix(lower, "优惠码 ") || strings.HasPrefix(lower, "coupon "):
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "coupon", Args: fields[1:]}
			}
		}
	}
	if cmd != nil {
		if cmd.Name == "start" {
			// 1. 发送底部常驻快捷键盘（在移动端持久驻留于输入框下方，随时可点）
			_, _ = telegram.SendToChat(state, chatID, "⚡ 控制中枢已连接", telegram.PersistentReplyKeyboard())
			// 2. 发送 HUD 动态控制中心与内联操作中枢
			_, _ = telegram.SendToChat(state, chatID, hudDashboardMessage(state, mon), dashboardKeyboard(state, mon))
			return
		}
		if cmd.Name == "help" {
			markup := telegram.InlineKeyboard([][]map[string]string{
				{telegram.CallbackButton("🔙 返回控制中心", "i:dash:refresh")},
			})
			_, _ = telegram.SendToChat(state, chatID, telegram.HelpMessage(), markup)
			return
		}
		if !authorized {
			state.Logger.Warn(fmt.Sprintf("拒绝未授权: chat=%v user=%v cmd=/%s", chatID, userID, cmd.Name), "telegram")
			telegram.SendReply(state, chatID,
				"❌ 未授权。请在网页「设置」中配置正确的 Telegram Chat ID，并确保用该会话发消息。",
				int64(messageID))
			return
		}
		if !telegram.IsKnownCommand(cmd.Name) {
			telegram.SendReply(state, chatID,
				"❌ 未知命令: /"+cmd.Name+"\n\n"+telegram.HelpMessage(),
				int64(messageID))
			return
		}
		if cmd.Name == "stock" {
			if len(cmd.Args) == 0 {
				startPlanPicker(state, mon, chatID, int64(messageID), "s")
				return
			}
			showStockCardWithButtons(state, mon, chatID, int64(messageID), cmd.Args[0])
			return
		}
		if cmd.Name == "monitor" {
			if len(cmd.Args) == 0 {
				showMonitorManager(state, mon, chatID, int64(messageID), false)
				return
			}
			if len(cmd.Args) == 1 {
				showDCPicker(state, chatID, 0, "m", cmd.Args[0], false)
				return
			}
			// 快捷式: /monitor <型号> <机房...> 立刻建订阅(盯全部配置)。
			// addon planCode 二三十字符打不出来,所以建完紧跟一排按钮一键改窄。
			reply := cmdMonitor(state, mon, cmd.Args)
			telegram.SendReply(state, chatID, reply, int64(messageID))
			if !strings.HasPrefix(reply, "❌") {
				offerNarrowConfig(state, chatID, int64(messageID), strings.TrimSpace(cmd.Args[0]))
			}
			return
		}
		if cmd.Name == "price" {
			if len(cmd.Args) == 0 {
				startPlanPicker(state, mon, chatID, int64(messageID), "pr")
				return
			}
			if len(cmd.Args) == 1 {
				showDCPicker(state, chatID, 0, "pr", cmd.Args[0], false)
				return
			}
			rawPrice := cmdPrice(state, cmd.Args)
			plan, dc := cmd.Args[0], cmd.Args[1]
			var markup map[string]interface{}
			if strings.HasPrefix(rawPrice, "❌") {
				validDCs, _ := validAndInStockDCs(state, plan, "")
				var rows [][]map[string]string
				if len(validDCs) > 0 && validDCs[0] != dc {
					rows = append(rows, []map[string]string{
						telegram.CallbackButton("💰 查看 "+telegram.DisplayDC(validDCs[0])+" 价格", "i:D:pr:"+plan+":"+validDCs[0]),
					})
				}
				rows = append(rows, []map[string]string{
					telegram.CallbackButton("🔙 重新选择机型", "i:cat:pr:root"),
				})
				markup = telegram.InlineKeyboard(rows)
			} else {
				markup = telegram.InlineKeyboard([][]map[string]string{
					{
						telegram.CallbackButton("⚡ "+telegram.DisplayDC(dc)+" 立即开抢", "i:D:b:"+plan+":"+dc),
						telegram.CallbackButton("👀 监控此型号", "i:M:"+plan),
					},
					{
						telegram.CallbackButton("🔙 重新询价", "i:cat:pr:root"),
					},
				})
			}
			_, _ = telegram.SendToChat(state, chatID, rawPrice, markup)
			return
		}
		if cmd.Name == "tasks" {
			showTasks(state, chatID, 0, false)
			return
		}
		if cmd.Name == "accounts" {
			showAccounts(state, chatID, 0, false)
			return
		}
		if cmd.Name == "buy" || cmd.Name == "queue" {
			mode := "b"
			if cmd.Name == "queue" {
				mode = "q"
			}
			if len(cmd.Args) == 0 {
				startPlanPicker(state, mon, chatID, int64(messageID), mode)
				return
			}
			info := telegram.ParseOrderArgs(cmd.Args)
			if startTelegramOrder(state, mon, chatID, cmd.Name, info) {
				return
			}
			telegram.SendReply(state, chatID, cmdBuyOrQueueInfo(state, mon, info, cmd.Name), int64(messageID))
			return
		}
		reply := dispatchTelegramCommand(state, mon, cmd)
		telegram.SendReply(state, chatID, reply, int64(messageID))
		return
	}

	// 2) free-form 下单: planCode [dc] [qty] [options]
	if !authorized {
		state.Logger.Debug(fmt.Sprintf("忽略未授权消息: chat=%v user=%v", chatID, userID), "telegram")
		return
	}
	orderInfo := telegram.ParseOrderMessage(text)
	if orderInfo == nil || orderInfo.PlanCode == "" {
		if strings.EqualFold(text, "help") || text == "?" || text == "帮助" {
			_, _ = telegram.SendToChat(state, chatID, telegram.HelpMessage(), dashboardKeyboard(state, mon))
		}
		// 严格拒绝无法识别的文本（避免误入队）
		return
	}
	if strings.HasPrefix(orderInfo.PlanCode, "/") {
		telegram.SendReply(state, chatID, "❌ 未知命令\n\n"+telegram.HelpMessage(), int64(messageID))
		return
	}

	state.Logger.Info(fmt.Sprintf("解析 free-form 下单: planCode=%s, datacenter=%s, quantity=%d, options=%v, ref=%s",
		orderInfo.PlanCode, orderInfo.Datacenter, orderInfo.Quantity, orderInfo.Options, orderInfo.AccountRef), "telegram")

	// 智能型号/关键词模糊识别与直达
	canonical, matches := findMatchingPlans(state, orderInfo.PlanCode)
	if canonical != "" {
		orderInfo.PlanCode = canonical
	}
	if orderInfo.Datacenter == "" && len(orderInfo.Options) == 0 && orderInfo.AccountRef == "" {
		if canonical != "" {
			// 精确或唯一命中，直接弹出机房选择卡片 (Screen 2)
			showDCPicker(state, chatID, 0, "b", canonical, false)
			return
		} else if len(matches) > 1 {
			// 命中多个候选型号，弹出精简选择列表
			renderMatchedPlansPicker(state, mon, chatID, int64(messageID), "b", orderInfo.PlanCode, matches)
			return
		}
	}

	if orderInfo.AccountRef != "" {
		accs, err := resolveAccountRef(state, mon, orderInfo.AccountRef, orderInfo.PlanCode)
		if err != "" {
			telegram.SendReply(state, chatID, "❌ "+err, int64(messageID))
			return
		}
		telegram.SendReply(state, chatID, runOrder(state, orderInfo, accs), int64(messageID))
		return
	}

	if startTelegramOrder(state, mon, chatID, "buy", orderInfo) {
		return
	}

	resolved := resolveOrderAccount(state, mon, orderInfo.PlanCode)
	if resolved.Reason != "" {
		telegram.SendReply(state, chatID, "❌ "+resolved.Reason, int64(messageID))
		return
	}
	acc := resolved.Account
	result := telegram.ProcessOrder(state, acc.ID, orderInfo.PlanCode, orderInfo.Datacenter, orderInfo.Quantity, orderInfo.Options, orderInfo.Coupon)
	var reply string
	if result.Success {
		dcText := "自动选择机房"
		if orderInfo.Datacenter != "" {
			dcText = telegram.DisplayDCFull(orderInfo.Datacenter)
		}
		optsText := "匹配配置"
		if len(orderInfo.Options) > 0 {
			optsText = strings.Join(orderInfo.Options, ", ")
		}
		accExplain := explainAccountChoice(resolved, orderInfo.PlanCode)
		couponText := ""
		if orderInfo.Coupon != "" {
			couponText = fmt.Sprintf("\n🎫 优惠码: %s", orderInfo.Coupon)
		}
		reply = fmt.Sprintf("📥 已成功创建 %d/%d 个抢购任务\n\n📦 型号: %s\n📍 机房: %s\n🔢 数量: %d\n⚙️ 配置: %s%s\n%s\n⏱ 重试间隔: %d 秒\n\n将按「设置 → 抢购参数」的间隔重试。锁单成功≠已付款。",
			result.CreatedOrders, result.TotalOrders, orderInfo.PlanCode, dcText, telegram.ClampQuantity(orderInfo.Quantity), optsText, couponText, accExplain, state.Config.RetryInterval())
	} else {
		reply = "❌ 任务创建失败\n\n" + result.Message
	}
	telegram.SendReply(state, chatID, reply, int64(messageID))
}

func hudDashboardMessage(state *app.State, mon *monitor.Monitor) string {
	accText := "未配置 OVH 账户"
	if state != nil {
		if acc, ok := telegram.ActiveAccount(state); ok && acc.ID != "" {
			zone := strings.ToUpper(strings.TrimSpace(acc.Zone))
			if zone == "" {
				zone = "EU"
			}
			accText = fmt.Sprintf("%s · %s (%s 区 · 共 %d 个账户)", acc.Name, zone, ovh.SubsidiaryRegion(zone), len(state.Accounts))
		}
	}

	coupon := ""
	couponText := "未设置 (点击下方管理)"
	if state != nil {
		coupon = telegram.ActiveCoupon(state)
		if coupon != "" {
			couponText = coupon + " (抢购自动抵扣已就绪)"
		}
	}

	taskText := "队列空闲 (暂无抢购任务)"
	if state != nil {
		state.QueueMu.Lock()
		activeTasks := 0
		for _, it := range state.Queue {
			if it.Status == "running" || it.Status == "pending" {
				activeTasks++
			}
		}
		state.QueueMu.Unlock()
		if activeTasks > 0 {
			taskText = fmt.Sprintf("%d 个任务挂机抢购中 (%d 秒轮询)", activeTasks, state.Config.RetryInterval())
		}
	}

	monText := "暂无监控机型"
	if mon != nil {
		monCount := len(mon.Snapshot())
		if monCount > 0 {
			monText = fmt.Sprintf("%d 款机型全天候毫秒监听中", monCount)
		}
	}

	cmdExample := "/buy 24ska01 gra 1"
	if coupon != "" {
		cmdExample += " coupon=" + coupon
	}

	var b strings.Builder
	b.WriteString("⚡ OVH CLOUD TERMINAL · 智能控制中枢 ⚡\n")
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString(fmt.Sprintf("👤 活跃账户: %s\n", accText))
	b.WriteString(fmt.Sprintf("🎫 活跃优惠码: %s\n", couponText))
	b.WriteString(fmt.Sprintf("🎯 抢购队列: %s\n", taskText))
	b.WriteString(fmt.Sprintf("📡 库存监控: %s\n", monText))
	b.WriteString("⏱️ 极速引擎: 🟢 运行中 · 毫秒级锁单\n")
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("💡 快捷下单指令 (点击即可复制):\n`" + cmdExample + "`\n\n")
	b.WriteString("📌 点击下方快捷按钮直达各项操作 👇")
	return b.String()
}

func dashboardKeyboard(state *app.State, mon *monitor.Monitor) map[string]interface{} {
	taskCount := 0
	if state != nil {
		state.QueueMu.Lock()
		for _, it := range state.Queue {
			if it.Status == "running" || it.Status == "pending" {
				taskCount++
			}
		}
		state.QueueMu.Unlock()
	}
	taskLabel := "📋 抢购任务"
	if taskCount > 0 {
		taskLabel = fmt.Sprintf("📋 抢购任务 (%d)", taskCount)
	}

	monCount := 0
	if mon != nil {
		monCount = len(mon.Snapshot())
	}
	monLabel := "📡 监控管理"
	if monCount > 0 {
		monLabel = fmt.Sprintf("📡 监控管理 (%d)", monCount)
	}

	accLabel := "👤 账户矩阵"
	if state != nil {
		if acc, ok := telegram.ActiveAccount(state); ok && acc.ID != "" {
			z := strings.ToUpper(strings.TrimSpace(acc.Zone))
			if z == "" {
				z = "EU"
			}
			accLabel = fmt.Sprintf("👤 账户矩阵 (%s★)", z)
		}
	}

	cpLabel := "🎫 优惠码管理"
	if state != nil {
		if cp := telegram.ActiveCoupon(state); cp != "" {
			cpLabel = fmt.Sprintf("🎫 优惠码 (%s)", cp)
		}
	}

	ivLabel := "⏱️ 抢购参数"
	if state != nil {
		iv := state.Config.RetryInterval()
		ivLabel = fmt.Sprintf("⏱️ 抢购参数 (%ds)", iv)
	}

	return telegram.InlineKeyboard([][]map[string]string{
		{
			telegram.CallbackButton("⚡ 极速选机下单", "i:cat:b:root"),
			telegram.CallbackButton(taskLabel, "i:Tk:list"),
		},
		{
			telegram.CallbackButton("📦 现货库存查询", "i:cat:s:root"),
			telegram.CallbackButton(monLabel, "i:mon:list"),
		},
		{
			telegram.CallbackButton(accLabel, "i:acc:list"),
			telegram.CallbackButton(cpLabel, "i:cp:manage"),
		},
		{
			telegram.CallbackButton(ivLabel, "i:dash:iv"),
			telegram.CallbackButton("🔄 刷新中枢", "i:dash:refresh"),
		},
	})
}

// cmdCoupon 查看或设置当前 Telegram 默认优惠码。
func cmdCoupon(state *app.State, args []string) string {
	if len(args) == 0 {
		cur := telegram.ActiveCoupon(state)
		if cur == "" {
			return fmt.Sprintf("🎫 当前未设置默认优惠码\n%s\n"+
				"💡 设置默认优惠码后，所有通过 Telegram 点击通知抢购、入队或文本下单都会自动带入该优惠码。\n\n"+
				"📌 指令用法:\n"+
				"  • 设置优惠码: /coupon <代码> (例如: /coupon SAVE10)\n"+
				"  • 清除优惠码: /coupon clear", telegram.CardDivider)
		}
		return fmt.Sprintf("🎫 当前默认优惠码: %s\n%s\n"+
			"✅ 状态: 激活中\n"+
			"🚀 效果: 所有 Telegram 下单与监控通知 1-Click 按钮将自动带入该优惠码！\n\n"+
			"📌 操作指引:\n"+
			"  • 更换优惠码: /coupon <新代码>\n"+
			"  • 清除优惠码: /coupon clear", cur, telegram.CardDivider)
	}

	arg := strings.TrimSpace(args[0])
	lower := strings.ToLower(arg)
	if lower == "clear" || lower == "none" || lower == "0" || lower == "off" || lower == "del" || lower == "delete" {
		_ = telegram.ClearActiveCoupon(state)
		return fmt.Sprintf("✅ 已清除默认优惠码\n%s\n后续 Telegram 下单将默认不使用优惠码。", telegram.CardDivider)
	}

	if err := telegram.SetActiveCoupon(state, arg); err != nil {
		return fmt.Sprintf("❌ 保存优惠码失败: %v", err)
	}
	return fmt.Sprintf("✅ 默认优惠码设置成功！\n%s\n"+
		"🎫 当前代码: %s\n"+
		"🚀 无论是点击 Telegram 监控通知上的抢购按钮，还是发送命令下单，都将自动带入该优惠码！\n\n"+
		"💡 如需清除，请发送 /coupon clear", telegram.CardDivider, arg)
}
