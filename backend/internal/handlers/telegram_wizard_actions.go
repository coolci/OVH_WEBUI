package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

func enqueueWizardDCs(state *app.State, chatID interface{}, messageID int64, mode, planCode string, dcs []string, accountID string, options []string, configLabel string, autoPay bool) int {
	if len(dcs) == 0 {
		return 0
	}
	if accountID == "" {
		accountID = telegram.ActiveAccountID(state)
	}
	quick := mode == "b"
	chatStr := telegram.ChatIDString(chatID)

	type taskEntry struct {
		ID         string
		Datacenter string
	}
	var createdTasks []taskEntry
	okN, failN := 0, 0
	var lastErr string

	for _, dc := range dcs {
		item := telegram.NewTelegramQueueItem(accountID, planCode, dc, options)
		item.TelegramChatID = chatStr
		item.TelegramMessageID = messageID
		if quick {
			item.QuickOrder = true
			item.Priority = 100
			item.MaxRetries = 20
		}
		item.RetryInterval = state.Config.RetryInterval()
		item.AutoPay = autoPay
		res := telegram.EnqueueTelegram(state, item, false)
		if res.Success {
			okN++
			for _, id := range res.ItemIDs {
				createdTasks = append(createdTasks, taskEntry{ID: id, Datacenter: dc})
			}
		} else {
			failN++
			lastErr = res.Message
		}
	}

	acc, _ := state.FindAccount(accountID)
	accLabel := telegram.AccountLabel(acc)
	if accLabel == "" {
		accLabel = accountID
	}
	kind := "抢购队列挂机"
	if quick {
		kind = "极速下单"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("⚡ 抢购任务已建立入队 · %s\n", kind))
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("📦 目标型号: " + planCode + "\n")
	cfgText := "任意硬件配置 (自动优选)"
	if configLabel != "" {
		cfgText = configLabel
	} else if len(options) > 0 {
		cfgText = telegram.HumanizeOptionCodes(options)
	}
	b.WriteString("⚙️ 锁定规格: " + cfgText + "\n")
	b.WriteString("👤 执行账户: " + accLabel + "\n")
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString(fmt.Sprintf("📍 监听节点 (%d 个机房并发抢占):\n", okN))
	for _, t := range createdTasks {
		b.WriteString("  • " + telegram.DisplayDCFull(t.Datacenter) + "\n")
	}
	if failN > 0 {
		b.WriteString(fmt.Sprintf("\n⚠️ 异常未入队 (%d个): %s\n", failN, lastErr))
	}
	b.WriteString("\n🚀 状态: 后台挂机监听中，检测到上架即毫秒抢占并锁单支付！")

	var btnRows [][]map[string]string
	var cancelBtns []map[string]string
	for _, t := range createdTasks {
		short := rememberShort(state, t.ID, "task")
		cancelBtns = append(cancelBtns, telegram.CallbackButton("⏹ 取消 "+strings.ToUpper(t.Datacenter), "i:T:one:"+short))
	}
	if len(cancelBtns) > 0 {
		btnRows = append(btnRows, telegram.ChunkButtons(cancelBtns, 3)...)
	}
	if len(createdTasks) > 1 {
		btnRows = append(btnRows, []map[string]string{
			telegram.CallbackButton(fmt.Sprintf("🛑 取消本次全部 (%d个)", len(createdTasks)), fmt.Sprintf("i:T:m:%d", messageID)),
		})
	}
	markup := telegram.InlineKeyboard(btnRows)

	telegram.EditMessage(state, chatID, messageID, b.String(), markup)

	var ids []string
	for _, t := range createdTasks {
		ids = append(ids, t.ID)
	}
	telegram.BindQueueTelegram(state, ids, chatStr, messageID)
	return okN
}

func showAccountPicker(state *app.State, chatID interface{}, messageID int64, buttonID string) {
	state.AccountsMu.RLock()
	accs := append([]types.OVHAccount{}, state.Accounts...)
	state.AccountsMu.RUnlock()
	if len(accs) == 0 {
		telegram.EditMessage(state, chatID, messageID, "❌ 未配置任何 OVH 账户", telegram.EmptyInlineKeyboard())
		return
	}
	btnShort := rememberShort(state, buttonID, "btn")
	btns := []map[string]string{}
	for _, a := range accs {
		accShort := rememberShort(state, a.ID, "acc")
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
	mode := "q"
	if action == "sniper" {
		mode = "b"
	}
	claimed := false
	var planCode, datacenter, accID, cfgLabel, optsJSON string

	if buttonID != "" && state.TelegramRepo != nil {
		row, ok, err := state.TelegramRepo.ClaimTelegramButton(buttonID)
		if err != nil {
			state.Logger.Error("认领一键下单按钮失败: "+err.Error(), "telegram")
			telegram.AnswerCallback(state, cbID, "认领按钮失败", true)
			return
		}
		if ok {
			if time.Since(time.Unix(int64(row.CreatedAt), 0)) > telegram.ButtonTTL {
				_ = state.TelegramRepo.UnclaimTelegramButton(buttonID)
				telegram.AnswerCallback(state, cbID, "该按钮已过期，请等待新的上架通知", true)
				return
			}
			claimed = true
			planCode, datacenter, accID = row.PlanCode, row.Datacenter, strings.TrimSpace(row.AccountID)
			optsJSON = row.Options
			if row.ConfigInfo != "" {
				var cfgMap map[string]interface{}
				if err := json.Unmarshal([]byte(row.ConfigInfo), &cfgMap); err == nil {
					if disp, ok := cfgMap["display"].(string); ok {
						cfgLabel = disp
					}
				}
			}
		} else if _, exists, _ := state.TelegramRepo.GetTelegramButton(buttonID); exists {
			telegram.AnswerCallback(state, cbID, "该按钮已使用过", true)
			return
		}
	}

	var opts []string
	if !claimed {
		if cached := mon.MessageUUIDCacheLookup(buttonID); cached != nil {
			planCode, datacenter = cached.PlanCode, cached.Datacenter
			opts = cached.Options
			if cached.ConfigInfo != nil {
				if d, ok := cached.ConfigInfo["display"].(string); ok {
					cfgLabel = d
				}
			}
		} else {
			telegram.AnswerCallback(state, cbID, "按钮已失效", true)
			return
		}
	} else if optsJSON != "" {
		_ = json.Unmarshal([]byte(optsJSON), &opts)
	}

	if accountOverride != "" {
		accID = accountOverride
	}
	dcs := splitButtonDCs(datacenter)
	if len(dcs) == 0 && datacenter != "" {
		dcs = []string{telegram.NormalizeDC(datacenter)}
	}
	okN := enqueueWizardDCs(state, chatID, messageID, mode, planCode, dcs, accID, opts, cfgLabel, autoPay)
	if claimed && okN == 0 {
		_ = state.TelegramRepo.UnclaimTelegramButton(buttonID)
	}
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
		text := fmt.Sprintf("📋 抢购任务监控队列\n%s\n✨ 当前队列为空，暂无进行中的抢购任务。\n\n💡 可发送 /buy 或点击下方按钮点选机型加入抢购！", telegram.CardDivider)
		markup := telegram.InlineKeyboard([][]map[string]string{
			{
				telegram.CallbackButton("⚡ 快速下单", "i:cat:b:root"),
				telegram.CallbackButton("📥 挑选排队", "i:cat:q:root"),
			},
			{
				telegram.CallbackButton("🔙 返回主菜单", "i:dash:refresh"),
			},
		})
		if edit && messageID > 0 {
			_ = telegram.EditMessage(state, chatID, messageID, text, markup)
			return
		}
		_, _ = telegram.SendToChat(state, chatID, text, markup)
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📋 抢购任务队列 (运行中: %d 个)\n", len(active)))
	b.WriteString(telegram.CardDivider + "\n")
	btns := []map[string]string{}
	limit := 6
	for i, it := range active {
		if i < limit {
			dcFull := telegram.DisplayDCFull(it.Datacenter)
			statusBadge := "⏳ 挂机排队中"
			if it.Status == "paused" {
				statusBadge = "⏸ 已暂停"
			}
			cfgDisplay := "任意硬件配置"
			if len(it.Options) > 0 {
				cfgDisplay = telegram.HumanizeOptionCodes(it.Options)
			}
			b.WriteString(fmt.Sprintf("%d️⃣ 📦 %s\n   📍 %s ｜ ⚙️ %s\n   ⚡ 状态: %s (已轮询 %d 轮 · 周期 %ds)\n\n",
				i+1, it.PlanCode, dcFull, cfgDisplay, statusBadge, it.RetryCount, it.RetryInterval))
			shortID := rememberShort(state, it.ID, "task")
			btns = append(btns, telegram.CallbackButton(fmt.Sprintf("⏹ 停止 %s@%s", it.PlanCode, telegram.DisplayDC(it.Datacenter)), "i:Tk:"+shortID))
		}
	}
	if len(active) > limit {
		b.WriteString(fmt.Sprintf("…另有 %d 个任务在后台运行中\n", len(active)-limit))
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("👇 点击下方按钮可中止单个或全部挂机任务：")
	rows := telegram.ChunkButtons(btns, 1)
	bottomRow := []map[string]string{}
	if len(active) > 1 {
		bottomRow = append(bottomRow, telegram.CallbackButton("🛑 停止全部抢购任务", "i:Tk:all"))
	}
	bottomRow = append(bottomRow, telegram.CallbackButton("🔄 刷新队列", "i:Tk:list"))
	bottomRow = append(bottomRow, telegram.CallbackButton("🔙 返回主菜单", "i:dash:refresh"))
	rows = append(rows, bottomRow)

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
		text := fmt.Sprintf("👤 OVH 账户矩阵控制台\n%s\n⚠️ 系统中尚未配置任何 OVH 账户。\n\n请在 Web 控制台「账户管理」中添加 OVH API 凭据。", telegram.CardDivider)
		if edit && messageID > 0 {
			_ = telegram.EditMessage(state, chatID, messageID, text, telegram.EmptyInlineKeyboard())
			return
		}
		_, _ = telegram.SendToChat(state, chatID, text, nil)
		return
	}

	activeAcc, _ := telegram.ActiveAccount(state)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("👤 OVH 账户矩阵控制台 (共 %d 个账户)\n", len(accs)))
	b.WriteString(telegram.CardDivider + "\n")
	btns := []map[string]string{}
	for i, a := range accs {
		isActive := a.ID == activeAcc.ID
		mark := "⚪ 空闲"
		if isActive {
			mark = "🟢 [当前活跃]"
		} else if a.IsDefault {
			mark = "🌟 [系统默认]"
		}
		zone := strings.ToUpper(a.Zone)
		if zone == "" {
			zone = "EU"
		}
		b.WriteString(fmt.Sprintf("%d️⃣ 👤 %s  %s\n   🌐 区域: %s ｜ 终端: %s\n\n",
			i+1, a.Name, mark, zone, a.Endpoint))
		if !isActive {
			shortID := rememberShort(state, a.ID, "acc")
			btns = append(btns, telegram.CallbackButton("👉 切换至: "+a.Name, "i:S:"+shortID))
		}
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("💡 活跃账户将作为当前 Telegram 下单与监控的执行身份。\n👇 点击下方按钮可一键无缝切换：")
	rows := telegram.ChunkButtons(btns, 1)
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔄 刷新账户状态", "i:acc:list"),
		telegram.CallbackButton("🔙 返回主菜单", "i:dash:refresh"),
	})
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
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔙 返回系列分类", "i:cat:s:root"),
		telegram.CallbackButton("🔙 返回控制中心", "i:dash:refresh"),
	})
	markup := telegram.InlineKeyboard(rows)
	if replyTo > 0 {
		if telegram.EditMessage(state, chatID, replyTo, rawText, markup) {
			return
		}
	}
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
		text := fmt.Sprintf("📡 毫秒级补货监控中枢\n%s\n✨ 当前暂无监控中的机型。\n\n一旦官方有新机器放货，监控引擎将在毫秒内向 Telegram 推送通知并支持一键秒抢！\n%s\n👇 点击下方按钮选择机型开启全天候监控：",
			telegram.CardDivider, telegram.CardDivider)
		markup := telegram.InlineKeyboard([][]map[string]string{
			{telegram.CallbackButton("➕ 选择机型添加监控", "i:cat:m:root")},
			{telegram.CallbackButton("🔙 返回主菜单", "i:dash:refresh")},
		})
		if edit && messageID > 0 {
			_ = telegram.EditMessage(state, chatID, messageID, text, markup)
			return
		}
		_, _ = telegram.SendToChat(state, chatID, text, markup)
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📡 补货监控控制台 (监控中: %d 款机型)\n", len(subs)))
	b.WriteString(telegram.CardDivider + "\n")
	btns := []map[string]string{}
	limit := 8
	for i, s := range subs {
		if s == nil || s.PlanCode == "" {
			continue
		}
		if i < limit {
			dcStr := "全球所有机房"
			if len(s.Datacenters) > 0 {
				up := make([]string, len(s.Datacenters))
				for idx, d := range s.Datacenters {
					up[idx] = telegram.DisplayDC(d)
				}
				dcStr = strings.Join(up, "、")
			}
			namePart := s.PlanCode
			if s.ServerName != "" {
				namePart += " (" + s.ServerName + ")"
			}
			cfgStr := "全部硬件规格组合"
			if len(s.Options) > 0 {
				cfgStr = telegram.HumanizeOptionCodes(s.Options)
			}
			b.WriteString(fmt.Sprintf("%d️⃣ 📦 %s\n   📍 机房: %s\n   ⚙️ 规格: %s\n\n",
				i+1, namePart, dcStr, cfgStr))
			btns = append(btns, telegram.CallbackButton("🗑 取消 "+s.PlanCode, "i:mon:del:"+s.PlanCode))
		}
	}
	if len(subs) > limit {
		b.WriteString(fmt.Sprintf("…另有 %d 款监控在后台运行中\n", len(subs)-limit))
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("👇 点击下方按钮管理或添加监控：")
	rows := telegram.ChunkButtons(btns, 2)
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("➕ 添加新监控", "i:cat:m:root"),
		telegram.CallbackButton("🗑 清空全部监控", "i:mon:clear"),
	})
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔙 返回主菜单", "i:dash:refresh"),
	})
	markup := telegram.InlineKeyboard(rows)
	if edit && messageID > 0 {
		_ = telegram.EditMessage(state, chatID, messageID, b.String(), markup)
		return
	}
	_, _ = telegram.SendToChat(state, chatID, b.String(), markup)
}
