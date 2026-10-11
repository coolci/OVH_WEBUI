package handlers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

func handleInlineCallback(state *app.State, mon *monitor.Monitor, cbID string, chatID interface{}, messageID int64, data string) bool {
	if !strings.HasPrefix(data, "i:") {
		return false
	}
	parts := strings.Split(data, ":")
	if len(parts) < 3 {
		telegram.AnswerCallback(state, cbID, "按钮已失效", true)
		return true
	}
	kind := parts[1]
	switch kind {
	case "cat":
		if len(parts) < 4 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		mode, cat := parts[2], parts[3]
		telegram.AnswerCallback(state, cbID, "加载中…", false)
		if cat == "root" {
			renderCategoryPicker(state, mon, chatID, messageID, mode, true)
		} else {
			renderPlanPage(state, mon, chatID, messageID, mode, cat, 0, true)
		}
	case "pg":
		if len(parts) < 5 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		mode, cat := parts[2], parts[3]
		page, _ := strconv.Atoi(parts[4])
		telegram.AnswerCallback(state, cbID, fmt.Sprintf("第 %d 页", page+1), false)
		renderPlanPage(state, mon, chatID, messageID, mode, cat, page, true)
	case "P":
		if len(parts) < 4 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		mode, plan := parts[2], parts[3]
		if mode == "s" {
			telegram.AnswerCallback(state, cbID, "已查库存: "+plan, false)
			showStockCardWithButtons(state, mon, chatID, messageID, plan)
			return true
		}
		if mode == "pr" {
			telegram.AnswerCallback(state, cbID, "已选 "+plan, false)
			showDCPicker(state, chatID, messageID, mode, plan, true)
			return true
		}
		configs := enumerateWizardConfigs(state, plan, "")
		if len(configs) > 1 {
			telegram.AnswerCallback(state, cbID, "请选择配置", false)
			showConfigPicker(state, mon, chatID, messageID, mode, plan, true, nil, 1)
			return true
		}
		sess := getPickerSession(chatID, messageID, mode, plan)
		if len(configs) == 1 {
			sess.Options = configs[0].Options
			sess.ConfigLabel = configs[0].Label
		} else {
			sess.Options = nil
			sess.ConfigLabel = ""
		}
		telegram.AnswerCallback(state, cbID, "已选 "+plan, false)
		showDCPicker(state, chatID, messageID, mode, plan, true)
		return true
	case "cfg":
		if len(parts) < 4 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		tok, cfgIdxStr := parts[2], parts[3]
		// 兼容旧按钮 i:cfg:<mode>:<plan>:<idx>
		if len(parts) >= 5 && (tok == "b" || tok == "q" || tok == "m") {
			mode, plan := parts[2], parts[3]
			cfgIdxStr = parts[4]
			sess := getPickerSession(chatID, messageID, mode, plan)
			if cfgIdxStr == "any" {
				sess.Options = nil
				sess.ConfigLabel = "任意/默认配置"
				telegram.AnswerCallback(state, cbID, "已选任意配置", false)
			} else {
				idx, err := strconv.Atoi(cfgIdxStr)
				configs := enumerateWizardConfigs(state, plan, "")
				if err == nil && idx >= 0 && idx < len(configs) {
					sess.Options = append([]string{}, configs[idx].Options...)
					sess.ConfigLabel = configs[idx].Label
					telegram.AnswerCallback(state, cbID, "已选: "+configs[idx].Label, false)
				} else {
					telegram.AnswerCallback(state, cbID, "配置列表已变化，请重新选择", true)
					showConfigPicker(state, mon, chatID, messageID, mode, plan, true, nil, 1)
					return true
				}
			}
			showDCPicker(state, chatID, messageID, mode, plan, true)
			return true
		}
		offer, ok := getCfgOffer(tok)
		if !ok {
			telegram.AnswerCallback(state, cbID, "选择已过期，请重新发 /buy", true)
			return true
		}
		var opts []string
		label := "任意/默认配置"
		if cfgIdxStr != "any" {
			idx, err := strconv.Atoi(cfgIdxStr)
			if err != nil || idx < 0 || idx >= len(offer.Configs) || idx >= 8 {
				telegram.AnswerCallback(state, cbID, "选项无效", true)
				return true
			}
			opts = append([]string{}, offer.Configs[idx].Options...)
			label = offer.Configs[idx].Label
			telegram.AnswerCallback(state, cbID, "已选: "+label, false)
		} else {
			telegram.AnswerCallback(state, cbID, "已选任意配置", false)
		}
		if len(offer.TargetDCs) > 0 && offer.Mode != "m" {
			qty := offer.Quantity
			if qty < 1 {
				qty = 1
			}
			accID := telegram.ActiveAccountID(state)
			if len(offer.TargetDCs) == 1 {
				res := telegram.ProcessOrder(state, accID, offer.PlanCode, offer.TargetDCs[0], qty, opts)
				text := res.Message
				if res.Success {
					text = fmt.Sprintf("✅ 已加入抢购队列\n\n📦 型号: %s\n📍 机房: %s\n⚙️ 配置: %s\n⏱ 重试间隔: %d 秒\n\n已创建 %d 个任务，按「设置 → 抢购参数」的间隔重试。",
						offer.PlanCode, telegram.DisplayDCFull(offer.TargetDCs[0]), label, state.Config.RetryInterval(), res.CreatedOrders)
				} else {
					text = "❌ " + res.Message
				}
				_ = telegram.EditMessage(state, chatID, messageID, text, telegram.EmptyInlineKeyboard())
				return true
			}
			enqueueWizardDCs(state, chatID, messageID, offer.Mode, offer.PlanCode, offer.TargetDCs, accID, opts, label, false)
			return true
		}
		sess := getPickerSession(chatID, messageID, offer.Mode, offer.PlanCode)
		sess.Options = opts
		sess.ConfigLabel = label
		showDCPicker(state, chatID, messageID, offer.Mode, offer.PlanCode, true)
		return true
	case "cfgshow":
		if len(parts) < 4 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		mode, plan := parts[2], parts[3]
		telegram.AnswerCallback(state, cbID, "请选择配置", false)
		showConfigPicker(state, mon, chatID, messageID, mode, plan, true, nil, 1)
		return true
	case "D":
		if len(parts) < 3 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		act := parts[2]
		// 多选操作: i:D:t (toggle), i:D:as (all stock), i:D:aa (all), i:D:cl (clear), i:D:ok (submit)
		switch act {
		case "t":
			if len(parts) < 6 {
				telegram.AnswerCallback(state, cbID, "按钮无效", true)
				return true
			}
			mode, plan, dc := parts[3], parts[4], parts[5]
			sess := getPickerSession(chatID, messageID, mode, plan)
			sess.Selected[dc] = !sess.Selected[dc]
			dcLabel := telegram.DisplayDC(dc)
			if sess.Selected[dc] {
				telegram.AnswerCallback(state, cbID, "已勾选 "+dcLabel, false)
			} else {
				telegram.AnswerCallback(state, cbID, "已取消 "+dcLabel, false)
			}
			showDCPicker(state, chatID, messageID, mode, plan, true)
			return true
		case "as":
			if len(parts) < 5 {
				telegram.AnswerCallback(state, cbID, "按钮无效", true)
				return true
			}
			mode, plan := parts[3], parts[4]
			sess := getPickerSession(chatID, messageID, mode, plan)
			_, stock := validAndInStockDCs(state, plan, "")
			if len(stock) == 0 {
				telegram.AnswerCallback(state, cbID, "当前全区缺货，请手动勾选机房挂机", true)
				return true
			}
			for _, d := range stock {
				sess.Selected[d] = true
			}
			telegram.AnswerCallback(state, cbID, fmt.Sprintf("已勾选 %d 个有货机房", len(stock)), false)
			showDCPicker(state, chatID, messageID, mode, plan, true)
			return true
		case "aa":
			if len(parts) < 5 {
				telegram.AnswerCallback(state, cbID, "按钮无效", true)
				return true
			}
			mode, plan := parts[3], parts[4]
			sess := getPickerSession(chatID, messageID, mode, plan)
			validDCs, _ := validAndInStockDCs(state, plan, "")
			if len(validDCs) == 0 {
				validDCs = telegram.StandardDCs
			}
			for _, d := range validDCs {
				sess.Selected[d] = true
			}
			telegram.AnswerCallback(state, cbID, fmt.Sprintf("已全选 %d 个机房", len(validDCs)), false)
			showDCPicker(state, chatID, messageID, mode, plan, true)
			return true
		case "cl":
			if len(parts) < 5 {
				telegram.AnswerCallback(state, cbID, "按钮无效", true)
				return true
			}
			mode, plan := parts[3], parts[4]
			sess := getPickerSession(chatID, messageID, mode, plan)
			sess.Selected = map[string]bool{}
			telegram.AnswerCallback(state, cbID, "已清空机房选择", false)
			showDCPicker(state, chatID, messageID, mode, plan, true)
			return true
		case "ok":
			if len(parts) < 5 {
				telegram.AnswerCallback(state, cbID, "按钮无效", true)
				return true
			}
			mode, plan := parts[3], parts[4]
			sess := getPickerSession(chatID, messageID, mode, plan)
			var selectedDCs []string
			for dc, sel := range sess.Selected {
				if sel {
					selectedDCs = append(selectedDCs, dc)
				}
			}
			if len(selectedDCs) == 0 {
				telegram.AnswerCallback(state, cbID, "请至少勾选一个机房！", true)
				return true
			}
			sort.Strings(selectedDCs)
			pickedOpts := append([]string{}, sess.Options...)
			pickedCfgLabel := sess.ConfigLabel
			clearPickerSession(chatID, messageID)

			if mode == "m" {
				if mon != nil {
					var serverName string
					state.ServerPlansMu.RLock()
					for _, s := range state.ServerPlans {
						if s.PlanCode == plan {
							serverName = s.Name
							break
						}
					}
					state.ServerPlansMu.RUnlock()
					mon.AddSubscription(plan, selectedDCs, true, false, serverName, nil, nil, false, 0, "", false, pickedOpts)
					mon.SaveToDB()
					if !mon.Running() {
						mon.Start()
					}
				}
				telegram.AnswerCallback(state, cbID, fmt.Sprintf("已添加 %d 个机房监控", len(selectedDCs)), false)
				var dcLabels []string
				for _, d := range selectedDCs {
					dcLabels = append(dcLabels, telegram.DisplayDCFull(d))
				}
				cfgLine := ""
				if pickedCfgLabel != "" {
					cfgLine = "\n⚙️ 监控配置: " + pickedCfgLabel
				}
				text := fmt.Sprintf("✅ 已成功添加库存监控！\n\n📦 型号: %s%s\n📍 监控机房 (%d个):\n  • %s\n\n一旦官方有货上架，Bot 将第一时间向您推送补货通知！",
					plan, cfgLine, len(selectedDCs), strings.Join(dcLabels, "\n  • "))
				markup := telegram.InlineKeyboard([][]map[string]string{
					{
						telegram.CallbackButton("📋 查看监控列表", "i:mon:list"),
						telegram.CallbackButton("➕ 继续添加监控", "i:cat:m:root"),
					},
				})
				_ = telegram.EditMessage(state, chatID, messageID, text, markup)
				if len(pickedOpts) == 0 {
					offerNarrowConfig(state, chatID, messageID, plan)
				}
				return true
			}
			telegram.AnswerCallback(state, cbID, fmt.Sprintf("已选 %d 个机房，正在提交抢购…", len(selectedDCs)), false)
			enqueueWizardDCs(state, chatID, messageID, mode, plan, selectedDCs, "", pickedOpts, pickedCfgLabel, false)
			return true
		case "pr":
			if len(parts) < 5 {
				telegram.AnswerCallback(state, cbID, "按钮无效", true)
				return true
			}
			plan, dc := parts[3], parts[4]
			telegram.AnswerCallback(state, cbID, "询价完成", false)
			rawPrice := cmdPrice(state, []string{plan, dc})
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
					telegram.CallbackButton("🔙 重新选择机房", "i:P:pr:"+plan),
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
			_ = telegram.EditMessage(state, chatID, messageID, rawPrice, markup)
			return true
		default:
			if len(parts) >= 5 {
				mode, plan, dc := parts[2], parts[3], parts[4]
				sess := getPickerSession(chatID, messageID, mode, plan)
				pickedOpts := append([]string{}, sess.Options...)
				pickedCfgLabel := sess.ConfigLabel
				if len(pickedOpts) == 0 {
					avail := catalog.CheckServerAvailabilityWithConfigs(state, plan, "")
					for _, cfg := range avail {
						if cfg != nil && catalog.IsAvailableForOrder(cfg.Datacenters[dc]) && len(cfg.Options) > 0 {
							pickedOpts = append([]string{}, cfg.Options...)
							pickedCfgLabel = strings.TrimSpace(cfg.Memory + " / " + cfg.Storage)
							break
						}
					}
				}
				if mode == "m" {
					var dcs []string
					dcLabel := "全部机房"
					if dc != "all" {
						dcs = []string{dc}
						dcLabel = telegram.DisplayDCFull(dc)
					}
					if mon != nil {
						var serverName string
						state.ServerPlansMu.RLock()
						for _, s := range state.ServerPlans {
							if s.PlanCode == plan {
								serverName = s.Name
								break
							}
						}
						state.ServerPlansMu.RUnlock()
						mon.AddSubscription(plan, dcs, true, false, serverName, nil, nil, false, 0, "", false, nil)
						mon.SaveToDB()
						if !mon.Running() {
							mon.Start()
						}
					}
					telegram.AnswerCallback(state, cbID, "已添加监控", false)
					text := fmt.Sprintf("✅ 已成功添加库存监控！\n\n📦 型号: %s\n📍 监控机房: %s\n\n一旦官方有货上架，Bot 将第一时间向您推送补货通知！", plan, dcLabel)
					markup := telegram.InlineKeyboard([][]map[string]string{
						{
							telegram.CallbackButton("📋 查看监控列表", "i:mon:list"),
							telegram.CallbackButton("➕ 继续添加监控", "i:cat:m:root"),
						},
					})
					_ = telegram.EditMessage(state, chatID, messageID, text, markup)
					offerNarrowConfig(state, chatID, messageID, plan)
					return true
				}
				telegram.AnswerCallback(state, cbID, telegram.DisplayDC(dc), false)
				enqueueWizardDCs(state, chatID, messageID, mode, plan, []string{dc}, "", pickedOpts, pickedCfgLabel, false)
				return true
			}
		}
	case "A":
		if len(parts) < 4 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		mode, plan := parts[2], parts[3]
		dcs := inStockDisplayDCs(state, plan, "")
		if len(dcs) == 0 {
			telegram.AnswerCallback(state, cbID, "当前无有货机房，请点单个机房排队", true)
			return true
		}
		telegram.AnswerCallback(state, cbID, "全选有货", false)
		enqueueWizardDCs(state, chatID, messageID, mode, plan, dcs, "", nil, "", false)
	case "Z":
		if len(parts) < 4 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		mode, plan := parts[2], parts[3]
		telegram.AnswerCallback(state, cbID, "全部机房", false)
		validDCs, _ := validAndInStockDCs(state, plan, "")
		targetDCs := validDCs
		if len(targetDCs) == 0 {
			targetDCs = append([]string{}, telegram.StandardDCs...)
		}
		enqueueWizardDCs(state, chatID, messageID, mode, plan, targetDCs, "", nil, "", false)
	case "C":
		if len(parts) < 4 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		btnID := resolveShort(state, parts[2])
		accID := resolveShort(state, parts[3])
		if btnID == "" || accID == "" {
			telegram.AnswerCallback(state, cbID, "会话已过期，请等新的上架通知", true)
			return true
		}
		telegram.AnswerCallback(state, cbID, "已选账户", false)
		enqueueFromNotifyButton(state, mon, cbID, chatID, messageID, btnID, "queue_all", accID, false)
	case "M":
		if len(parts) < 3 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		plan := parts[2]
		if mon != nil {
			var serverName string
			state.ServerPlansMu.RLock()
			for _, s := range state.ServerPlans {
				if s.PlanCode == plan {
					serverName = s.Name
					break
				}
			}
			state.ServerPlansMu.RUnlock()
			mon.AddSubscription(plan, nil, true, false, serverName, nil, nil, false, 0, "", false, nil)
			mon.SaveToDB()
			if !mon.Running() {
				mon.Start()
			}
		}
		telegram.AnswerCallback(state, cbID, "已添加 "+plan+" 全机房监控", false)
		_ = telegram.EditMessage(state, chatID, messageID, fmt.Sprintf("✅ 已成功添加 %s 全机房库存监控，有货时将通过 Telegram 自动推送！", plan), telegram.EmptyInlineKeyboard())
		offerNarrowConfig(state, chatID, messageID, plan)
	case "T":
		if len(parts) < 3 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		subAct := parts[2]

		if subAct == "one" && len(parts) >= 4 {
			short := parts[3]
			taskID := resolveShort(state, short)
			if taskID == "" {
				taskID = short
			}
			state.QueueMu.Lock()
			var cancelledItem *types.QueueItem
			var remaining []types.QueueItem
			newQueue := make([]types.QueueItem, 0, len(state.Queue))
			for _, it := range state.Queue {
				if it.ID == taskID || strings.HasPrefix(it.ID, taskID) {
					cp := it
					cancelledItem = &cp
					continue
				}
				newQueue = append(newQueue, it)
			}
			state.Queue = newQueue
			if cancelledItem != nil {
				for _, it := range state.Queue {
					if it.TelegramMessageID == cancelledItem.TelegramMessageID && (it.Status == "running" || it.Status == "pending" || it.Status == "paused") {
						remaining = append(remaining, it)
					}
				}
			}
			state.QueueMu.Unlock()

			if cancelledItem == nil {
				telegram.AnswerCallback(state, cbID, "该任务已不存在或已结束", true)
				return true
			}
			state.MarkTaskDeleted(cancelledItem.ID)
			_ = state.SaveQueue()
			dcName := telegram.DisplayDC(cancelledItem.Datacenter)

			if len(remaining) > 0 {
				telegram.AnswerCallback(state, cbID, "已取消 "+dcName+" 抢购任务", false)
				var b strings.Builder
				kind := "抢购挂机排队中"
				if cancelledItem.QuickOrder {
					kind = "极速下单排队中"
				}
				b.WriteString(fmt.Sprintf("⏳ %s…\n\n", kind))
				b.WriteString("📦 型号: " + cancelledItem.PlanCode + "\n")
				b.WriteString("📍 正在排队:\n")
				for _, r := range remaining {
					b.WriteString("  • " + telegram.DisplayDCFull(r.Datacenter) + "\n")
				}
				b.WriteString(fmt.Sprintf("\n🛑 已取消机房: %s (用户主动取消)\n", telegram.DisplayDCFull(cancelledItem.Datacenter)))
				b.WriteString(fmt.Sprintf("📊 任务状态: %d 个运行中\n", len(remaining)))
				b.WriteString("\n💡 官方放货后将按设置的间隔自动提交，进度会实时更新本条消息。")

				var cancelBtns []map[string]string
				for _, r := range remaining {
					s := rememberShort(state, r.ID, "task")
					cancelBtns = append(cancelBtns, telegram.CallbackButton("⏹ 取消 "+strings.ToUpper(r.Datacenter), "i:T:one:"+s))
				}
				var btnRows [][]map[string]string
				if len(cancelBtns) > 0 {
					btnRows = append(btnRows, telegram.ChunkButtons(cancelBtns, 3)...)
				}
				if len(remaining) > 1 {
					btnRows = append(btnRows, []map[string]string{
						telegram.CallbackButton(fmt.Sprintf("🛑 取消剩余全部 (%d个)", len(remaining)), fmt.Sprintf("i:T:m:%d", cancelledItem.TelegramMessageID)),
					})
				}
				_ = telegram.EditMessage(state, chatID, messageID, b.String(), telegram.InlineKeyboard(btnRows))
			} else {
				telegram.AnswerCallback(state, cbID, "本次抢购任务已全部取消", false)
				text := fmt.Sprintf("🛑 抢购任务已全部取消\n\n📦 型号: %s\n📍 机房: %s\nℹ️ 说明: 用户已在 Telegram 中取消了本次创建的全部任务",
					cancelledItem.PlanCode, telegram.DisplayDCFull(cancelledItem.Datacenter))
				_ = telegram.EditMessage(state, chatID, messageID, text, telegram.EmptyInlineKeyboard())
			}
			return true
		}

		if subAct == "m" && len(parts) >= 4 {
			targetMsgID, _ := strconv.ParseInt(parts[3], 10, 64)
			if targetMsgID == 0 {
				targetMsgID = messageID
			}
			state.QueueMu.Lock()
			count := 0
			var planCode string
			newQueue := make([]types.QueueItem, 0, len(state.Queue))
			var deletedIDs []string
			for _, it := range state.Queue {
				if it.TelegramMessageID == targetMsgID {
					count++
					planCode = it.PlanCode
					deletedIDs = append(deletedIDs, it.ID)
					continue
				}
				newQueue = append(newQueue, it)
			}
			state.Queue = newQueue
			state.QueueMu.Unlock()
			for _, id := range deletedIDs {
				state.MarkTaskDeleted(id)
			}
			if count > 0 {
				_ = state.SaveQueue()
				telegram.AnswerCallback(state, cbID, fmt.Sprintf("已取消本次全部 %d 个任务", count), false)
				text := fmt.Sprintf("🛑 抢购任务已全部取消\n\n📦 型号: %s\n📊 状态: 本次创建的 %d 个抢购任务已全部终止", planCode, count)
				_ = telegram.EditMessage(state, chatID, messageID, text, telegram.EmptyInlineKeyboard())
			} else {
				telegram.AnswerCallback(state, cbID, "任务已不存在或已结束", true)
				_ = telegram.EditMessage(state, chatID, messageID, "🛑 抢购任务已不存在或已结束。", telegram.EmptyInlineKeyboard())
			}
			return true
		}

		if subAct == "all" {
			state.QueueMu.Lock()
			count := 0
			newQueue := make([]types.QueueItem, 0, len(state.Queue))
			var deletedIDs []string
			chatStr := telegram.ChatIDString(chatID)
			for _, it := range state.Queue {
				if it.TelegramMessageID == messageID || (chatStr != "" && it.TelegramChatID == chatStr) {
					count++
					deletedIDs = append(deletedIDs, it.ID)
					continue
				}
				newQueue = append(newQueue, it)
			}
			state.Queue = newQueue
			state.QueueMu.Unlock()
			for _, id := range deletedIDs {
				state.MarkTaskDeleted(id)
			}
			if count > 0 {
				_ = state.SaveQueue()
			}
			telegram.AnswerCallback(state, cbID, fmt.Sprintf("已终止 %d 个抢购任务", count), false)
			_ = telegram.EditMessage(state, chatID, messageID, fmt.Sprintf("🛑 已终止并取消当前会话的 %d 个抢购任务。", count), telegram.EmptyInlineKeyboard())
			return true
		}

		taskID := resolveShort(state, subAct)
		if taskID == "" {
			taskID = subAct
		}
		state.QueueMu.Lock()
		var foundItem *types.QueueItem
		newQueue := make([]types.QueueItem, 0, len(state.Queue))
		for _, it := range state.Queue {
			if it.ID == taskID || strings.HasPrefix(it.ID, taskID) {
				cp := it
				foundItem = &cp
				continue
			}
			newQueue = append(newQueue, it)
		}
		state.Queue = newQueue
		state.QueueMu.Unlock()
		if foundItem != nil {
			state.MarkTaskDeleted(foundItem.ID)
			_ = state.SaveQueue()
			telegram.AnswerCallback(state, cbID, "抢购任务已取消", false)
			text := fmt.Sprintf("🛑 抢购任务已取消\n\n📦 型号: %s\n📍 机房: %s\nℹ️ 说明: 用户已在 Telegram 中取消了此任务",
				foundItem.PlanCode, telegram.DisplayDCFull(foundItem.Datacenter))
			_ = telegram.EditMessage(state, chatID, messageID, text, telegram.EmptyInlineKeyboard())
		} else {
			telegram.AnswerCallback(state, cbID, "任务已不存在或已结束", true)
			_ = telegram.EditMessage(state, chatID, messageID, "🛑 抢购任务已不存在或已结束。", telegram.EmptyInlineKeyboard())
		}
		return true
	case "Tk":
		if len(parts) < 3 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		target := parts[2]
		if target == "list" {
			telegram.AnswerCallback(state, cbID, "任务列表", false)
			showTasks(state, chatID, messageID, true)
			return true
		}
		if target == "all" {
			state.QueueMu.Lock()
			ids := make([]string, 0, len(state.Queue))
			for _, it := range state.Queue {
				ids = append(ids, it.ID)
			}
			state.Queue = nil
			state.QueueMu.Unlock()
			for _, id := range ids {
				state.MarkTaskDeleted(id)
			}
			_ = state.SaveQueue()
			telegram.AnswerCallback(state, cbID, "已清空队列", false)
			showTasks(state, chatID, messageID, true)
			return true
		}
		taskID := resolveShort(state, target)
		if taskID == "" {
			taskID = target
		}
		state.QueueMu.Lock()
		found := false
		var deletedID string
		newQueue := make([]types.QueueItem, 0, len(state.Queue))
		for _, it := range state.Queue {
			if it.ID == taskID || strings.HasPrefix(it.ID, taskID) {
				found = true
				deletedID = it.ID
				continue
			}
			newQueue = append(newQueue, it)
		}
		state.Queue = newQueue
		state.QueueMu.Unlock()
		if found {
			if deletedID != "" {
				state.MarkTaskDeleted(deletedID)
			}
			_ = state.SaveQueue()
			telegram.AnswerCallback(state, cbID, "任务已停止", false)
			showTasks(state, chatID, messageID, true)
		} else {
			telegram.AnswerCallback(state, cbID, "任务已不存在", true)
			showTasks(state, chatID, messageID, true)
		}
	case "mon":
		if len(parts) < 3 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		subAct := parts[2]
		if subAct == "n" && len(parts) >= 5 {
			tok, idxStr := parts[3], parts[4]
			offer, ok := takeNarrowOffer(tok)
			if !ok {
				telegram.AnswerCallback(state, cbID, "这个选择已过期，请重新 /monitor", true)
				return true
			}
			if mon == nil {
				telegram.AnswerCallback(state, cbID, "监控模块不可用", true)
				return true
			}
			var opts []string
			label := "全部配置"
			if idxStr != "all" {
				idx, err := strconv.Atoi(idxStr)
				if err != nil || idx < 0 || idx >= len(offer.Configs) || idx >= narrowMaxButtons {
					telegram.AnswerCallback(state, cbID, "选项无效", true)
					return true
				}
				opts = offer.Configs[idx].Options
				label = offer.Configs[idx].Label
			}
			if !mon.SetSubscriptionOptions(offer.PlanCode, opts) {
				telegram.AnswerCallback(state, cbID, "订阅已不在了", true)
				telegram.SendReply(state, chatID, "这条订阅已经被删掉了，发 /monitor "+offer.PlanCode+" 重新开始。", messageID)
				return true
			}
			mon.SaveToDB()
			telegram.AnswerCallback(state, cbID, "好", false)
			if len(opts) == 0 {
				telegram.SendReply(state, chatID,
					"🌐 "+offer.PlanCode+" 改回盯全部配置。\n每套配置补货都会各自通知、各自下单。", messageID)
			} else {
				telegram.SendReply(state, chatID,
					"🎯 "+offer.PlanCode+" 现在只盯："+label+"\n其它配置补货不再通知，也不会下单。", messageID)
			}
			return true
		}
		if subAct == "list" {
			telegram.AnswerCallback(state, cbID, "监控列表", false)
			showMonitorManager(state, mon, chatID, messageID, true)
			return true
		}
		if subAct == "del" && len(parts) >= 4 {
			planToDel := parts[3]
			if mon != nil {
				mon.RemoveSubscription(planToDel)
				mon.SaveToDB()
			}
			telegram.AnswerCallback(state, cbID, "已移除 "+planToDel, false)
			showMonitorManager(state, mon, chatID, messageID, true)
			return true
		}
		if subAct == "clear" {
			if mon != nil {
				for _, s := range mon.Snapshot() {
					if s != nil && s.PlanCode != "" {
						mon.RemoveSubscription(s.PlanCode)
					}
				}
				mon.SaveToDB()
			}
			telegram.AnswerCallback(state, cbID, "已清空全部监控", false)
			showMonitorManager(state, mon, chatID, messageID, true)
			return true
		}
	case "dash":
		act := "refresh"
		if len(parts) >= 3 {
			act = parts[2]
		}
		if act == "iv" {
			telegram.AnswerCallback(state, cbID, "重试间隔", false)
			text := intervalText(state, nil)
			markup := telegram.InlineKeyboard([][]map[string]string{
				{telegram.CallbackButton("🔙 返回主菜单", "i:dash:refresh")},
			})
			_ = telegram.EditMessage(state, chatID, messageID, text, markup)
			return true
		}
		telegram.AnswerCallback(state, cbID, "控制台已就绪", false)
		_ = telegram.EditMessage(state, chatID, messageID, telegram.HelpMessage(), dashboardKeyboard())
		return true
	case "acc":
		telegram.AnswerCallback(state, cbID, "账户管理", false)
		showAccounts(state, chatID, messageID, true)
		return true
	case "S":
		if len(parts) < 3 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		accID := resolveShort(state, parts[2])
		if accID == "" {
			accID = parts[2]
		}
		_ = telegram.SetActiveAccount(state, accID)
		acc, _ := state.FindAccount(accID)
		accName := acc.Name
		if accName == "" {
			accName = accID
		}
		telegram.AnswerCallback(state, cbID, "已切换当前活跃账户: "+accName, false)
		showAccounts(state, chatID, messageID, true)
	default:
		telegram.AnswerCallback(state, cbID, "未知按钮", true)
	}
	return true
}
