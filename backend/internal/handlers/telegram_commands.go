package handlers

import (
	"fmt"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/price"
	"github.com/ovh-webui/server/internal/telegram"
)

// dispatchTelegramCommand 处理 /buy /stock 等斜杠命令，返回回复文案。
func dispatchTelegramCommand(state *app.State, mon *monitor.Monitor, cmd *telegram.BotCommand) string {
	if cmd == nil {
		return telegram.HelpMessage()
	}
	switch cmd.Name {
	case "start", "help":
		return telegram.HelpMessage()
	case "stock":
		return cmdStock(state, cmd.Args)
	case "queue", "buy":
		return cmdBuyOrQueue(state, cmd.Args, cmd.Name)
	case "monitor":
		return cmdMonitor(state, mon, cmd.Args)
	case "price":
		return cmdPrice(state, cmd.Args)
	default:
		return "❌ 未知命令: /" + cmd.Name + "\n\n" + telegram.HelpMessage()
	}
}

func cmdStock(state *app.State, args []string) string {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		return "用法: /stock <planCode>\n例: /stock 24ska01"
	}
	planCode := strings.TrimSpace(args[0])
	if !state.HasAnyAccount() {
		return "❌ 未配置任何 OVH 账户"
	}
	accountID := telegram.DefaultAccountID(state)
	avail := catalog.CheckServerAvailabilityWithConfigs(state, planCode, accountID)
	if len(avail) == 0 {
		return "❌ 无法获取 " + planCode + " 的库存信息（型号可能不存在或 API 失败）"
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
			availDCs = append(availDCs, strings.ToUpper(dc)+"("+st+")")
			// 记录全局机房状态
			if prev, ok := dcStatus[dc]; !ok || prev == "unavailable" {
				dcStatus[dc] = st
			}
		}
		mem, stor := cfg.Memory, cfg.Storage
		if mem == "" {
			mem = "?"
		}
		if stor == "" {
			stor = "?"
		}
		if len(availDCs) == 0 {
			configLines = append(configLines, fmt.Sprintf("· %s / %s → 无货", mem, stor))
		} else {
			configLines = append(configLines, fmt.Sprintf("· %s / %s → %s", mem, stor, strings.Join(availDCs, ", ")))
		}
	}

	inStock := []string{}
	outStock := []string{}
	for dc, st := range dcStatus {
		if st != "" && st != "unavailable" && st != "unknown" {
			inStock = append(inStock, strings.ToUpper(dc)+"("+st+")")
		} else {
			outStock = append(outStock, strings.ToUpper(dc))
		}
	}

	var b strings.Builder
	b.WriteString("📦 库存查询: " + planCode + "\n\n")
	if len(inStock) > 0 {
		b.WriteString("✅ 有货机房: " + strings.Join(inStock, ", ") + "\n")
	} else {
		b.WriteString("❌ 当前所有机房无货\n")
	}
	if len(configLines) > 0 {
		b.WriteString("\n配置明细:\n")
		// 最多展示 12 行，避免 Telegram 消息过长
		limit := 12
		for i, line := range configLines {
			if i >= limit {
				b.WriteString(fmt.Sprintf("…另有 %d 条配置省略\n", len(configLines)-limit))
				break
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n有货时可: /buy " + planCode + " <dc>")
	return b.String()
}

func cmdBuyOrQueue(state *app.State, args []string, cmdName string) string {
	if len(args) < 1 {
		return "用法: /" + cmdName + " <planCode> [datacenter] [quantity] [options]\n例: /" + cmdName + " 24ska01 gra"
	}
	info := telegram.ParseOrderArgs(args)
	if info == nil || info.PlanCode == "" {
		return "❌ 无法解析参数\n用法: /" + cmdName + " <planCode> [datacenter] [quantity] [options]"
	}
	// 拒绝把子命令当 planCode
	if strings.HasPrefix(info.PlanCode, "/") {
		return "❌ 型号无效: " + info.PlanCode
	}

	state.Logger.Info(fmt.Sprintf("Telegram /%s: planCode=%s dc=%s qty=%d opts=%v",
		cmdName, info.PlanCode, info.Datacenter, info.Quantity, info.Options), "telegram")

	result := telegram.ProcessOrder(state, info.PlanCode, info.Datacenter, info.Quantity, info.Options)
	if result.Success {
		dcText := "所有可用机房"
		if info.Datacenter != "" {
			dcText = strings.ToUpper(info.Datacenter)
		}
		optsText := "所有可用配置"
		if len(info.Options) > 0 {
			optsText = strings.Join(info.Options, ", ")
		}
		title := "✅ 已加入抢购队列"
		if cmdName == "buy" {
			title = "✅ 快速下单已入队"
		}
		return fmt.Sprintf("%s！\n\n型号: %s\n机房: %s\n数量: %d\n配置: %s\n\n已创建: %d/%d 个订单\n系统将自动尝试下单。",
			title, info.PlanCode, dcText, info.Quantity, optsText, result.CreatedOrders, result.TotalOrders)
	}
	return "❌ 下单失败\n\n" + result.Message
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

	mon.AddSubscription(planCode, dcs, true, false, serverName, nil, nil, false, 0, "")
	mon.SaveToDB()
	if !mon.Running() {
		mon.Start()
		state.Logger.Info("Telegram /monitor 添加订阅后自动启动监控", "telegram")
	}

	dcText := "全部机房"
	if len(dcs) > 0 {
		up := make([]string, len(dcs))
		for i, d := range dcs {
			up[i] = strings.ToUpper(d)
		}
		dcText = strings.Join(up, ", ")
	}
	namePart := planCode
	if serverName != "" {
		namePart = planCode + " (" + serverName + ")"
	}
	return fmt.Sprintf("✅ 已添加监控\n\n型号: %s\n机房: %s\n有货时将通过 Telegram 通知。", namePart, dcText)
}

func cmdPrice(state *app.State, args []string) string {
	if len(args) < 2 {
		return "用法: /price <planCode> <datacenter>\n例: /price 24ska01 gra"
	}
	planCode := strings.TrimSpace(args[0])
	dc := strings.ToLower(strings.TrimSpace(args[1]))
	if planCode == "" || dc == "" {
		return "用法: /price <planCode> <datacenter>\n例: /price 24ska01 gra"
	}
	if !state.HasAnyAccount() {
		return "❌ 未配置任何 OVH 账户"
	}
	accountID := telegram.DefaultAccountID(state)

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

	result := price.GetInternal(state, accountID, planCode, dc, options)
	if !result.Success {
		err := result.Error
		if err == "" {
			err = "询价失败"
		}
		return "❌ 价格查询失败\n\n" + err
	}

	var withTax, withoutTax interface{}
	currency := ""
	if result.Price != nil {
		if result.Price.Prices != nil {
			withTax = result.Price.Prices["withTax"]
			withoutTax = result.Price.Prices["withoutTax"]
		}
	}
	// 尝试从 items 里找货币
	optsText := "默认/匹配配置"
	if len(options) > 0 {
		optsText = strings.Join(options, ", ")
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("💰 价格查询: %s @ %s\n\n", planCode, strings.ToUpper(dc)))
	b.WriteString("配置: " + optsText + "\n")
	if withTax != nil {
		b.WriteString(fmt.Sprintf("含税: %v %s\n", withTax, currency))
	}
	if withoutTax != nil {
		b.WriteString(fmt.Sprintf("未税: %v %s\n", withoutTax, currency))
	}
	if withTax == nil && withoutTax == nil {
		b.WriteString("（未返回具体金额，请在网页端查看详情）\n")
	}
	b.WriteString("\n下单: /buy " + planCode + " " + dc)
	return b.String()
}

// handleTelegramText 统一处理 webhook 普通文本：斜杠命令 / free-form 下单 / 帮助。
func handleTelegramText(state *app.State, mon *monitor.Monitor, text string, chatID, userID interface{}, messageID float64) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	// Chat ID 或 User ID 白名单
	authorized := telegram.IsAuthorizedActor(state, chatID, userID)

	// 1) 斜杠命令
	if cmd := telegram.ParseBotCommand(text); cmd != nil {
		if cmd.Name == "start" || cmd.Name == "help" {
			telegram.SendReply(state, chatID, dispatchTelegramCommand(state, mon, cmd), int64(messageID))
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
		// 未知多余参数：buy/queue 若无 planCode 在 dispatch 内拒绝
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
			telegram.SendReply(state, chatID, telegram.HelpMessage(), int64(messageID))
		}
		// 严格拒绝无法识别的文本（避免误入队）
		return
	}
	if strings.HasPrefix(orderInfo.PlanCode, "/") {
		telegram.SendReply(state, chatID, "❌ 未知命令\n\n"+telegram.HelpMessage(), int64(messageID))
		return
	}

	state.Logger.Info(fmt.Sprintf("解析 free-form 下单: planCode=%s, datacenter=%s, quantity=%d, options=%v",
		orderInfo.PlanCode, orderInfo.Datacenter, orderInfo.Quantity, orderInfo.Options), "telegram")
	result := telegram.ProcessOrder(state, orderInfo.PlanCode, orderInfo.Datacenter, orderInfo.Quantity, orderInfo.Options)
	var reply string
	if result.Success {
		dcText := "自动选择机房"
		if orderInfo.Datacenter != "" {
			dcText = strings.ToUpper(orderInfo.Datacenter)
		}
		optsText := "匹配配置"
		if len(orderInfo.Options) > 0 {
			optsText = strings.Join(orderInfo.Options, ", ")
		}
		reply = fmt.Sprintf("✅ 下单成功！\n\n型号: %s\n机房: %s\n数量: %d\n配置: %s\n\n已创建: %d/%d 个订单\n系统将自动尝试下单。",
			orderInfo.PlanCode, dcText, telegram.ClampQuantity(orderInfo.Quantity), optsText, result.CreatedOrders, result.TotalOrders)
	} else {
		reply = "❌ 下单失败\n\n" + result.Message
	}
	telegram.SendReply(state, chatID, reply, int64(messageID))
}
