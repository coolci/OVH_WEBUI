package handlers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

type wizardPlan struct {
	Code    string
	Name    string
	CPU     string
	Memory  string
	Storage string
}

func getPlansForCategory(state *app.State, mon *monitor.Monitor, category string) (string, []wizardPlan) {
	state.ServerPlansMu.RLock()
	allPlans := append([]types.ServerPlan{}, state.ServerPlans...)
	state.ServerPlansMu.RUnlock()

	planMap := make(map[string]types.ServerPlan, len(allPlans))
	for _, p := range allPlans {
		planMap[p.PlanCode] = p
	}

	makePlan := func(code, name string) wizardPlan {
		p, ok := planMap[code]
		if ok {
			cleanName := name
			if cleanName == "" {
				cleanName = p.Name
			}
			return wizardPlan{
				Code:    code,
				Name:    cleanName,
				CPU:     p.CPU,
				Memory:  p.Memory,
				Storage: p.Storage,
			}
		}
		return wizardPlan{Code: code, Name: name}
	}

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
			out = append(out, makePlan(code, name))
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
		return "📋 我的监控/队列", out

	case "instock":
		out := []wizardPlan{}
		seen := map[string]bool{}
		for _, s := range allPlans {
			for _, dc := range s.Datacenters {
				if catalog.IsAvailableForOrder(dc.Availability) || dc.Availability == "available" {
					if !seen[s.PlanCode] {
						seen[s.PlanCode] = true
						out = append(out, makePlan(s.PlanCode, s.Name))
					}
					break
				}
			}
		}
		if mon != nil {
			for _, sub := range mon.Snapshot() {
				if sub == nil || sub.PlanCode == "" || seen[sub.PlanCode] {
					continue
				}
				for _, st := range sub.LastStatus {
					if st == "available" || catalog.IsAvailableForOrder(st) {
						seen[sub.PlanCode] = true
						name := sub.ServerName
						if name == "" {
							name = sub.PlanCode
						}
						out = append(out, makePlan(sub.PlanCode, name))
						break
					}
				}
			}
		}
		return "🟢 实时有货机型", out

	case "entry", "ks":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			if strings.HasPrefix(p, "24sk") || strings.HasPrefix(p, "ks") || strings.Contains(p, "kimsufi") || strings.Contains(n, "kimsufi") || strings.Contains(n, "ks-") {
				out = append(out, makePlan(s.PlanCode, s.Name))
			}
		}
		return "🏷️ 特惠入门系列 (<€20/月)", out

	case "storage":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			st := strings.ToLower(s.Storage)
			isStor := strings.Contains(st, "hdd") || strings.Contains(st, "sata") ||
				strings.Contains(st, "sa") || strings.Contains(st, "softraid-2x") ||
				strings.Contains(st, "2x2t") || strings.Contains(st, "2x4t") || strings.Contains(st, "2x6t") ||
				strings.Contains(p, "stor") || strings.Contains(n, "stor") || strings.Contains(n, "storage")
			if isStor {
				out = append(out, makePlan(s.PlanCode, s.Name))
			}
		}
		return "💾 大盘存储系列 (多盘/大容量)", out

	case "perf", "rise":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			st := strings.ToLower(s.Storage)
			if strings.HasPrefix(p, "24adv") || strings.HasPrefix(p, "adv") || strings.Contains(n, "advance") {
				continue
			}
			isPerf := strings.HasPrefix(p, "24rise") || strings.HasPrefix(p, "rise") ||
				strings.HasPrefix(p, "24game") || strings.HasPrefix(p, "game") ||
				strings.Contains(n, "rise") || strings.Contains(n, "game") ||
				strings.HasPrefix(p, "24sys") || strings.HasPrefix(p, "sys") ||
				strings.Contains(st, "nvme") || strings.Contains(st, "ssd")
			if isPerf {
				out = append(out, makePlan(s.PlanCode, s.Name))
			}
		}
		return "⚡ 性能主力系列 (高频/NVMe)", out

	case "flagship", "adv":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			m := strings.ToLower(s.Memory)
			isFlag := strings.HasPrefix(p, "24adv") || strings.HasPrefix(p, "adv") ||
				strings.Contains(n, "advance") || strings.Contains(m, "64g") ||
				strings.Contains(m, "128g") || strings.Contains(m, "256g") ||
				strings.Contains(m, "512g") || strings.Contains(m, "64 gb") ||
				strings.Contains(m, "128 gb")
			if isFlag {
				out = append(out, makePlan(s.PlanCode, s.Name))
			}
		}
		return "🏢 旗舰高配系列 (64G+/企业专有)", out

	case "sys":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			if strings.HasPrefix(p, "sys") || strings.HasPrefix(p, "24sys") || strings.HasPrefix(p, "stor") || strings.Contains(n, "so you start") || strings.Contains(n, "sys-") {
				out = append(out, makePlan(s.PlanCode, s.Name))
			}
		}
		return "⚡ So you Start / SYS 经典系列", out

	case "game":
		out := []wizardPlan{}
		for _, s := range allPlans {
			p := strings.ToLower(s.PlanCode)
			n := strings.ToLower(s.Name)
			if strings.HasPrefix(p, "24game") || strings.HasPrefix(p, "game") || strings.Contains(n, "game") {
				out = append(out, makePlan(s.PlanCode, s.Name))
			}
		}
		return "🎮 Game 游戏高防系列", out

	default: // "all"
		out := make([]wizardPlan, 0, len(allPlans))
		for _, s := range allPlans {
			out = append(out, makePlan(s.PlanCode, s.Name))
		}
		if len(out) == 0 {
			for _, p := range []string{"24ska01", "24rise01", "24game01", "ks-le-1", "sys-le-1"} {
				out = append(out, makePlan(p, p))
			}
		}
		return "🌐 全部服务器型号", out
	}
}

func renderCategoryPicker(state *app.State, mon *monitor.Monitor, chatID interface{}, messageID int64, mode string, edit bool) {
	_, entryPlans := getPlansForCategory(state, mon, "entry")
	_, storPlans := getPlansForCategory(state, mon, "storage")
	_, perfPlans := getPlansForCategory(state, mon, "perf")
	_, flagPlans := getPlansForCategory(state, mon, "flagship")
	_, inStockPlans := getPlansForCategory(state, mon, "instock")
	_, monPlans := getPlansForCategory(state, mon, "mon")
	_, allPlans := getPlansForCategory(state, mon, "all")

	actionText := "快速下单"
	icon := "⚡"
	switch mode {
	case "q":
		actionText = "抢购排队"
		icon = "📥"
	case "s":
		actionText = "查询库存"
		icon = "📦"
	case "m":
		actionText = "添加监控"
		icon = "📡"
	case "pr":
		actionText = "查询价格"
		icon = "💰"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s 服务器选型中心 · %s\n", icon, actionText))
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("🔥 明星标杆机型参考：\n")
	b.WriteString("• KS-1  │ €4.99/月  │ Atom 4C ｜ 4G ｜ 500G HDD\n")
	b.WriteString("• KS-5  │ €17.99/月 │ Xeon-E 6C ｜ 32G ECC ｜ 2×2T HDD\n")
	b.WriteString("• SYS-1 │ €23.99/月 │ Xeon-E 6C ｜ 32G ECC ｜ 2×512G NVMe\n")
	b.WriteString("• Rise-1│ €35.99/月 │ Ryzen 6C ｜ 32G DDR4 ｜ 2×512G NVMe\n")
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("🎯 按核心硬件与需求拆解选型：\n")
	b.WriteString(fmt.Sprintf("🏷️ 特惠入门 (%d款): 低至 €4.99，个人折腾与轻量服务超值首选\n", len(entryPlans)))
	b.WriteString(fmt.Sprintf("💾 大盘存储 (%d款): 2×2T ~ 4×4T 海量机械多盘，归档备份利器\n", len(storPlans)))
	b.WriteString(fmt.Sprintf("⚡ 性能主力 (%d款): 极速 NVMe 高频独立核心，企业生产主力\n", len(perfPlans)))
	b.WriteString(fmt.Sprintf("🏢 旗舰高配 (%d款): 64G~128G+ 专有计算核心，集群与密集负载\n", len(flagPlans)))
	b.WriteString(telegram.CardDivider + "\n")
	if len(inStockPlans) > 0 {
		b.WriteString(fmt.Sprintf("🟢 实时现货: 监控雷达检测到 %d 款机型可直接下单\n", len(inStockPlans)))
	} else {
		b.WriteString("💡 提示: 现货稀缺机型建议选择「📥 抢购排队」全天候挂机抢购\n")
	}

	btns := [][]map[string]string{
		{
			telegram.CallbackButton("🏷️ 特惠入门 (<€20)", "i:cat:"+mode+":entry"),
			telegram.CallbackButton("💾 大盘存储 (多盘)", "i:cat:"+mode+":storage"),
		},
		{
			telegram.CallbackButton("⚡ 性能主力 (NVMe)", "i:cat:"+mode+":perf"),
			telegram.CallbackButton("🏢 旗舰高配 (64G+)", "i:cat:"+mode+":flagship"),
		},
	}

	thirdRow := []map[string]string{}
	stockBtnText := "🟢 实时有货机型速览"
	if len(inStockPlans) > 0 {
		stockBtnText = fmt.Sprintf("🟢 实时有货 (%d款)", len(inStockPlans))
	}
	thirdRow = append(thirdRow, telegram.CallbackButton(stockBtnText, "i:cat:"+mode+":instock"))
	if len(monPlans) > 0 {
		thirdRow = append(thirdRow, telegram.CallbackButton(fmt.Sprintf("📋 我的监控 (%d款)", len(monPlans)), "i:cat:"+mode+":mon"))
	}
	btns = append(btns, thirdRow)

	btns = append(btns, []map[string]string{
		telegram.CallbackButton(fmt.Sprintf("🌐 全部型号列表 (%d款)", len(allPlans)), "i:cat:"+mode+":all"),
	})
	btns = append(btns, []map[string]string{
		telegram.CallbackButton("🔙 返回控制中心", "i:dash:refresh"),
	})

	markup := telegram.InlineKeyboard(btns)
	if edit && messageID > 0 {
		if telegram.EditMessage(state, chatID, messageID, b.String(), markup) {
			return
		}
	}
	_, _ = telegram.SendToChat(state, chatID, b.String(), markup)
}

func renderPlanPage(state *app.State, mon *monitor.Monitor, chatID interface{}, messageID int64, mode, category string, page int, edit bool) {
	catTitle, plans := getPlansForCategory(state, mon, category)
	if len(plans) == 0 {
		text := fmt.Sprintf("%s 暂无现货机型。\n\n💡 建议选择「📥 抢购排队」或「📡 添加监控」，有货时系统将毫秒级自动响应。", catTitle)
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

	pageSize := 6
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

	actionText := "快速下单"
	icon := "⚡"
	switch mode {
	case "q":
		actionText = "抢购排队"
		icon = "📥"
	case "s":
		actionText = "查询库存"
		icon = "📦"
	case "m":
		actionText = "添加监控"
		icon = "📡"
	case "pr":
		actionText = "查询价格"
		icon = "💰"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s 机型列表 · %s\n", icon, catTitle))
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString(fmt.Sprintf("📄 浏览进度: 第 %d / %d 页 (共收录 %d 款型号)\n", page+1, totalPages, len(plans)))
	b.WriteString(telegram.CardDivider + "\n")

	for _, p := range pagePlans {
		cleanName := p.Name
		if idx := strings.Index(cleanName, " | "); idx != -1 {
			cleanName = cleanName[:idx]
		}
		cleanName = strings.TrimSpace(cleanName)

		title := p.Code
		if cleanName != "" && !strings.EqualFold(cleanName, p.Code) {
			title = fmt.Sprintf("%s (%s)", p.Code, cleanName)
		}

		var specs []string
		if p.CPU != "" {
			specs = append(specs, p.CPU)
		}
		if p.Memory != "" {
			specs = append(specs, telegram.HumanizeHardware(p.Memory, ""))
		}
		if p.Storage != "" {
			specs = append(specs, telegram.HumanizeHardware("", p.Storage))
		}
		specStr := strings.Join(specs, " ｜ ")
		if specStr != "" {
			b.WriteString(fmt.Sprintf("• %s\n  └ 规格: %s\n", title, specStr))
		} else {
			b.WriteString(fmt.Sprintf("• %s\n", title))
		}
	}

	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString(fmt.Sprintf("👇 点选下方机型进入机房配置并%s：", actionText))

	prefix := "i:P:" + mode + ":"
	modelBtns := make([]map[string]string, 0, len(pagePlans))
	for _, p := range pagePlans {
		data := prefix + p.Code
		if len(data) > 64 {
			continue
		}
		label := "⚡ " + p.Code
		cleanName := p.Name
		if idx := strings.Index(cleanName, " | "); idx != -1 {
			cleanName = cleanName[:idx]
		}
		cleanName = strings.TrimSpace(cleanName)
		if cleanName != "" && !strings.EqualFold(cleanName, p.Code) {
			label = fmt.Sprintf("⚡ %s", cleanName)
			if len(label) > 28 {
				label = "⚡ " + p.Code
			}
		}
		modelBtns = append(modelBtns, telegram.CallbackButton(label, data))
	}
	rows := telegram.ChunkButtons(modelBtns, 2)

	// 分页导航按钮
	navRow := []map[string]string{}
	if page > 0 {
		navRow = append(navRow, telegram.CallbackButton("◀ 上一页", fmt.Sprintf("i:pg:%s:%s:%d", mode, category, page-1)))
	}
	if totalPages > 1 {
		navRow = append(navRow, telegram.CallbackButton(fmt.Sprintf("📄 %d/%d 页", page+1, totalPages), fmt.Sprintf("i:pg:%s:%s:%d", mode, category, page)))
	}
	if page < totalPages-1 {
		navRow = append(navRow, telegram.CallbackButton("下一页 ▶", fmt.Sprintf("i:pg:%s:%s:%d", mode, category, page+1)))
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	// 底部返回按钮
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔙 返回系列分类", "i:cat:"+mode+":root"),
	})

	markup := telegram.InlineKeyboard(rows)
	if edit && messageID > 0 {
		if telegram.EditMessage(state, chatID, messageID, b.String(), markup) {
			return
		}
	}
	_, _ = telegram.SendToChat(state, chatID, b.String(), markup)
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

func enumerateWizardConfigs(state *app.State, planCode, accountID string) []wizardConfigChoice {
	if accountID == "" {
		accountID = telegram.ActiveAccountID(state)
	}
	byConfig := catalog.CheckServerAvailabilityWithConfigs(state, planCode, accountID)
	out := make([]wizardConfigChoice, 0, len(byConfig))
	for _, d := range byConfig {
		if d == nil {
			continue
		}
		mem := strings.TrimSpace(d.Memory)
		stor := strings.TrimSpace(d.Storage)
		label := telegram.HumanizeHardware(mem, stor)
		if label == "" || label == "/" {
			continue
		}
		inStock := 0
		for _, status := range d.Datacenters {
			if catalog.IsAvailableForOrder(status) {
				inStock++
			}
		}
		out = append(out, wizardConfigChoice{
			Label:   label,
			Options: d.Options,
			InStock: inStock,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].InStock > 0) != (out[j].InStock > 0) {
			return out[i].InStock > 0
		}
		return out[i].Label < out[j].Label
	})
	return out
}

func showConfigPicker(state *app.State, mon *monitor.Monitor, chatID interface{}, messageID int64, mode, planCode string, edit bool, targetDCs []string, quantity int) {
	configs := enumerateWizardConfigs(state, planCode, "")
	if len(configs) <= 1 {
		sess := getPickerSession(chatID, messageID, mode, planCode)
		if len(configs) == 1 {
			sess.Options = append([]string{}, configs[0].Options...)
			sess.ConfigLabel = configs[0].Label
		} else {
			sess.Options = nil
			sess.ConfigLabel = ""
		}
		if len(targetDCs) > 0 && mode != "m" {
			qty := quantity
			if qty < 1 {
				qty = 1
			}
			opts := append([]string{}, sess.Options...)
			label := sess.ConfigLabel
			accID := telegram.ActiveAccountID(state)
			for i := 0; i < qty; i++ {
				enqueueWizardDCs(state, chatID, messageID, mode, planCode, targetDCs, accID, opts, label, false)
			}
			return
		}
		showDCPicker(state, chatID, messageID, mode, planCode, edit)
		return
	}

	tok := putCfgOffer(&cfgOffer{
		Mode:      mode,
		PlanCode:  planCode,
		Configs:   configs,
		TargetDCs: append([]string{}, targetDCs...),
		Quantity:  quantity,
	})
	var btns []map[string]string
	for i, c := range configs {
		if i >= 8 {
			break
		}
		stockBadge := "[🔴 缺货]"
		if c.InStock > 0 {
			stockBadge = fmt.Sprintf("[🟢 %d机房有货]", c.InStock)
		}
		btnText := tgBtnLabel(fmt.Sprintf("⚡ %s  %s", c.Label, stockBadge))
		btns = append(btns, telegram.CallbackButton(btnText, fmt.Sprintf("i:cfg:%s:%d", tok, i)))
	}
	rows := telegram.ChunkButtons(btns, 1)

	rows = append(rows, []map[string]string{
		telegram.CallbackButton(tgBtnLabel("🌐 任意硬件配置 (不限规格 · 优先抢占首选机房)"), fmt.Sprintf("i:cfg:%s:any", tok)),
	})
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔙 返回系列分类", "i:cat:"+mode+":root"),
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("🖥️ 硬件规格自选匹配 · %s\n", planCode))
	b.WriteString(telegram.CardDivider + "\n")
	if len(targetDCs) > 0 {
		up := make([]string, len(targetDCs))
		for i, d := range targetDCs {
			up[i] = telegram.DisplayDCFull(d)
		}
		b.WriteString("📍 目标机房: " + strings.Join(up, "、") + "\n")
	}
	b.WriteString(fmt.Sprintf("📊 规格选项: 共检测到 %d 种物理硬件配置方案\n", len(configs)))
	b.WriteString("🎯 下单策略: 硬件规格精准锁单匹配\n")
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("💡 说明: 抢购将按选定配置精确锁单。若对内存/硬盘无特定要求，推荐选择最下方的「🌐 任意硬件配置」，成功率最高！\n\n")
	b.WriteString("👇 请点选要锁定抢购的硬件方案：")

	markup := telegram.InlineKeyboard(rows)
	if edit && messageID > 0 {
		if telegram.EditMessage(state, chatID, messageID, b.String(), markup) {
			return
		}
	}
	_, _ = telegram.SendToChat(state, chatID, b.String(), markup)
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

	// 价格查询单选处理
	if mode == "pr" {
		btns := []map[string]string{}
		for _, dc := range targetDCs {
			btns = append(btns, telegram.CallbackButton(telegram.DisplayDC(dc), "i:D:pr:"+planCode+":"+dc))
		}
		rows := telegram.ChunkButtons(btns, 3)
		rows = append(rows, []map[string]string{
			telegram.CallbackButton("🔙 返回系列分类", "i:cat:pr:root"),
		})
		text := fmt.Sprintf("💰 型号 %s\n请选择要询价的目标机房：", planCode)
		markup := telegram.InlineKeyboard(rows)
		if edit && messageID > 0 {
			if telegram.EditMessage(state, chatID, messageID, text, markup) {
				return
			}
		}
		_, _ = telegram.SendToChat(state, chatID, text, markup)
		return
	}

	// 多选模式 (mode == "b" || mode == "q" || mode == "m")
	sess := getPickerSession(chatID, messageID, mode, planCode)

	btns := []map[string]string{}
	selectedCount := 0
	var selectedList []string
	for _, dc := range targetDCs {
		selected := sess.Selected[dc]
		dot := "🔴 "
		if stockSet[dc] {
			dot = "🟢 "
		}
		label := dot + telegram.DisplayDC(dc)
		if selected {
			label = "☑️ " + label
			selectedCount++
			selectedList = append(selectedList, telegram.DisplayDC(dc))
		}
		data := "i:D:t:" + mode + ":" + planCode + ":" + dc
		btns = append(btns, telegram.CallbackButton(label, data))
	}
	rows := telegram.ChunkButtons(btns, 3)

	// 顶部快捷控制行
	var topRow []map[string]string
	topRow = append(topRow, telegram.CallbackButton("⚡ 全选有货", "i:D:as:"+mode+":"+planCode))
	topRow = append(topRow, telegram.CallbackButton("🌐 全选所有", "i:D:aa:"+mode+":"+planCode))
	if selectedCount > 0 {
		topRow = append(topRow, telegram.CallbackButton("🔄 清空", "i:D:cl:"+mode+":"+planCode))
	}
	rows = append([][]map[string]string{topRow}, rows...)

	// 底部确认提交按钮 (当选中 >= 1 个机房时展示)
	if selectedCount > 0 {
		submitLabel := fmt.Sprintf("🚀 立即下单 (已选 %d 个机房)", selectedCount)
		if mode == "q" {
			submitLabel = fmt.Sprintf("📥 确认入队 (已选 %d 个机房)", selectedCount)
		} else if mode == "m" {
			submitLabel = fmt.Sprintf("👁 开启监控 (已选 %d 个机房)", selectedCount)
		}
		rows = append(rows, []map[string]string{
			telegram.CallbackButton(submitLabel, "i:D:ok:"+mode+":"+planCode),
		})
	}

	var navRow []map[string]string
	navRow = append(navRow, telegram.CallbackButton("⚙️ 更改配置", "i:cfgshow:"+mode+":"+planCode))
	navRow = append(navRow, telegram.CallbackButton("🔙 返回系列分类", "i:cat:"+mode+":root"))
	rows = append(rows, navRow)

	var b strings.Builder
	actionTitle := "🛒 目标机房选择"
	if mode == "q" {
		actionTitle = "📥 队列挂机机房选择"
	} else if mode == "m" {
		actionTitle = "📡 监控目标机房选择"
	}
	b.WriteString(fmt.Sprintf("%s · %s\n", actionTitle, planCode))
	b.WriteString(telegram.CardDivider + "\n")
	cfgText := "任意硬件配置 (全规格优选)"
	if sess.ConfigLabel != "" {
		cfgText = sess.ConfigLabel
	} else if len(sess.Options) > 0 {
		cfgText = telegram.HumanizeOptionCodes(sess.Options)
	}
	b.WriteString("⚙️ 锁定配置: " + cfgText + "\n")
	if len(stock) > 0 {
		up := make([]string, len(stock))
		for i, d := range stock {
			up[i] = telegram.DisplayDC(d)
		}
		b.WriteString(fmt.Sprintf("🟢 实时现货: %s 有现货！\n", strings.Join(up, "、")))
	} else {
		b.WriteString("🔴 现货状态: 当前全区缺货 (支持挂机自动抢购)\n")
	}
	b.WriteString(telegram.CardDivider + "\n")
	if selectedCount > 0 {
		b.WriteString(fmt.Sprintf("📌 已勾选 (%d 个机房): %s\n", selectedCount, strings.Join(selectedList, "、")))
		b.WriteString("👉 点击下方提交按钮立即生效！")
	} else {
		if mode == "m" {
			b.WriteString("💡 请点击机房按钮进行多选（🟢有现货 🔴缺货监控）\n支持多机房同时监听，上架即刻报警！")
		} else {
			b.WriteString("💡 请点击机房按钮进行多选（🟢有现货 🔴缺货挂机）\n可点击上方「⚡ 全选有货」或「🌐 全选所有」。")
		}
	}

	markup := telegram.InlineKeyboard(rows)
	sentID := messageID
	if edit && messageID > 0 {
		if telegram.EditMessage(state, chatID, messageID, b.String(), markup) {
			return
		}
		sentID = 0
	}
	if id, ok := telegram.SendToChat(state, chatID, b.String(), markup); ok && id != 0 {
		sentID = id
	}
	if sentID != 0 && sentID != messageID {
		migratePickerSession(chatID, messageID, sentID, mode, planCode)
	}
}
