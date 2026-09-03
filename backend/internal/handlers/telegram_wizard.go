package handlers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

var (
	shortMu   sync.Mutex
	shortToID = map[string]string{}
)

func rememberShort(full string) string {
	s := strings.ReplaceAll(full, "-", "")
	if len(s) > 8 {
		s = s[:8]
	}
	shortMu.Lock()
	shortToID[s] = full
	shortMu.Unlock()
	return s
}

func resolveShort(s string) string {
	shortMu.Lock()
	defer shortMu.Unlock()
	if v, ok := shortToID[s]; ok {
		return v
	}
	return ""
}

type wizardPlan struct {
	Code string
	Name string
}

func getPlansForCategory(state *app.State, mon *monitor.Monitor, category string) (string, []wizardPlan) {
	state.ServerPlansMu.RLock()
	allPlans := append([]types.ServerPlan{}, state.ServerPlans...)
	state.ServerPlansMu.RUnlock()

	switch category {
	case "mon":
		seen := map[string]string{}
		if mon != nil {
			for _, s := range mon.Snapshot() {
				if s != nil && s.PlanCode != "" {
					seen[s.PlanCode] = s.ServerName
				}
			}
		}
		state.QueueMu.Lock()
		for _, q := range state.Queue {
			if q.PlanCode != "" {
				if _, ok := seen[q.PlanCode]; !ok {
					seen[q.PlanCode] = ""
				}
			}
		}
		state.QueueMu.Unlock()
		out := make([]wizardPlan, 0, len(seen))
		for code, name := range seen {
			out = append(out, wizardPlan{Code: code, Name: name})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
		return "📋 我的监控/队列", out

	case "ks":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			if strings.HasPrefix(p, "24sk") || strings.HasPrefix(p, "ks") || strings.Contains(p, "kimsufi") || strings.Contains(n, "kimsufi") || strings.Contains(n, "ks-") {
				out = append(out, wizardPlan{Code: s.PlanCode, Name: s.Name})
			}
		}
		return "💎 Kimsufi / KS 特惠系列", out

	case "rise":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			if strings.HasPrefix(p, "24rise") || strings.HasPrefix(p, "rise") || strings.Contains(n, "rise") {
				out = append(out, wizardPlan{Code: s.PlanCode, Name: s.Name})
			}
		}
		return "🚀 Rise 高性价比系列", out

	case "adv":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			if strings.HasPrefix(p, "24adv") || strings.HasPrefix(p, "adv") || strings.Contains(n, "advance") {
				out = append(out, wizardPlan{Code: s.PlanCode, Name: s.Name})
			}
		}
		return "🏢 Advance 企业级系列", out

	case "sys":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			if strings.HasPrefix(p, "sys") || strings.HasPrefix(p, "24sys") || strings.HasPrefix(p, "stor") || strings.Contains(n, "so you start") || strings.Contains(n, "sys-") {
				out = append(out, wizardPlan{Code: s.PlanCode, Name: s.Name})
			}
		}
		return "💾 SYS / 存储型系列", out

	default: // "all"
		out := make([]wizardPlan, 0, len(allPlans))
		for _, s := range allPlans {
			out = append(out, wizardPlan{Code: s.PlanCode, Name: s.Name})
		}
		if len(out) == 0 {
			for _, p := range []string{"24ska01", "24rise01", "24game01", "ks-le-1", "sys-le-1"} {
				out = append(out, wizardPlan{Code: p, Name: p})
			}
		}
		return "🌐 全部服务器型号", out
	}
}

func renderCategoryPicker(state *app.State, mon *monitor.Monitor, chatID interface{}, messageID int64, mode string, edit bool) {
	_, ksPlans := getPlansForCategory(state, mon, "ks")
	_, risePlans := getPlansForCategory(state, mon, "rise")
	_, advPlans := getPlansForCategory(state, mon, "adv")
	_, sysPlans := getPlansForCategory(state, mon, "sys")
	_, monPlans := getPlansForCategory(state, mon, "mon")
	_, allPlans := getPlansForCategory(state, mon, "all")

	actionText := "抢购"
	icon := "🛒"
	switch mode {
	case "q":
		actionText = "加入队列"
		icon = "📥"
	case "s":
		actionText = "查询库存"
		icon = "📦"
	case "m":
		actionText = "添加监控"
		icon = "👀"
	case "pr":
		actionText = "查询价格"
		icon = "💰"
	}

	text := fmt.Sprintf("%s 请选择要%s的服务器系列分类：\n（当前库中共 %d 款机型，也可直接发送命令如: /buy 24rise01-v2 gra）", icon, actionText, len(allPlans))

	btns := [][]map[string]string{
		{
			telegram.CallbackButton(fmt.Sprintf("💎 Kimsufi/KS (%d款)", len(ksPlans)), "i:cat:"+mode+":ks"),
			telegram.CallbackButton(fmt.Sprintf("🚀 Rise系列 (%d款)", len(risePlans)), "i:cat:"+mode+":rise"),
		},
		{
			telegram.CallbackButton(fmt.Sprintf("🏢 Advance系列 (%d款)", len(advPlans)), "i:cat:"+mode+":adv"),
			telegram.CallbackButton(fmt.Sprintf("💾 SYS/存储型 (%d款)", len(sysPlans)), "i:cat:"+mode+":sys"),
		},
	}

	thirdRow := []map[string]string{}
	if len(monPlans) > 0 {
		thirdRow = append(thirdRow, telegram.CallbackButton(fmt.Sprintf("📋 监控/队列 (%d款)", len(monPlans)), "i:cat:"+mode+":mon"))
	}
	thirdRow = append(thirdRow, telegram.CallbackButton(fmt.Sprintf("🌐 全部型号 (%d款)", len(allPlans)), "i:cat:"+mode+":all"))
	btns = append(btns, thirdRow)

	markup := telegram.InlineKeyboard(btns)
	if edit && messageID > 0 {
		if telegram.EditMessage(state, chatID, messageID, text, markup) {
			return
		}
	}
	_, _ = telegram.SendToChat(state, chatID, text, markup)
}

func renderPlanPage(state *app.State, mon *monitor.Monitor, chatID interface{}, messageID int64, mode, category string, page int, edit bool) {
	catTitle, plans := getPlansForCategory(state, mon, category)
	if len(plans) == 0 {
		text := fmt.Sprintf("%s 暂无机型。\n\n请返回选择其他系列分类。", catTitle)
		markup := telegram.InlineKeyboard([][]map[string]string{
			{telegram.CallbackButton("🔙 返回系列分类", "i:cat:"+mode+":root")},
		})
		if edit && messageID > 0 {
			_ = telegram.EditMessage(state, chatID, messageID, text, markup)
			return
		}
		_, _ = telegram.SendToChat(state, chatID, text, markup)
		return
	}

	pageSize := 10
	totalPages := (len(plans) + pageSize - 1) / pageSize
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * pageSize
	end := start + pageSize
	if end > len(plans) {
		end = len(plans)
	}
	pagePlans := plans[start:end]

	prefix := "i:P:" + mode + ":"
	modelBtns := make([]map[string]string, 0, len(pagePlans))
	for _, p := range pagePlans {
		data := prefix + p.Code
		if len(data) > 64 {
			continue
		}
		label := p.Code
		modelBtns = append(modelBtns, telegram.CallbackButton(label, data))
	}
	rows := telegram.ChunkButtons(modelBtns, 2)

	// 分页导航按钮
	navRow := []map[string]string{}
	if page > 0 {
		navRow = append(navRow, telegram.CallbackButton("⬅️ 上一页", fmt.Sprintf("i:pg:%s:%s:%d", mode, category, page-1)))
	}
	if totalPages > 1 {
		navRow = append(navRow, telegram.CallbackButton(fmt.Sprintf("%d/%d 页", page+1, totalPages), fmt.Sprintf("i:pg:%s:%s:%d", mode, category, page)))
	}
	if page < totalPages-1 {
		navRow = append(navRow, telegram.CallbackButton("下一页 ➡️", fmt.Sprintf("i:pg:%s:%s:%d", mode, category, page+1)))
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	// 底部返回按钮
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔙 返回系列分类", "i:cat:"+mode+":root"),
	})

	actionText := "抢购"
	icon := "🛒"
	switch mode {
	case "q":
		actionText = "加入队列"
		icon = "📥"
	case "s":
		actionText = "查询库存"
		icon = "📦"
	case "m":
		actionText = "添加监控"
		icon = "👀"
	case "pr":
		actionText = "查询价格"
		icon = "💰"
	}
	text := fmt.Sprintf("%s %s（第 %d/%d 页，共 %d 款）：\n请点击选择要%s的型号：", icon, catTitle, page+1, totalPages, len(plans), actionText)

	markup := telegram.InlineKeyboard(rows)
	if edit && messageID > 0 {
		if telegram.EditMessage(state, chatID, messageID, text, markup) {
			return
		}
	}
	_, _ = telegram.SendToChat(state, chatID, text, markup)
}

func validAndInStockDCs(state *app.State, planCode, accountID string) (validDCs []string, inStockDCs []string) {
	if accountID == "" {
		accountID = telegram.DefaultAccountID(state)
	}
	avail := catalog.CheckServerAvailabilityWithConfigs(state, planCode, accountID)
	validMap := map[string]bool{}
	inStockMap := map[string]bool{}
	for _, cfg := range avail {
		for dc, st := range cfg.Datacenters {
			norm := telegram.NormalizeDC(dc)
			validMap[norm] = true
			if catalog.IsAvailableForOrder(st) {
				inStockMap[norm] = true
			}
		}
	}
	for _, dc := range telegram.StandardDCs {
		if validMap[dc] {
			validDCs = append(validDCs, dc)
		}
		if inStockMap[dc] {
			inStockDCs = append(inStockDCs, dc)
		}
	}
	for dc := range validMap {
		found := false
		for _, v := range validDCs {
			if v == dc {
				found = true
				break
			}
		}
		if !found {
			validDCs = append(validDCs, dc)
			if inStockMap[dc] {
				inStockDCs = append(inStockDCs, dc)
			}
		}
	}
	return validDCs, inStockDCs
}

func inStockDisplayDCs(state *app.State, planCode, accountID string) []string {
	_, stock := validAndInStockDCs(state, planCode, accountID)
	return stock
}

func startPlanPicker(state *app.State, mon *monitor.Monitor, chatID interface{}, replyTo int64, mode string) {
	renderCategoryPicker(state, mon, chatID, 0, mode, false)
}

func showDCPicker(state *app.State, chatID interface{}, messageID int64, mode, planCode string, edit bool) {
	accountID := telegram.DefaultAccountID(state)
	validDCs, stock := validAndInStockDCs(state, planCode, accountID)
	targetDCs := validDCs
	if len(targetDCs) == 0 {
		targetDCs = telegram.StandardDCs
	}
	stockSet := map[string]bool{}
	for _, d := range stock {
		stockSet[d] = true
	}
	btns := []map[string]string{}
	for _, dc := range targetDCs {
		dot := "🔴 "
		if stockSet[dc] {
			dot = "🟢 "
		}
		if mode == "m" || mode == "pr" {
			dot = ""
		}
		data := "i:D:" + mode + ":" + planCode + ":" + dc
		if len(data) > 64 {
			continue
		}
		btns = append(btns, telegram.CallbackButton(dot+telegram.DisplayDC(dc), data))
	}
	rows := telegram.ChunkButtons(btns, 3)

	extra := []map[string]string{}
	if mode == "m" {
		extra = append(extra, telegram.CallbackButton("🌐 监控全部机房(推荐)", "i:D:m:"+planCode+":all"))
	} else if mode == "b" || mode == "q" {
		allStock := "i:A:" + mode + ":" + planCode
		allAny := "i:Z:" + mode + ":" + planCode
		if len(allStock) <= 64 {
			extra = append(extra, telegram.CallbackButton("⚡ 全选有货", allStock))
		}
		if len(allAny) <= 64 {
			extra = append(extra, telegram.CallbackButton("🌐 全部机房(含缺货)", allAny))
		}
	}
	if len(extra) > 0 {
		rows = append([][]map[string]string{extra}, rows...)
	}
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔙 返回系列分类", "i:cat:"+mode+":root"),
	})

	text := fmt.Sprintf("型号 %s\n请选择目标机房（🟢有现货 🔴缺货挂机抢）：", planCode)
	if mode == "m" {
		text = fmt.Sprintf("👀 型号 %s\n请选择要监控的机房（推荐监控全部机房，放货第一时间推送）：", planCode)
	} else if mode == "pr" {
		text = fmt.Sprintf("💰 型号 %s\n请选择要询价的目标机房：", planCode)
	} else {
		if len(stock) > 0 {
			up := make([]string, len(stock))
			for i, d := range stock {
				up[i] = telegram.DisplayDC(d)
			}
			text += "\n当前有货: " + strings.Join(up, ", ")
		} else {
			text += "\n当前全区缺货，选机房后将自动加入抢购队列挂机。"
		}
	}
	markup := telegram.InlineKeyboard(rows)
	if edit && messageID > 0 {
		if telegram.EditMessage(state, chatID, messageID, text, markup) {
			return
		}
	}
	_, _ = telegram.SendToChat(state, chatID, text, markup)
}

func enqueueWizardDCs(state *app.State, chatID interface{}, messageID int64, mode, planCode string, dcs []string, accountID string, autoPay bool) {
	if len(dcs) == 0 {
		return
	}
	if accountID == "" {
		accountID = telegram.DefaultAccountID(state)
	}
	quick := mode == "b"
	chatStr := telegram.ChatIDString(chatID)
	ids := []string{}
	okN, failN := 0, 0
	var lastErr string
	for _, dc := range dcs {
		item := telegram.NewTelegramQueueItem(accountID, planCode, dc, nil)
		item.TelegramChatID = chatStr
		item.TelegramMessageID = messageID
		if quick {
			item.QuickOrder = true
			item.Priority = 100
			item.RetryInterval = 2
			item.MaxRetries = 20
		}
		item.AutoPay = autoPay
		res := telegram.EnqueueTelegram(state, item, false)
		if res.Success {
			okN++
			ids = append(ids, res.ItemIDs...)
		} else {
			failN++
			lastErr = res.Message
		}
	}
	up := make([]string, len(dcs))
	for i, d := range dcs {
		up[i] = telegram.DisplayDC(d)
	}
	acc, _ := state.FindAccount(accountID)
	accLabel := acc.Name
	if accLabel == "" {
		accLabel = accountID
	}
	kind := "抢购队列"
	if quick {
		kind = "极速下单"
	}
	text := fmt.Sprintf("⏳ 排队中…（%s）\n\n型号: %s\n机房: %s\n账户: %s\n已创建 %d 个任务",
		kind, planCode, strings.Join(up, ", "), accLabel, okN)
	if failN > 0 {
		text += fmt.Sprintf("\n%d 个未入队：%s", failN, lastErr)
	}
	text += "\n\n有货后将自动提交 OVH，进度会实时更新这条消息。"
	var markup map[string]interface{}
	if len(ids) == 1 {
		short := rememberShort(ids[0])
		markup = telegram.InlineKeyboard([][]map[string]string{
			{telegram.CallbackButton("⏹ 取消抢购任务", "i:T:"+short)},
		})
	} else if len(ids) > 1 {
		markup = telegram.InlineKeyboard([][]map[string]string{
			{telegram.CallbackButton("🛑 取消全部任务", "i:T:all")},
		})
	} else {
		markup = telegram.EmptyInlineKeyboard()
	}
	telegram.EditMessage(state, chatID, messageID, text, markup)
	telegram.BindQueueTelegram(state, ids, chatStr, messageID)
}

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
		telegram.AnswerCallback(state, cbID, "已选 "+plan, false)
		showDCPicker(state, chatID, messageID, mode, plan, true)
	case "D":
		if len(parts) < 5 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		mode, plan, dc := parts[2], parts[3], parts[4]
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
				mon.AddSubscription(plan, dcs, true, false, serverName, nil, nil, false, 0, "", false)
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
			return true
		}
		if mode == "pr" {
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
		}
		telegram.AnswerCallback(state, cbID, telegram.DisplayDC(dc), false)
		enqueueWizardDCs(state, chatID, messageID, mode, plan, []string{dc}, "", false)
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
		enqueueWizardDCs(state, chatID, messageID, mode, plan, dcs, "", false)
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
		enqueueWizardDCs(state, chatID, messageID, mode, plan, targetDCs, "", false)
	case "C":
		if len(parts) < 4 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		btnID := resolveShort(parts[2])
		accID := resolveShort(parts[3])
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
			mon.AddSubscription(plan, nil, true, false, serverName, nil, nil, false, 0, "", false)
			mon.SaveToDB()
			if !mon.Running() {
				mon.Start()
			}
		}
		telegram.AnswerCallback(state, cbID, "已添加 "+plan+" 全机房监控", false)
		_ = telegram.EditMessage(state, chatID, messageID, fmt.Sprintf("✅ 已成功添加 %s 全机房库存监控，有货时将通过 Telegram 自动推送！", plan), telegram.EmptyInlineKeyboard())
	case "T":
		if len(parts) < 3 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		target := parts[2]
		if target == "all" {
			state.QueueMu.Lock()
			count := len(state.Queue)
			state.Queue = nil
			state.QueueMu.Unlock()
			_ = state.SaveQueue()
			telegram.AnswerCallback(state, cbID, "已取消全部任务", false)
			_ = telegram.EditMessage(state, chatID, messageID, fmt.Sprintf("🛑 已终止并清空全部 %d 个抢购任务。", count), telegram.EmptyInlineKeyboard())
			return true
		}
		taskID := resolveShort(target)
		if taskID == "" {
			taskID = target
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
			_ = state.SaveQueue()
			telegram.AnswerCallback(state, cbID, "抢购任务已取消", false)
			text := fmt.Sprintf("🛑 抢购任务已取消\n\n📦 型号: %s\n📍 机房: %s\nℹ️ 说明: 用户已在 Telegram 中取消了此任务",
				foundItem.PlanCode, telegram.DisplayDCFull(foundItem.Datacenter))
			_ = telegram.EditMessage(state, chatID, messageID, text, telegram.EmptyInlineKeyboard())
		} else {
			telegram.AnswerCallback(state, cbID, "任务已不存在或已结束", true)
			_ = telegram.EditMessage(state, chatID, messageID, "🛑 抢购任务已不存在或已结束。", telegram.EmptyInlineKeyboard())
		}
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
			state.Queue = nil
			state.QueueMu.Unlock()
			_ = state.SaveQueue()
			telegram.AnswerCallback(state, cbID, "已清空队列", false)
			showTasks(state, chatID, messageID, true)
			return true
		}
		taskID := resolveShort(target)
		if taskID == "" {
			taskID = target
		}
		state.QueueMu.Lock()
		found := false
		newQueue := make([]types.QueueItem, 0, len(state.Queue))
		for _, it := range state.Queue {
			if it.ID == taskID || strings.HasPrefix(it.ID, taskID) {
				found = true
				continue
			}
			newQueue = append(newQueue, it)
		}
		state.Queue = newQueue
		state.QueueMu.Unlock()
		if found {
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
	case "acc":
		telegram.AnswerCallback(state, cbID, "账户管理", false)
		showAccounts(state, chatID, messageID, true)
		return true
	case "S":
		if len(parts) < 3 {
			telegram.AnswerCallback(state, cbID, "按钮无效", true)
			return true
		}
		accID := resolveShort(parts[2])
		if accID == "" {
			accID = parts[2]
		}
		if state.DB != nil {
			_ = state.DB.SetDefaultAccount(accID)
		}
		_ = state.ReloadAccounts()
		acc, _ := state.FindAccount(accID)
		accName := acc.Name
		if accName == "" {
			accName = accID
		}
		telegram.AnswerCallback(state, cbID, "已切换默认账户: "+accName, false)
		showAccounts(state, chatID, messageID, true)
	default:
		telegram.AnswerCallback(state, cbID, "未知按钮", true)
	}
	return true
}

func showAccountPicker(state *app.State, chatID interface{}, messageID int64, buttonID string) {
	state.AccountsMu.RLock()
	accs := append([]types.OVHAccount{}, state.Accounts...)
	state.AccountsMu.RUnlock()
	if len(accs) == 0 {
		telegram.EditMessage(state, chatID, messageID, "❌ 未配置任何 OVH 账户", telegram.EmptyInlineKeyboard())
		return
	}
	btnShort := rememberShort(buttonID)
	btns := []map[string]string{}
	for _, a := range accs {
		accShort := rememberShort(a.ID)
		data := "i:C:" + btnShort + ":" + accShort
		if len(data) > 64 {
			continue
		}
		label := a.Name + " · " + strings.ToUpper(a.Zone)
		if a.IsDefault {
			label += " ★"
		}
		btns = append(btns, telegram.CallbackButton(label, data))
	}
	telegram.EditMessage(state, chatID, messageID, "请选择下单账户：", telegram.InlineKeyboard(telegram.ChunkButtons(btns, 1)))
}

func splitButtonDCs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := []string{}
	seen := map[string]bool{}
	for _, p := range parts {
		dc := telegram.NormalizeDC(p)
		if dc == "" || seen[dc] {
			continue
		}
		seen[dc] = true
		out = append(out, dc)
	}
	return out
}

func enqueueFromNotifyButton(state *app.State, mon *monitor.Monitor, cbID string, chatID interface{}, messageID int64, buttonID, action, accountOverride string, autoPay bool) {
	row, ok, err := state.DB.GetTelegramButton(buttonID)
	if err != nil || !ok {
		if cached := mon.MessageUUIDCacheLookup(buttonID); cached != nil {
			mode := "q"
			if action == "sniper" {
				mode = "b"
			}
			dcs := splitButtonDCs(cached.Datacenter)
			if len(dcs) == 0 && cached.Datacenter != "" {
				dcs = []string{telegram.NormalizeDC(cached.Datacenter)}
			}
			enqueueWizardDCs(state, chatID, messageID, mode, cached.PlanCode, dcs, accountOverride, autoPay)
			return
		}
		telegram.AnswerCallback(state, cbID, "按钮已失效", true)
		return
	}
	if time.Since(time.Unix(int64(row.CreatedAt), 0)) > telegram.ButtonTTL {
		telegram.AnswerCallback(state, cbID, "该按钮已过期，请等待新的上架通知", true)
		return
	}
	dcs := splitButtonDCs(row.Datacenter)
	if len(dcs) == 0 {
		dcs = []string{telegram.NormalizeDC(row.Datacenter)}
	}
	accID := accountOverride
	if accID == "" {
		accID = strings.TrimSpace(row.AccountID)
	}
	mode := "q"
	if action == "sniper" {
		mode = "b"
	}
	enqueueWizardDCs(state, chatID, messageID, mode, row.PlanCode, dcs, accID, autoPay)
}

func showTasks(state *app.State, chatID interface{}, messageID int64, edit bool) {
	state.QueueMu.Lock()
	active := make([]types.QueueItem, 0, len(state.Queue))
	for _, it := range state.Queue {
		if it.Status == "running" || it.Status == "pending" || it.Status == "paused" {
			active = append(active, it)
		}
	}
	state.QueueMu.Unlock()

	if len(active) == 0 {
		text := "📋 当前抢购队列为空，没有正在运行的任务。\n\n可发送 /buy 选择型号机房加入抢购。"
		if edit && messageID > 0 {
			_ = telegram.EditMessage(state, chatID, messageID, text, telegram.EmptyInlineKeyboard())
			return
		}
		_, _ = telegram.SendToChat(state, chatID, text, nil)
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📋 抢购任务队列（共 %d 个运行中）:\n\n", len(active)))
	btns := []map[string]string{}
	limit := 6
	for i, it := range active {
		if i < limit {
			dcUpper := strings.ToUpper(it.Datacenter)
			b.WriteString(fmt.Sprintf("%d. %s @ %s (重试: %d 次)\n", i+1, it.PlanCode, dcUpper, it.RetryCount))
			shortID := rememberShort(it.ID)
			btns = append(btns, telegram.CallbackButton(fmt.Sprintf("⏹ 停止 %s@%s", it.PlanCode, dcUpper), "i:Tk:"+shortID))
		}
	}
	if len(active) > limit {
		b.WriteString(fmt.Sprintf("\n…另有 %d 个任务未列出\n", len(active)-limit))
	}
	rows := telegram.ChunkButtons(btns, 1)
	if len(active) > 1 {
		rows = append(rows, []map[string]string{
			telegram.CallbackButton("🛑 停止全部抢购任务", "i:Tk:all"),
		})
	}
	markup := telegram.InlineKeyboard(rows)
	if edit && messageID > 0 {
		_ = telegram.EditMessage(state, chatID, messageID, b.String(), markup)
		return
	}
	_, _ = telegram.SendToChat(state, chatID, b.String(), markup)
}

func showAccounts(state *app.State, chatID interface{}, messageID int64, edit bool) {
	state.AccountsMu.RLock()
	accs := append([]types.OVHAccount{}, state.Accounts...)
	state.AccountsMu.RUnlock()

	if len(accs) == 0 {
		text := "👤 系统中尚未添加任何 OVH 账户。\n请先在网页控制台「设置」中添加账户。"
		if edit && messageID > 0 {
			_ = telegram.EditMessage(state, chatID, messageID, text, telegram.EmptyInlineKeyboard())
			return
		}
		_, _ = telegram.SendToChat(state, chatID, text, nil)
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("👤 OVH 账户列表（共 %d 个）:\n\n", len(accs)))
	btns := []map[string]string{}
	for i, a := range accs {
		defMark := ""
		if a.IsDefault {
			defMark = " ★[当前默认]"
		}
		zone := strings.ToUpper(a.Zone)
		if zone == "" {
			zone = "EU"
		}
		b.WriteString(fmt.Sprintf("%d. %s (%s / %s)%s\n", i+1, a.Name, zone, a.Endpoint, defMark))
		if !a.IsDefault {
			shortID := rememberShort(a.ID)
			btns = append(btns, telegram.CallbackButton("设为默认: "+a.Name, "i:S:"+shortID))
		}
	}
	rows := telegram.ChunkButtons(btns, 1)
	markup := telegram.InlineKeyboard(rows)
	if edit && messageID > 0 {
		_ = telegram.EditMessage(state, chatID, messageID, b.String(), markup)
		return
	}
	_, _ = telegram.SendToChat(state, chatID, b.String(), markup)
}

func showStockCardWithButtons(state *app.State, mon *monitor.Monitor, chatID interface{}, replyTo int64, planCode string) {
	planCode = strings.TrimSpace(planCode)
	if planCode == "" {
		telegram.SendReply(state, chatID, "用法: /stock <planCode>\n例: /stock 24ska01", replyTo)
		return
	}
	accountID := telegram.DefaultAccountID(state)
	stock := inStockDisplayDCs(state, planCode, accountID)
	rawText := cmdStock(state, []string{planCode})

	btns := []map[string]string{}
	// 如果有货，生成前 4 个有货机房的秒抢按钮
	for i, d := range stock {
		if i >= 4 {
			break
		}
		btns = append(btns, telegram.CallbackButton("⚡ "+telegram.DisplayDC(d)+" 极速开抢", "i:D:b:"+planCode+":"+d))
	}
	actionRow := []map[string]string{
		telegram.CallbackButton("👀 监控该型号", "i:M:"+planCode),
		telegram.CallbackButton("📥 选机房排队", "i:P:q:"+planCode),
	}
	rows := telegram.ChunkButtons(btns, 2)
	rows = append(rows, actionRow)
	markup := telegram.InlineKeyboard(rows)
	_, _ = telegram.SendToChat(state, chatID, rawText, markup)
}

func showMonitorManager(state *app.State, mon *monitor.Monitor, chatID interface{}, messageID int64, edit bool) {
	if mon == nil {
		text := "❌ 监控模块未就绪"
		if edit && messageID > 0 {
			_ = telegram.EditMessage(state, chatID, messageID, text, telegram.EmptyInlineKeyboard())
			return
		}
		_, _ = telegram.SendToChat(state, chatID, text, nil)
		return
	}
	subs := mon.Snapshot()
	if len(subs) == 0 {
		text := "👀 当前暂无正在运行的库存监控。\n\n点击下方按钮即可选择机型开启全天候监控："
		markup := telegram.InlineKeyboard([][]map[string]string{
			{telegram.CallbackButton("➕ 选择机型添加监控", "i:cat:m:root")},
		})
		if edit && messageID > 0 {
			_ = telegram.EditMessage(state, chatID, messageID, text, markup)
			return
		}
		_, _ = telegram.SendToChat(state, chatID, text, markup)
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("👀 已监控机型列表（共 %d 款）:\n\n", len(subs)))
	btns := []map[string]string{}
	limit := 8
	for i, s := range subs {
		if s == nil || s.PlanCode == "" {
			continue
		}
		if i < limit {
			dcStr := "全部机房"
			if len(s.Datacenters) > 0 {
				up := make([]string, len(s.Datacenters))
				for idx, d := range s.Datacenters {
					up[idx] = telegram.DisplayDC(d)
				}
				dcStr = strings.Join(up, ", ")
			}
			namePart := s.PlanCode
			if s.ServerName != "" {
				namePart += " (" + s.ServerName + ")"
			}
			b.WriteString(fmt.Sprintf("%d. %s\n   机房: %s\n", i+1, namePart, dcStr))
			btns = append(btns, telegram.CallbackButton("🗑 取消 "+s.PlanCode, "i:mon:del:"+s.PlanCode))
		}
	}
	if len(subs) > limit {
		b.WriteString(fmt.Sprintf("\n…另有 %d 款未展示\n", len(subs)-limit))
	}
	rows := telegram.ChunkButtons(btns, 2)
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("➕ 添加新监控", "i:cat:m:root"),
		telegram.CallbackButton("🗑 清空全部监控", "i:mon:clear"),
	})
	markup := telegram.InlineKeyboard(rows)
	if edit && messageID > 0 {
		_ = telegram.EditMessage(state, chatID, messageID, b.String(), markup)
		return
	}
	_, _ = telegram.SendToChat(state, chatID, b.String(), markup)
}

