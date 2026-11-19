package handlers

import (
	"sort"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/types"
)

type wizardPlan struct {
	Code    string
	Name    string
	CPU     string
	Memory  string
	Storage string
}

func wizardPlansSnapshot(state *app.State) []types.ServerPlan {
	if state == nil {
		return nil
	}
	accID := tgEffectiveAccountID(state)
	if accID != "" && state.ServerCache != nil {
		if cached, _ := state.ServerCache.GetBucket(state.ServerCacheKey(accID)); len(cached) > 0 {
			out := make([]types.ServerPlan, len(cached))
			copy(out, cached)
			return out
		}
	}
	state.ServerPlansMu.RLock()
	out := append([]types.ServerPlan{}, state.ServerPlans...)
	state.ServerPlansMu.RUnlock()
	return out
}

func getPlansForCategory(state *app.State, mon *monitor.Monitor, category string) (string, []wizardPlan) {
	allPlans := wizardPlansSnapshot(state)

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
	allPlans := wizardPlansSnapshot(state)

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
		clean = truncate(clean, 8)
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
