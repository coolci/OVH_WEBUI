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
	case "tasks":
		return "📋 请在与 Bot 的私聊或群内发送 /tasks 即可直接交互管理抢购任务。"
	case "accounts":
		return "👤 请在与 Bot 的私聊或群内发送 /accounts 即可查看并切换下单账户。"
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
			inStock = append(inStock, telegram.DisplayDCFull(dc)+" ("+st+")")
		} else {
			outStock = append(outStock, telegram.DisplayDCFull(dc))
		}
	}

	var b strings.Builder
	b.WriteString("📦 库存查询结果\n\n")
	b.WriteString("📦 型号: " + planCode + "\n\n")
	if len(inStock) > 0 {
		b.WriteString(fmt.Sprintf("✅ 有货机房 (%d个):\n", len(inStock)))
		for _, s := range inStock {
			b.WriteString("  • " + s + "\n")
		}
	} else {
		b.WriteString("❌ 当前所有机房均缺货（支持无货挂机抢购）\n")
	}
	if len(configLines) > 0 {
		b.WriteString("\n⚙️ 配置概览:\n")
		// 最多展示 8 行，避免 Telegram 消息过长
		limit := 8
		for i, line := range configLines {
			if i >= limit {
				b.WriteString(fmt.Sprintf("…另有 %d 条配置省略\n", len(configLines)-limit))
				break
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n💡 立即下单: /buy " + planCode + " <机房代码>")
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
			dcText = telegram.DisplayDCFull(info.Datacenter)
		}
		optsText := "所有可用配置"
		if len(info.Options) > 0 {
			optsText = strings.Join(info.Options, ", ")
		}
		title := "✅ 已加入抢购队列"
		if cmdName == "buy" {
			title = "⚡ 快速下单已入队"
		}
		return fmt.Sprintf("%s\n\n📦 型号: %s\n📍 机房: %s\n🔢 数量: %d\n⚙️ 配置: %s\n\n已成功创建 %d/%d 个抢购任务，官方放货后将自动秒级尝试提交。",
			title, info.PlanCode, dcText, info.Quantity, optsText, result.CreatedOrders, result.TotalOrders)
	}
	return "❌ 任务创建失败\n\n" + result.Message
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

	mon.AddSubscription(planCode, dcs, true, false, serverName, nil, nil, false, 0, "", false)
	mon.SaveToDB()
	if !mon.Running() {
		mon.Start()
		state.Logger.Info("Telegram /monitor 添加订阅后自动启动监控", "telegram")
	}

	dcText := "全部机房"
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
	return fmt.Sprintf("✅ 已成功添加库存监控！\n\n📦 型号: %s\n📍 监控机房:\n  • %s\n\n一旦官方有货上架，Bot 将第一时间向您推送补货通知！", namePart, dcText)
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
	b.WriteString("💰 价格查询结果\n\n")
	b.WriteString("📦 型号: " + planCode + "\n")
	b.WriteString("📍 机房: " + telegram.DisplayDCFull(dc) + "\n")
	b.WriteString("⚙️ 配置: " + optsText + "\n")
	if withTax != nil {
		b.WriteString(fmt.Sprintf("💵 含税价: %v %s\n", withTax, currency))
	}
	if withoutTax != nil {
		b.WriteString(fmt.Sprintf("💴 未税价: %v %s\n", withoutTax, currency))
	}
	if withTax == nil && withoutTax == nil {
		b.WriteString("（未返回具体金额，请在网页端查看详情）\n")
	}
	b.WriteString(fmt.Sprintf("\n💡 立即下单: /buy %s %s", planCode, dc))
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

	// 1) 斜杠命令或中文快捷指令
	cmd := telegram.ParseBotCommand(text)
	if cmd == nil {
		trimmed := strings.TrimSpace(text)
		lower := strings.ToLower(trimmed)
		if lower == "监控" || lower == "监控列表" {
			cmd = &telegram.BotCommand{Name: "monitor"}
		} else if lower == "库存" || lower == "查库存" {
			cmd = &telegram.BotCommand{Name: "stock"}
		} else if lower == "价格" || lower == "查价格" {
			cmd = &telegram.BotCommand{Name: "price"}
		} else if lower == "任务" || lower == "队列" || lower == "任务列表" {
			cmd = &telegram.BotCommand{Name: "tasks"}
		} else if lower == "账户" || lower == "账号" || lower == "切换账户" {
			cmd = &telegram.BotCommand{Name: "accounts"}
		} else if lower == "买" || lower == "抢" || lower == "下单" {
			cmd = &telegram.BotCommand{Name: "buy"}
		} else if strings.HasPrefix(lower, "查库存 ") || strings.HasPrefix(lower, "库存 ") || strings.HasPrefix(lower, "查 ") {
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "stock", Args: fields[1:]}
			}
		} else if strings.HasPrefix(lower, "监控 ") {
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "monitor", Args: fields[1:]}
			}
		} else if strings.HasPrefix(lower, "查价格 ") || strings.HasPrefix(lower, "价格 ") {
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "price", Args: fields[1:]}
			}
		} else if strings.HasPrefix(lower, "买 ") || strings.HasPrefix(lower, "抢 ") || strings.HasPrefix(lower, "下单 ") {
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				cmd = &telegram.BotCommand{Name: "buy", Args: fields[1:]}
			}
		}
	}
	if cmd != nil {
		if cmd.Name == "start" || cmd.Name == "help" {
			markup := telegram.InlineKeyboard([][]map[string]string{
				{
					telegram.CallbackButton("⚡ 快速下单", "i:cat:b:root"),
					telegram.CallbackButton("📥 抢购排队", "i:cat:q:root"),
				},
				{
					telegram.CallbackButton("📦 查询库存", "i:cat:s:root"),
					telegram.CallbackButton("👀 监控管理", "i:mon:list"),
				},
				{
					telegram.CallbackButton("💰 价格查询", "i:cat:pr:root"),
					telegram.CallbackButton("📋 抢购任务", "i:Tk:list"),
				},
				{
					telegram.CallbackButton("👤 账户管理", "i:acc:list"),
				},
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
			if info != nil && info.PlanCode != "" && info.Datacenter == "" {
				showDCPicker(state, chatID, 0, mode, info.PlanCode, false)
				return
			}
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
			markup := telegram.InlineKeyboard([][]map[string]string{
				{
					telegram.CallbackButton("⚡ 快速下单", "i:cat:b:root"),
					telegram.CallbackButton("📥 抢购排队", "i:cat:q:root"),
				},
				{
					telegram.CallbackButton("📦 查询库存", "i:cat:s:root"),
					telegram.CallbackButton("👀 监控管理", "i:mon:list"),
				},
				{
					telegram.CallbackButton("💰 价格查询", "i:cat:pr:root"),
					telegram.CallbackButton("📋 抢购任务", "i:Tk:list"),
				},
				{
					telegram.CallbackButton("👤 账户管理", "i:acc:list"),
				},
			})
			_, _ = telegram.SendToChat(state, chatID, telegram.HelpMessage(), markup)
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
			dcText = telegram.DisplayDCFull(orderInfo.Datacenter)
		}
		optsText := "匹配配置"
		if len(orderInfo.Options) > 0 {
			optsText = strings.Join(orderInfo.Options, ", ")
		}
		reply = fmt.Sprintf("📥 已成功创建 %d/%d 个抢购任务\n\n📦 型号: %s\n📍 机房: %s\n🔢 数量: %d\n⚙️ 配置: %s\n\n系统已进入秒级监控与轮询排队，锁单成功后将第一时间通知（注意：锁单成功≠已付款）。",
			result.CreatedOrders, result.TotalOrders, orderInfo.PlanCode, dcText, telegram.ClampQuantity(orderInfo.Quantity), optsText)
	} else {
		reply = "❌ 任务创建失败\n\n" + result.Message
	}
	telegram.SendReply(state, chatID, reply, int64(messageID))
}
