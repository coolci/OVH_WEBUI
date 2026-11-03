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

type topPickModel struct {
	Code      string
	ShortName string
	Badge     string // "🟢", "📋", "🔥", "🕘"
	InStock   bool
	Monitored bool
}

func getTopPickModels(state *app.State, mon *monitor.Monitor) []topPickModel {
	state.ServerPlansMu.RLock()
	allPlans := append([]types.ServerPlan{}, state.ServerPlans...)
	state.ServerPlansMu.RUnlock()

	planMap := make(map[string]types.ServerPlan, len(allPlans))
	for _, p := range allPlans {
		planMap[p.PlanCode] = p
	}

	formatShortName := func(code string, fullName string) string {
		clean := fullName
		if idx := strings.Index(clean, " | "); idx != -1 {
			clean = clean[:idx]
		}
		clean = strings.TrimSpace(clean)
		if clean == "" || strings.EqualFold(clean, code) {
			return ""
		}
		if len(clean) > 8 {
			clean = clean[:8]
		}
		return clean
	}

	inStockMap := make(map[string]bool)
	for _, s := range allPlans {
		for _, dc := range s.Datacenters {
			if catalog.IsAvailableForOrder(dc.Availability) || dc.Availability == "available" {
				inStockMap[s.PlanCode] = true
				break
			}
		}
	}
	if mon != nil {
		for _, sub := range mon.Snapshot() {
			if sub != nil && sub.PlanCode != "" {
				for _, st := range sub.LastStatus {
					if catalog.IsAvailableForOrder(st) || st == "available" {
						inStockMap[sub.PlanCode] = true
						break
					}
				}
			}
		}
	}

	monitoredMap := make(map[string]bool)
	if mon != nil {
		for _, sub := range mon.Snapshot() {
			if sub != nil && sub.PlanCode != "" {
				monitoredMap[sub.PlanCode] = true
			}
		}
	}
	if state != nil {
		state.QueueMu.Lock()
		for _, q := range state.Queue {
			if q.PlanCode != "" {
				monitoredMap[q.PlanCode] = true
			}
		}
		state.QueueMu.Unlock()
	}

	benchmarkCodes := []string{
		"24ska01", "24sk20", "24sys01", "24rise01",
		"ks-1", "ks-5", "sys-1", "rise-1",
		"24sk50", "24sk50-v1", "24sk60", "24ska02", "24ska03",
		"24game01", "24adv01",
	}

	var result []topPickModel
	seen := make(map[string]bool)

	addModel := func(code string, badge string) {
		if seen[code] || len(result) >= 6 {
			return
		}
		seen[code] = true
		var name string
		if p, ok := planMap[code]; ok {
			name = formatShortName(code, p.Name)
		}
		result = append(result, topPickModel{
			Code:      code,
			ShortName: name,
			Badge:     badge,
			InStock:   inStockMap[code],
			Monitored: monitoredMap[code],
		})
	}

	// Priority 1: 实时现货机型
	for code := range inStockMap {
		addModel(code, "🟢")
		if len(result) >= 3 {
			break
		}
	}

	// Priority 2: 关注或排队中机型
	for code := range monitoredMap {
		addModel(code, "📋")
		if len(result) >= 4 {
			break
		}
	}

	// Priority 3: 知名常青热门神机
	for _, code := range benchmarkCodes {
		if _, exists := planMap[code]; exists {
			badge := "🔥"
			if inStockMap[code] {
				badge = "🟢"
			} else if monitoredMap[code] {
				badge = "📋"
			}
			addModel(code, badge)
		}
		if len(result) >= 6 {
			break
		}
	}

	// Priority 4: 兜底机型库
	if len(result) < 6 {
		for _, p := range allPlans {
			badge := "🕘"
			if inStockMap[p.PlanCode] {
				badge = "🟢"
			} else if monitoredMap[p.PlanCode] {
				badge = "📋"
			}
			addModel(p.PlanCode, badge)
			if len(result) >= 6 {
				break
			}
		}
	}

	// 空库兜底
	if len(result) == 0 {
		fallbacks := []string{"24ska01", "24sk20", "24sys01", "24rise01", "ks-1", "sys-1"}
		for _, fb := range fallbacks {
			addModel(fb, "🔥")
		}
	}

	return result
}

func renderCategoryPicker(state *app.State, mon *monitor.Monitor, chatID interface{}, messageID int64, mode string, edit bool) {
	picks := getTopPickModels(state, mon)

	inStockCount := 0
	monitoredCount := 0
	if mon != nil {
		monitoredCount = len(mon.Snapshot())
	}
	for _, m := range picks {
		if m.InStock {
			inStockCount++
		}
	}

	accTag := "默认账户"
	if state != nil {
		if acc, ok := telegram.ActiveAccount(state); ok && acc.Name != "" {
			accTag = acc.Name
			if acc.Zone != "" {
				accTag += " (" + strings.ToUpper(acc.Zone) + ")"
			}
		}
	}
	cpTag := ""
	if state != nil {
		if cp := telegram.ActiveCoupon(state); cp != "" {
			cpTag = " · 🎫 " + cp
		}
	}

	actionTitle := "⚡ 快速下单"
	switch mode {
	case "q":
		actionTitle = "📥 抢购排队"
	case "s":
		actionTitle = "📦 现货库存查询"
	case "m":
		actionTitle = "📡 添加库存监控"
	case "pr":
		actionTitle = "💰 价格查询"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s · 👤 %s%s\n", actionTitle, accTag, cpTag))
	b.WriteString(fmt.Sprintf("🟢 现货 %d 款 · 📋 关注 %d 款\n", inStockCount, monitoredCount))
	b.WriteString("💡 点击下方机型直达，或直接发送型号 (如 ks5 / 24ska01)")

	modelBtns := make([]map[string]string, 0, len(picks))
	for _, p := range picks {
		label := fmt.Sprintf("%s %s", p.Badge, p.Code)
		if p.ShortName != "" {
			label = fmt.Sprintf("%s %s %s", p.Badge, p.Code, p.ShortName)
		}
		data := fmt.Sprintf("i:P:%s:%s", mode, p.Code)
		modelBtns = append(modelBtns, telegram.CallbackButton(label, data))
	}
	rows := telegram.ChunkButtons(modelBtns, 2)

	totalPlansCount := 0
	if state != nil {
		state.ServerPlansMu.RLock()
		totalPlansCount = len(state.ServerPlans)
		state.ServerPlansMu.RUnlock()
	}

	allLabel := "🔍 全部型号"
	if totalPlansCount > 0 {
		allLabel = fmt.Sprintf("🔍 全部型号 (%d款)", totalPlansCount)
	}

	rows = append(rows, []map[string]string{
		telegram.CallbackButton(allLabel, "i:cat:"+mode+":all"),
		telegram.CallbackButton("🔙 返回控制中心", "i:dash:refresh"),
	})

	markup := telegram.InlineKeyboard(rows)
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
		text := fmt.Sprintf("%s 暂无可用机型。\n\n💡 建议选择「📥 抢购排队」或直接输入型号抢购。", catTitle)
		markup := telegram.InlineKeyboard([][]map[string]string{
			{telegram.CallbackButton("🔙 返回精选机型", "i:cat:"+mode+":root")},
		})
		if edit && messageID > 0 {
			_ = telegram.EditMessage(state, chatID, messageID, text, markup)
			return
		}
		_, _ = telegram.SendToChat(state, chatID, text, markup)
		return
	}

	pageSize := 8
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

	var b strings.Builder
	b.WriteString(fmt.Sprintf("🌐 全部型号列表 · 第 %d / %d 页 (共 %d 款)\n", page+1, totalPages, len(plans)))
	b.WriteString("👇 请点击目标机型进入机房配置：")

	prefix := "i:P:" + mode + ":"
	modelBtns := make([]map[string]string, 0, len(pagePlans))
	for _, p := range pagePlans {
		label := "⚡ " + p.Code
		cleanName := p.Name
		if idx := strings.Index(cleanName, " | "); idx != -1 {
			cleanName = cleanName[:idx]
		}
		cleanName = strings.TrimSpace(cleanName)
		if cleanName != "" && !strings.EqualFold(cleanName, p.Code) {
			if len(cleanName) > 10 {
				cleanName = cleanName[:10]
			}
			label = fmt.Sprintf("⚡ %s %s", p.Code, cleanName)
		}
		modelBtns = append(modelBtns, telegram.CallbackButton(label, prefix+p.Code))
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
		telegram.CallbackButton("🔙 返回精选机型", "i:cat:"+mode+":root"),
		telegram.CallbackButton("🔙 返回控制中心", "i:dash:refresh"),
	})

	markup := telegram.InlineKeyboard(rows)
	if edit && messageID > 0 {
		if telegram.EditMessage(state, chatID, messageID, b.String(), markup) {
			return
		}
	}
	_, _ = telegram.SendToChat(state, chatID, b.String(), markup)
}

// findMatchingPlans 模糊匹配机型代码与名称
func findMatchingPlans(state *app.State, raw string) (canonical string, matches []types.ServerPlan) {
	rawClean := strings.TrimSpace(strings.ToLower(raw))
	if rawClean == "" {
		return "", nil
	}
	rawNorm := strings.ReplaceAll(strings.ReplaceAll(rawClean, "-", ""), " ", "")

	if state == nil {
		return "", nil
	}
	state.ServerPlansMu.RLock()
	plans := append([]types.ServerPlan{}, state.ServerPlans...)
	state.ServerPlansMu.RUnlock()

	for _, p := range plans {
		pCodeClean := strings.ToLower(p.PlanCode)
		pCodeNorm := strings.ReplaceAll(strings.ReplaceAll(pCodeClean, "-", ""), " ", "")
		pNameClean := strings.ToLower(p.Name)
		pNameNorm := strings.ReplaceAll(strings.ReplaceAll(pNameClean, "-", ""), " ", "")

		if pCodeClean == rawClean || pCodeNorm == rawNorm {
			return p.PlanCode, []types.ServerPlan{p}
		}
		if pNameClean == rawClean || pNameNorm == rawNorm {
			return p.PlanCode, []types.ServerPlan{p}
		}
		if strings.Contains(pCodeNorm, rawNorm) || strings.Contains(pNameNorm, rawNorm) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0].PlanCode, matches
	}
	return "", matches
}

func renderMatchedPlansPicker(state *app.State, mon *monitor.Monitor, chatID interface{}, messageID int64, mode, query string, matches []types.ServerPlan) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🔍 关键词「%s」匹配到 %d 款机型\n", query, len(matches)))
	b.WriteString("👇 请点击目标机型进入机房下单：")

	limit := 6
	if len(matches) < limit {
		limit = len(matches)
	}

	modelBtns := make([]map[string]string, 0, limit)
	for i := 0; i < limit; i++ {
		p := matches[i]
		cleanName := p.Name
		if idx := strings.Index(cleanName, " | "); idx != -1 {
			cleanName = cleanName[:idx]
		}
		cleanName = strings.TrimSpace(cleanName)
		label := "⚡ " + p.PlanCode
		if cleanName != "" && !strings.EqualFold(cleanName, p.PlanCode) {
			if len(cleanName) > 10 {
				cleanName = cleanName[:10]
			}
			label = fmt.Sprintf("⚡ %s %s", p.PlanCode, cleanName)
		}
		data := fmt.Sprintf("i:P:%s:%s", mode, p.PlanCode)
		modelBtns = append(modelBtns, telegram.CallbackButton(label, data))
	}
	rows := telegram.ChunkButtons(modelBtns, 2)
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔙 返回精选机型", "i:cat:"+mode+":root"),
		telegram.CallbackButton("🔙 返回控制中心", "i:dash:refresh"),
	})

	markup := telegram.InlineKeyboard(rows)
	if messageID > 0 {
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
	btns = append(btns, telegram.CallbackButton(tgBtnLabel("🌐 任意硬件配置 (推荐 · 优先抢占首选机房)"), fmt.Sprintf("i:cfg:%s:any", tok)))

	for i, c := range configs {
		if i >= 6 {
			break
		}
		stockBadge := "[🔴 缺货]"
		if c.InStock > 0 {
			stockBadge = fmt.Sprintf("[🟢 %d机房有货]", c.InStock)
		}
		btnText := tgBtnLabel(fmt.Sprintf("⚙ %s %s", c.Label, stockBadge))
		btns = append(btns, telegram.CallbackButton(btnText, fmt.Sprintf("i:cfg:%s:%d", tok, i)))
	}
	rows := telegram.ChunkButtons(btns, 1)
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔙 返回机房选择", "i:P:"+mode+":"+planCode),
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("⚙️ 硬件规格自选 · %s\n", planCode))
	b.WriteString(fmt.Sprintf("检测到 %d 种硬件配置方案，请点选要锁定的配置：", len(configs)))

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
			dot := "🔴 "
			if stockSet[dc] {
				dot = "🟢 "
			}
			btns = append(btns, telegram.CallbackButton(dot+telegram.DisplayDC(dc), "i:D:pr:"+planCode+":"+dc))
		}
		rows := telegram.ChunkButtons(btns, 3)
		rows = append(rows, []map[string]string{
			telegram.CallbackButton("🔙 换个机型", "i:cat:pr:root"),
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

	// 智能自动预选：如果当前未勾选任何机房，且有货机房 > 0，自动预选所有现货机房！
	if len(sess.Selected) == 0 && len(stock) > 0 {
		for _, d := range stock {
			sess.Selected[d] = true
		}
	}

	// 检测硬件配置
	configs := enumerateWizardConfigs(state, planCode, accountID)
	if len(configs) == 1 && sess.ConfigLabel == "" {
		sess.Options = append([]string{}, configs[0].Options...)
		sess.ConfigLabel = configs[0].Label
	}

	var rows [][]map[string]string

	// 如果有多种配置，在机房上方显示配置选择下拉按钮
	if len(configs) > 1 {
		cfgBtnLabel := "⚙ 配置: 任意 ▾"
		if sess.ConfigLabel != "" {
			cfgBtnLabel = "⚙ 配置: " + sess.ConfigLabel + " ▾"
		}
		rows = append(rows, []map[string]string{
			telegram.CallbackButton(tgBtnLabel(cfgBtnLabel), fmt.Sprintf("i:cfgshow:%s:%s", mode, planCode)),
		})
	}

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
	rows = append(rows, telegram.ChunkButtons(btns, 3)...)

	// 提交操作行
	if selectedCount > 0 {
		submitLabel := fmt.Sprintf("🚀 立即下单 (已选 %d 机房)", selectedCount)
		if mode == "q" {
			submitLabel = fmt.Sprintf("📥 确认入队 (已选 %d 机房)", selectedCount)
		} else if mode == "m" {
			submitLabel = fmt.Sprintf("👁 开启监控 (已选 %d 机房)", selectedCount)
		}
		rows = append(rows, []map[string]string{
			telegram.CallbackButton(submitLabel, "i:D:ok:"+mode+":"+planCode),
		})
	}

	// 快速挂机与操作行
	actionRow := []map[string]string{
		telegram.CallbackButton("🌐 全部机房挂机", "i:D:allq:"+mode+":"+planCode),
	}
	if selectedCount > 0 {
		actionRow = append(actionRow, telegram.CallbackButton("🔄 清空", "i:D:cl:"+mode+":"+planCode))
	} else if len(stock) > 0 {
		actionRow = append(actionRow, telegram.CallbackButton("⚡ 全选有货", "i:D:as:"+mode+":"+planCode))
	}
	rows = append(rows, actionRow)

	// 底部返回
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🔙 换个机型", "i:cat:"+mode+":root"),
	})

	// 消息卡片正文 (严格控制在 3 行)
	cleanName := planCode
	if state != nil {
		state.ServerPlansMu.RLock()
		for _, s := range state.ServerPlans {
			if s.PlanCode == planCode && s.Name != "" {
				cleanName = s.Name
				if idx := strings.Index(cleanName, " | "); idx != -1 {
					cleanName = cleanName[:idx]
				}
				break
			}
		}
		state.ServerPlansMu.RUnlock()
	}

	cfgText := "任意配置"
	if sess.ConfigLabel != "" {
		cfgText = sess.ConfigLabel
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("🚀 %s · %s\n", planCode, cleanName))
	if selectedCount > 0 {
		autoNote := ""
		if len(stock) > 0 && selectedCount == len(stock) {
			autoNote = " (有货机房已自动勾选)"
		}
		b.WriteString(fmt.Sprintf("⚙ %s · 已选 %s%s\n", cfgText, strings.Join(selectedList, " "), autoNote))
	} else {
		if len(stock) > 0 {
			b.WriteString(fmt.Sprintf("⚙ %s · 🟢 %d 个机房有现货 (点击勾选)\n", cfgText, len(stock)))
		} else {
			b.WriteString(fmt.Sprintf("⚙ %s · 🔴 当前全区缺货 (建议全机房挂机)\n", cfgText))
		}
	}
	b.WriteString("👇 点击机房切换勾选，点击下方按钮立即提交")

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
