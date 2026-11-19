package handlers

import (
	"fmt"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/telegram"
)

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
		if !authorized {
			state.Logger.Warn(fmt.Sprintf("拒绝未授权: chat=%v user=%v cmd=/%s", chatID, userID, cmd.Name), "telegram")
			telegram.SendReply(state, chatID,
				"❌ 未授权。请在网页「设置」中配置正确的 Telegram Chat ID，并确保用该会话发消息。",
				int64(messageID))
			return
		}
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
			rawPrice := cmdPrice(state, cmd.Args, mon)
			plan, dc := cmd.Args[0], cmd.Args[1]
			planRef := wizardPlanCallbackRef(state, plan)
			var markup map[string]interface{}
			if strings.HasPrefix(rawPrice, "❌") {
				validDCs, _ := validAndInStockDCs(state, plan, "")
				var rows [][]map[string]string
				if len(validDCs) > 0 && validDCs[0] != dc {
					rows = append(rows, []map[string]string{
						telegram.CallbackButton("💰 查看 "+telegram.DisplayDC(validDCs[0])+" 价格", "i:D:pr:"+planRef+":"+validDCs[0]),
					})
				}
				rows = append(rows, []map[string]string{
					telegram.CallbackButton("🔙 重新选择机型", "i:cat:pr:root"),
				})
				markup = telegram.InlineKeyboard(rows)
			} else {
				markup = telegram.InlineKeyboard([][]map[string]string{
					{
						telegram.CallbackButton("⚡ "+telegram.DisplayDC(dc)+" 立即开抢", "i:D:b:"+planRef+":"+dc),
						telegram.CallbackButton("👀 监控此型号", "i:M:"+planRef),
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
			accText = fmt.Sprintf("%s · %s (%s 区 · 共 %d 个账户)", acc.Name, zone, ovh.SubsidiaryRegion(zone), len(listAccounts(state)))
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
