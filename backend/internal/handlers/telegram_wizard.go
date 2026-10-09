package handlers

import (
	"encoding/json"
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

type dcPickerSession struct {
	PlanCode    string
	Mode        string
	Selected    map[string]bool
	Options     []string
	ConfigLabel string
	UpdatedAt   time.Time
}

var (
	shortMu        sync.Mutex
	shortToID      = map[string]string{}
	pickerMu       sync.Mutex
	pickerSessions = map[string]*dcPickerSession{}
	narrowMu       sync.Mutex
	narrowOffers   = map[string]*narrowOffer{}
	cfgMu          sync.Mutex
	cfgOffers      = map[string]*cfgOffer{}
)

const (
	narrowTTL        = 10 * time.Minute
	narrowMaxButtons = 8
)

// narrowOffer 快捷 /monitor 建完后「一键改窄配置」的待选列表。
// 必须冻住按钮对应的配置:再 enumerate 一次顺序可能变,点下去会对上另一套。
type narrowOffer struct {
	PlanCode string
	Configs  []wizardConfigChoice
	Expires  time.Time
}

// cfgOffer 冻住配置选择器的选项。点按钮时不能再 enumerate:
// 库存状态一变,序号就会对上另一套,看起来就是「选不中指定配置」。
// callback_data 只放 token+序号,避开 Telegram 64 字节上限,也避开 planCode 里的冒号。
type cfgOffer struct {
	Mode      string
	PlanCode  string
	Configs   []wizardConfigChoice
	TargetDCs []string
	Quantity  int
	Expires   time.Time
}

// tgBtnLabel Telegram 按钮文字上限 64 个字符,超了 API 不报错,按钮发出去点了没反应。
func tgBtnLabel(s string) string {
	r := []rune(s)
	if len(r) <= 64 {
		return s
	}
	return string(r[:61]) + "..."
}

func pickerKey(chatID interface{}, messageID int64) string {
	return fmt.Sprintf("%v_%d", chatID, messageID)
}

func getPickerSession(chatID interface{}, messageID int64, mode, planCode string) *dcPickerSession {
	key := pickerKey(chatID, messageID)
	pickerMu.Lock()
	defer pickerMu.Unlock()

	// 清理 30 分钟未更新的废弃会话，防止内存泄漏
	if len(pickerSessions) > 100 {
		cutoff := time.Now().Add(-30 * time.Minute)
		for k, s := range pickerSessions {
			if s.UpdatedAt.Before(cutoff) {
				delete(pickerSessions, k)
			}
		}
	}

	sess, ok := pickerSessions[key]
	if !ok || sess.PlanCode != planCode || sess.Mode != mode {
		sess = &dcPickerSession{
			PlanCode:  planCode,
			Mode:      mode,
			Selected:  map[string]bool{},
			UpdatedAt: time.Now(),
		}
		pickerSessions[key] = sess
	}
	sess.UpdatedAt = time.Now()
	return sess
}

func clearPickerSession(chatID interface{}, messageID int64) {
	key := pickerKey(chatID, messageID)
	pickerMu.Lock()
	delete(pickerSessions, key)
	pickerMu.Unlock()
}

const maxShortMemoryItems = 5000

func rememberShort(state *app.State, full string, category ...string) string {
	full = strings.TrimSpace(full)
	if full == "" {
		return ""
	}
	cat := ""
	if len(category) > 0 {
		cat = category[0]
	}

	// 优先在 SQLite 检查是否已有映射，避免重复生成不同 short ID
	if state != nil && state.DB != nil {
		if existingShort, ok, _ := state.DB.FindShortIDByFull(full); ok && existingShort != "" {
			shortMu.Lock()
			shortToID[existingShort] = full
			shortMu.Unlock()
			return existingShort
		}
	}

	cleaned := strings.ReplaceAll(full, "-", "")
	// 基础长度 8 位，若冲突则逐步扩展至 10、12 位
	targetLen := 8
	if targetLen > len(cleaned) {
		targetLen = len(cleaned)
	}
	s := cleaned[:targetLen]

	shortMu.Lock()
	// 防哈希碰撞自增长度
	for {
		existing, exists := shortToID[s]
		if !exists || existing == full {
			break
		}
		if targetLen < len(cleaned) {
			targetLen += 2
			if targetLen > len(cleaned) {
				targetLen = len(cleaned)
			}
			s = cleaned[:targetLen]
		} else {
			break
		}
	}

	// 限制内存缓存上限，防止无界增长
	if len(shortToID) > maxShortMemoryItems {
		count := 0
		for k := range shortToID {
			delete(shortToID, k)
			count++
			if count >= maxShortMemoryItems/5 {
				break
			}
		}
	}
	shortToID[s] = full
	shortMu.Unlock()

	// 持久化到 SQLite
	if state != nil && state.DB != nil {
		_ = state.DB.UpsertShortID(s, full, cat)
	}
	return s
}

func resolveShort(state *app.State, s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// L1: 内存快速查询
	shortMu.Lock()
	if v, ok := shortToID[s]; ok && v != "" {
		shortMu.Unlock()
		return v
	}
	shortMu.Unlock()

	// L2: SQLite 持久化查询（跨进程重启后仍有效）
	if state != nil && state.DB != nil {
		if full, ok, err := state.DB.GetShortID(s); err == nil && ok && full != "" {
			shortMu.Lock()
			shortToID[s] = full
			shortMu.Unlock()
			return full
		}
	}

	// L3: 智能业务前缀与全量匹配兜底
	// 3.1 账户 ID 前缀匹配
	if state != nil {
		state.AccountsMu.RLock()
		for _, a := range state.Accounts {
			cleanAcc := strings.ReplaceAll(a.ID, "-", "")
			cleanS := strings.ReplaceAll(s, "-", "")
			if a.ID == s || strings.HasPrefix(a.ID, s) || strings.HasPrefix(cleanAcc, cleanS) {
				state.AccountsMu.RUnlock()
				shortMu.Lock()
				shortToID[s] = a.ID
				shortMu.Unlock()
				return a.ID
			}
		}
		state.AccountsMu.RUnlock()
	}

	// 3.2 队列任务 ID 前缀匹配
	if state != nil {
		state.QueueMu.Lock()
		for _, it := range state.Queue {
			cleanTask := strings.ReplaceAll(it.ID, "-", "")
			cleanS := strings.ReplaceAll(s, "-", "")
			if it.ID == s || strings.HasPrefix(it.ID, s) || strings.HasPrefix(cleanTask, cleanS) {
				state.QueueMu.Unlock()
				shortMu.Lock()
				shortToID[s] = it.ID
				shortMu.Unlock()
				return it.ID
			}
		}
		state.QueueMu.Unlock()
	}

	return s
}

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

type wizardConfigChoice struct {
	Label   string
	Options []string
	InStock int
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

func putNarrowOffer(planCode string, configs []wizardConfigChoice) string {
	tok := fmt.Sprintf("%x", time.Now().UnixNano())
	narrowMu.Lock()
	now := time.Now()
	for k, v := range narrowOffers {
		if now.After(v.Expires) {
			delete(narrowOffers, k)
		}
	}
	narrowOffers[tok] = &narrowOffer{
		PlanCode: planCode,
		Configs:  configs,
		Expires:  now.Add(narrowTTL),
	}
	narrowMu.Unlock()
	return tok
}

func takeNarrowOffer(tok string) (*narrowOffer, bool) {
	narrowMu.Lock()
	defer narrowMu.Unlock()
	o, ok := narrowOffers[tok]
	if !ok || time.Now().After(o.Expires) {
		delete(narrowOffers, tok)
		return nil, false
	}
	delete(narrowOffers, tok)
	return o, true
}

// offerNarrowConfig 快捷式 /monitor 建完订阅后,挂一排按钮让用户一键改窄配置。
//
// 返回 false = 没什么可挑的(只有一套配置),调用方不用额外说什么。
// 故意放在订阅**已经建好之后**:快捷式的价值就是一条命令立刻开始盯,
// 不能因为多了个选择步骤把它变成又一个向导。
func offerNarrowConfig(state *app.State, chatID interface{}, _ int64, planCode string) bool {
	configs := enumerateWizardConfigs(state, planCode, "")
	if len(configs) <= 1 {
		return false
	}
	tok := putNarrowOffer(planCode, configs)
	var btns []map[string]string
	for i, c := range configs {
		if i >= narrowMaxButtons {
			break
		}
		stockBadge := "[🔴 缺货]"
		if c.InStock > 0 {
			stockBadge = fmt.Sprintf("[🟢 %d机房有货]", c.InStock)
		}
		btnLabel := tgBtnLabel(fmt.Sprintf("⚡ %s  %s", c.Label, stockBadge))
		btns = append(btns, telegram.CallbackButton(btnLabel, fmt.Sprintf("i:mon:n:%s:%d", tok, i)))
	}
	rows := telegram.ChunkButtons(btns, 1)
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🌐 保持监控全部硬件配置", fmt.Sprintf("i:mon:n:%s:all", tok)),
	})
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🔧 规格细化 · %s\n", planCode))
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString(fmt.Sprintf("检测到该型号存在 %d 套物理硬件规格，当前默认监控全部配置。\n", len(configs)))
	b.WriteString("若您只想锁定特定硬件配置（避免其他配置放货时误触发），请点击下方直接锁定：")
	text := b.String()
	_, _ = telegram.SendToChat(state, chatID, text, telegram.InlineKeyboard(rows))
	return true
}

func putCfgOffer(o *cfgOffer) string {
	tok := fmt.Sprintf("%x", time.Now().UnixNano())
	o.Expires = time.Now().Add(narrowTTL)
	cfgMu.Lock()
	now := time.Now()
	for k, v := range cfgOffers {
		if now.After(v.Expires) {
			delete(cfgOffers, k)
		}
	}
	cfgOffers[tok] = o
	cfgMu.Unlock()
	return tok
}

func getCfgOffer(tok string) (*cfgOffer, bool) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	o, ok := cfgOffers[tok]
	if !ok || time.Now().After(o.Expires) {
		delete(cfgOffers, tok)
		return nil, false
	}
	return o, true
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
	// /buy 型号 时 messageID=0,会话写在 chat_0 上;点机房时是新消息 ID,必须把 Options 拷过去。
	if sentID != 0 && sentID != messageID {
		migratePickerSession(chatID, messageID, sentID, mode, planCode)
	}
}

func migratePickerSession(chatID interface{}, fromID, toID int64, mode, planCode string) {
	if toID == 0 || fromID == toID {
		return
	}
	from, to := pickerKey(chatID, fromID), pickerKey(chatID, toID)
	pickerMu.Lock()
	defer pickerMu.Unlock()
	sess, ok := pickerSessions[from]
	if !ok || sess.PlanCode != planCode || sess.Mode != mode {
		return
	}
	pickerSessions[to] = sess
	if fromID == 0 {
		delete(pickerSessions, from)
	}
}

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
			// 价格查询单选: i:D:pr:<plan>:<dc>
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
			// 兼容旧单选: i:D:<mode>:<plan>:<dc>
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

		// 单项取消: i:T:one:<shortID>
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
				// 收集与该消息关联的其余仍在排队中的任务
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

		// 本批次全部取消: i:T:m:<messageID>
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

		// 兼容老格式 i:T:all (仅取消当前会话/消息关联的任务，坚决不再清空全局 state.Queue!)
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

		// 兼容单项取消老格式: i:T:<shortID>
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

	if buttonID != "" && state.DB != nil {
		row, ok, err := state.DB.ClaimTelegramButton(buttonID)
		if err != nil {
			state.Logger.Error("认领一键下单按钮失败: "+err.Error(), "telegram")
			telegram.AnswerCallback(state, cbID, "认领按钮失败", true)
			return
		}
		if ok {
			if time.Since(time.Unix(int64(row.CreatedAt), 0)) > telegram.ButtonTTL {
				_ = state.DB.UnclaimTelegramButton(buttonID)
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
		} else if _, exists, _ := state.DB.GetTelegramButton(buttonID); exists {
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
		_ = state.DB.UnclaimTelegramButton(buttonID)
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

