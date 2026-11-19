package catalog

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// 返回每个配置组合的可用性 + 匹配到的 API2 options
type ConfigAvailability struct {
	Memory        string            `json:"memory"`
	Storage       string            `json:"storage"`
	SystemStorage string            `json:"systemStorage,omitempty"`
	GPU           string            `json:"gpu,omitempty"`
	Datacenters   map[string]string `json:"datacenters"`
	FQN           string            `json:"fqn"`
	Options       []string          `json:"options"`
	// CatalogError 记录拉 /order/catalog/public/eco 时的 OVH 错误。
	// 目录拉不到时 Options 会退化成空数组,调用方必须能区分"目录没拉到"和
	// "这个配置确实没有可匹配 addon",否则会把 OVH 瞬断报成"该机房无可定价配置"。
	CatalogError string `json:"catalogError,omitempty"`
	// CatalogMissing:目录拉到了,但这个 planCode 不在该账户子公司的目录里。
	// 可能是跨区型号，也可能是尚未同步到公开目录的自定义型号。
	// 此时 Options 保留库存 FQN 中的原始硬件段，须经真实购物车验证后才能下单。
	CatalogMissing bool `json:"catalogMissing,omitempty"`
	// OptionsNote:目录和 planCode 都在,但这一条 FQN 的内存/存储在该子公司目录里
	// 没有对应 addon(实测 US 35 条、EU/CA 各 18 条)。这类配置在本子公司买不到,
	// 不是"目录挂了",调用方应该照实说而不是报错。
	OptionsNote string `json:"optionsNote,omitempty"`
}

// AwaitingCatalog 库存里有这个 Eco 型号、但销售目录还没收录(典型是预上架)。
// 与预订判定使用同一 Eco 系列范围;其他系列不受影响。
func (c *ConfigAvailability) AwaitingCatalog(planCode string) bool {
	return c != nil && c.CatalogMissing && ecoStockPlanCode.MatchString(planCode)
}

// ErrPlanNotInRegion:该 planCode 在这个账户所属站点没有任何可用性记录。
// OVH 对"不属于本站点的 planCode"返回的是 HTTP 200 + 空数组(实测:欧区 24rise01-v1
// 查 api.us.ovhcloud.com、美区 24rise01-v1-us 查 eu.api.ovh.com,两边都是 []),
// 而真正的缺货是有记录、availability=unavailable(EU 8973 条里 6700 条如此)。
// 两者必须分开报,否则用户看到的都是"空",无从判断是拿错区还是没货。
var ErrPlanNotInRegion = errors.New("该 planCode 在当前站点没有可用性记录")

// ErrConfigNotMatched:plan 在,但指定的 options 组合在它的任何 FQN 里都不存在。
var ErrConfigNotMatched = errors.New("指定配置组合不存在")

// IsAvailableForOrder 判断可用性取值是否真的可以下单。判据统一在 ovh 包
// (app / telegram / catalog 三边都要用同一份,而 app 不能 import catalog),
// 这里保留同名函数只是为了不改动已有调用方。
func IsAvailableForOrder(availability string) bool {
	return ovh.IsAvailableForOrder(availability)
}

// CheckServerAvailabilityWithConfigs 返回每个配置组合的可用性 + 匹配到的 API2 options。
//   - accountID:决定用哪个账户的 OVH client 和 zone 拉 catalog。空 = 默认账户。
//     `/dedicated/server/datacenter/availabilities` 是全局接口,client 走哪个账户无所谓;
//     但 `/order/catalog/public/eco` 必须用对应 subsidiary 拉,否则跨子公司账户的 options 匹配会失败。
//   - monitor 检查 loop 没有"当前账户"概念,直接传 "",意味着只能保证默认账户 + 同 subsidiary 账户准确;
//     quick-order / Telegram 这种已知 account_id 的调用方应该传具体 ID。
func CheckServerAvailabilityWithConfigs(state *app.State, planCode string, accountID string) map[string]*ConfigAvailability {
	client, err := state.OVH.IsolatedClientFor(accountID)
	if err != nil {
		return map[string]*ConfigAvailability{}
	}

	// 子公司/站点先算出来:下面所有日志都要带上它,否则"查不到"这种话在三区之间
	// 完全没法定位到底是哪个站点在回空。
	acc, _ := state.FindAccount(accountID)
	subsidiary := SubsidiaryOfAccount(acc)
	region := ovh.SubsidiaryRegion(subsidiary)

	state.Logger.Info(fmt.Sprintf("[配置监控] 查询 %s 的所有配置组合(%s 站点 / 子公司 %s)...", planCode, region, subsidiary), "monitor")

	var availabilities []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", planCode)
	if err := client.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &availabilities); err != nil {
		state.Logger.Error(fmt.Sprintf("[配置监控] 获取配置可用性失败: %s", err.Error()), "monitor")
		return map[string]*ConfigAvailability{}
	}

	if len(availabilities) == 0 {
		// 空数组 ≠ 无货:OVH 对"不属于本站点的 planCode"就是回 200 + [](实测三区互查都如此),
		// 真正的缺货是有记录、availability=unavailable。这里必须把区域线索写进日志,
		// 否则跨区订阅(拿欧区 planCode 监控美区账户)会永远显示"取不到可用性"而查不出原因。
		state.Logger.Warn(fmt.Sprintf("[配置监控] %s 站点(子公司 %s)没有 %s 的任何可用性记录 —— "+
			"这不是无货,而是该 planCode 不属于这个站点(美区机型带 -us/-ca/-eu/-sgp 后缀,欧区裸 planCode 在美区查不到)",
			region, subsidiary, planCode), "monitor")
		return map[string]*ConfigAvailability{}
	}

	state.Logger.Info(fmt.Sprintf("[配置监控] OVH API 返回 %d 个配置组合", len(availabilities)), "monitor")

	// addon 匹配用的目录走 120 秒内存缓存(公开数据,不耗账户配额)。
	// 以前这里每次调用都用账户凭据现拉一份 ~10MB 的 eco 目录,而监控是
	// 「每订阅 × 每 5 秒」调一次 —— 几个订阅就能把 OVH 打到 429,
	// 结果是补货那一刻监控恰好最不可靠。
	addonFamilies, addonErr := AddonFamiliesForPlan(state, accountID, planCode)
	catalogErr := ""
	catalogMissing := false
	if addonErr != nil {
		catalogErr = addonErr.Error()
		// "目录里没有这个 planCode" 和 "目录拉取失败" 是两码事:前者是跨区拿错 planCode,
		// 重试无用,得换带本区后缀的机型;后者是 OVH/网络瞬断,重试就好。
		// 调用方(quick_order / telegram)据此给用户不同的提示。
		catalogMissing = errors.Is(addonErr, ErrPlanNotInCatalog)
		if catalogMissing {
			catalogErr = fmt.Sprintf("%s 子公司(%s 站点)的公开目录尚未收录 planCode %s，可能为新品、自定义或跨区型号；以该账户购物车校验结果为准",
				subsidiary, region, planCode)
			state.Logger.Warn("[配置监控] "+catalogErr, "monitor")
		} else {
			state.Logger.Error(fmt.Sprintf("[配置监控] 取 %s 的目录 addon 失败(子公司 %s / %s 站点): %s，本次无法匹配 API2 选项",
				planCode, subsidiary, region, catalogErr), "monitor")
		}
	}

	result := map[string]*ConfigAvailability{}
	for _, item := range availabilities {
		memory := getString(item, "memory", "N/A")
		storage := getString(item, "storage", "N/A")
		systemStorage := getString(item, "systemStorage", "N/A")
		gpu := getString(item, "gpu", "N/A")
		fqn := getString(item, "fqn", "")
		configKey := fqn
		if configKey == "" {
			configKey = strings.Join([]string{memory, storage, systemStorage, gpu}, "|")
		}

		datacenters := map[string]string{}
		if dcsRaw, ok := item["datacenters"].([]interface{}); ok {
			for _, dcRaw := range dcsRaw {
				dc, ok := dcRaw.(map[string]interface{})
				if !ok {
					continue
				}
				dcName := getString(dc, "datacenter", "")
				availability := getString(dc, "availability", "unknown")
				if dcName != "" {
					datacenters[dcName] = availability
				}
			}
		}

		// 匹配 API2 options（完整覆盖 memory / storage / system-storage / gpu 必选硬件家族）
		api2Options := []string{}
		memoryStd, storageStd, sysStorageStd, gpuStd := "", "", "", ""
		// 原始段(未标准化)是第一优先的匹配依据,"N/A" 视为没有该段
		rawMemory, rawStorage, rawSysStorage, rawGPU := "", "", "", ""
		if memory != "N/A" && memory != "" {
			rawMemory = memory
			memoryStd = StandardizeConfig(memory)
		}
		if storage != "N/A" && storage != "" {
			rawStorage = storage
			storageStd = StandardizeConfig(storage)
		}
		if systemStorage != "N/A" && systemStorage != "" {
			rawSysStorage = systemStorage
			sysStorageStd = StandardizeConfig(systemStorage)
		}
		if gpu != "N/A" && gpu != "" {
			rawGPU = gpu
			gpuStd = StandardizeConfig(gpu)
		}

		state.Logger.Debug(fmt.Sprintf("[配置监控] 提取选项: memory=%s (标准化: %s), storage=%s (标准化: %s), systemStorage=%s, gpu=%s",
			memory, memoryStd, storage, storageStd, rawSysStorage, rawGPU), "monitor")

		// 从缓存的 addonFamilies 里挑与本条 FQN 的内存/存储/系统盘/GPU 匹配的 addon planCode。
		// 匹配口径见 matchAddonsForSegment(三段式,绝不用 Contains)。
		wantedSegs, matchedSegs := 0, 0
		matchAddons := func(candidates []string, raw, std string) {
			if raw == "" && std == "" {
				return
			}
			wantedSegs++
			hits := matchAddonsForSegment(candidates, raw, std)
			if len(hits) == 0 {
				return
			}
			matchedSegs++
			for _, addon := range hits {
				if !contains(api2Options, addon) {
					api2Options = append(api2Options, addon)
				}
			}
		}
		optionsNote := ""
		if len(addonFamilies) > 0 {
			// 原始段优先,标准化段兜底
			matchAddons(addonFamilies["memory"], rawMemory, memoryStd)
			matchAddons(addonFamilies["storage"], rawStorage, storageStd)
			sysFam := addonFamilies["system-storage"]
			if len(sysFam) == 0 {
				sysFam = addonFamilies["systemstorage"]
			}
			matchAddons(sysFam, rawSysStorage, sysStorageStd)
			matchAddons(addonFamilies["gpu"], rawGPU, gpuStd)
			if wantedSegs > 0 && matchedSegs < wantedSegs {
				// 目录在、plan 在,就是这套硬件在本子公司目录里没有对应 addon。
				// 实测确实存在(US 35 条、EU/CA 各 18 条,例:24rise04-v1 的 softraid-4x6000sa),
				// 属于"这个配置在本子公司买不到",不能和"目录挂了"混为一谈。
				optionsNote = fmt.Sprintf("该配置(%s / %s)在 %s 子公司的目录里没有对应 addon，本子公司买不到这套配置",
					memory, storage, subsidiary)
				state.Logger.Warn("[配置监控] "+optionsNote, "monitor")
			}
		} else if catalogMissing {
			// The stock API already identified this exact hardware combination.
			// Keep those raw addon segments so a missing public catalog cannot
			// silently discard the user's hardware filter. price/purchase resolve
			// them against live eco/options and reject anything not orderable.
			api2Options = stockHardwareOptions(planCode, fqn, rawMemory, rawStorage, rawSysStorage, rawGPU)
		} else if catalogErr == "" {
			state.Logger.Warn(fmt.Sprintf("[配置监控] %s 子公司的目录里 %s 没有 addonFamilies,无法匹配选项", subsidiary, planCode), "monitor")
		}

		if len(api2Options) > 0 {
			state.Logger.Info(fmt.Sprintf("[配置监控] 成功提取 %d 个API2选项: %v", len(api2Options), api2Options), "monitor")
		}

		sysOut := ""
		if rawSysStorage != "" {
			sysOut = systemStorage
		}
		gpuOut := ""
		if rawGPU != "" {
			gpuOut = gpu
		}
		result[configKey] = &ConfigAvailability{
			Memory:         memory,
			Storage:        storage,
			SystemStorage:  sysOut,
			GPU:            gpuOut,
			Datacenters:    datacenters,
			FQN:            fqn,
			Options:        api2Options,
			CatalogError:   catalogErr,
			CatalogMissing: catalogMissing,
			OptionsNote:    optionsNote,
		}
		state.Logger.Info(fmt.Sprintf("[配置监控] 配置: %s + %s, 数据中心数: %d", memory, storage, len(datacenters)), "monitor")
	}

	state.Logger.Info(fmt.Sprintf("[配置监控] 成功获取 %d 个配置组合的可用性", len(result)), "monitor")
	return result
}

func stockHardwareOptions(planCode, fqn string, rawParts ...string) []string {
	var candidates []string
	if suffix, ok := strings.CutPrefix(fqn, planCode+"."); ok {
		candidates = strings.Split(suffix, ".")
	} else {
		candidates = rawParts
	}
	options := []string{}
	for _, code := range candidates {
		code = strings.TrimSpace(code)
		if code != "" && code != "N/A" && !contains(options, code) {
			options = append(options, code)
		}
	}
	return options
}

// CustomInventoryOptions maps a raw stock addon to the user's full addon code
// only when the suffix is exactly this planCode. A shared storage prefix is not
// sufficient evidence: a mixed-disk option must never match the smaller layout.
func CustomInventoryOptions(planCode string, hardware, selected []string) []string {
	options := append([]string(nil), hardware...)
	for i, raw := range options {
		for _, code := range selected {
			if code == raw+"-"+planCode {
				options[i] = code
				break
			}
		}
	}
	return options
}

// matchAddonsForSegment 把 availabilities 里的一段 FQN(如 softraid-2x6000sa / ram-32g-ecc-2400)
// 映射成该子公司目录里可下单的 addon planCode。三段式,顺序不能换:
//
//  1. 标准化后完全相等 —— 欧区/加区绝大多数走这条(实测 EU/CA 各 1022 条内存走这条);
//  2. addon 标准化值以「段 + -」开头 —— 美区目录的 addon 都带 -us/-ca/-eu 机房后缀
//     (ram-32g-ecc-2400-24risegame01-ca 标准化后仍是 ram-32g-ca),实测美区 1970 条内存里
//     能走第 1 条的是 0 条,全靠这一条;
//  3. addon 标准化值以「段」开头(无分隔符)—— StandardizeConfig 抹掉机型串后会粘连
//     (ram-256g-ecc-2400-24sk60b → ram-256gb、ram-128g-on-die-...-25risel01-v1-ca →
//     ram-128g-on-diel01-ca),这类只能靠裸前缀兜底。
//
// 绝对不能退回 strings.Contains:softraid-2x6000sa 会同时命中
// hybridsoftraid-2x6000sa-2x512nvme(实测 US 92 条、EU/CA 各 119 条这样误配),
// 而 Options 是 monitor / quick_order / telegram 直接拿去下单的 —— 误配就是买错配置。
func matchAddonsForSegment(addons []string, seg, segStd string) []string {
	if len(addons) == 0 || (seg == "" && segStd == "") {
		return nil
	}
	// 第 1 档:原始码完全相等
	for _, addon := range addons {
		if addon == seg {
			return []string{addon}
		}
	}
	// 第 2 档:原始码以「段-」开头。同一 plan 里存在互为前缀的存储项
	// (softraid-2x960nvme-26sk50a-v1 与 softraid-2x960nvme-2x6000sa-26sk50a-v1),
	// 剩余部分越短越可能只是机型后缀、越长越可能多编了一段别的配置 —— 取最短的那条。
	//
	// 这一档必须放在标准化之前:StandardizeConfig 会把机型后缀吃成粘连残渣
	// (…-26sk50a-v1 → …nvmea),正确项因此丢掉分隔符、反而落到比错误项更低的档,
	// 实测就是这样把 €0 的 2x960NVMe 配成了 €24 的混合盘。
	if seg != "" {
		if best := shortestWithAllowedPrefix(addons, seg+"-"); best != "" {
			return []string{best}
		}
	}
	// 第 3 档:标准化后相等。OVH 的 FQN 段和目录 addon 的内存频率经常对不上
	// (可用性报 ram-1024g-ecc-2933、目录只卖 ram-1024g-ecc-3200),只有标准化认得出是同一档。
	if segStd != "" {
		var exact []string
		for _, addon := range addons {
			if StandardizeConfig(addon) == segStd {
				exact = append(exact, addon)
			}
		}
		if len(exact) > 0 {
			return exact
		}
		// 第 4 档:标准化后带 "-" 边界且后缀仅为机型/区域标识的前缀
		best, bestLen := "", -1
		for _, addon := range addons {
			as := StandardizeConfig(addon)
			if !strings.HasPrefix(as, segStd+"-") || !IsAllowedOptionSuffix(strings.TrimPrefix(as, segStd+"-")) {
				continue
			}
			if bestLen < 0 || len(as) < bestLen {
				best, bestLen = addon, len(as)
			}
		}
		if best != "" {
			return []string{best}
		}
	}
	return nil
}

// shortestWithAllowedPrefix 返回以 prefix 开头、剩余后缀仅为机型/区域标识且整体最短的 addon(没有则空串)
func shortestWithAllowedPrefix(addons []string, prefix string) string {
	best := ""
	lowerPrefix := strings.ToLower(prefix)
	for _, addon := range addons {
		low := strings.ToLower(addon)
		if !strings.HasPrefix(low, lowerPrefix) {
			continue
		}
		if !IsAllowedOptionSuffix(strings.TrimPrefix(low, lowerPrefix)) {
			continue
		}
		if best == "" || len(addon) < len(best) {
			best = addon
		}
	}
	return best
}

// 匹配强度分档:数值只用来比大小,越大越可信。
const (
	segScoreEqual     = 4000 // 原样相等
	segScorePrefix    = 2000 // 一方是另一方的 "x-" 前缀(再加上匹配长度做细分)
	segScoreStdEqual  = 1000 // 标准化后相等(会吃掉 -ecc-2400 这种频率差异)
	segScoreStdPrefix = 500  // 标准化后前缀
	segScoreNoMatch   = -1
)

// segmentMatchScore 判断"用户传的 addon planCode"和"availabilities 里的 FQN 段"是不是同一项配置,
// 返回 -1 表示不是,否则返回匹配强度。
//
// 为什么要分档 + 取最高分,而不是"第一个匹配就 break":
//   - 同一个 plan 里存在互为前缀的存储配置(实测 US 112 对、EU/CA 各 61 对,例
//     softraid-36x22000-hdd-sas 与 softraid-36x22000-hdd-sas-2x7680nvme),先撞上短的那条
//     就会把另一套配置的库存当成用户选的那套 —— 抢到的机器和下单的配置对不上;
//   - 但标准化这一档又不能去掉:OVH 的 FQN 段和目录 addon 的内存频率经常对不上
//     (26sk10b-v1 的 FQN 段是 ram-32g-ecc-2133,目录 addon 却是 ram-32g-ecc-2400),
//     只有标准化(抹掉 -ecc-\d+)才认得出是同一档内存。
//
// 三区通用:美区的 -us/-ca/-eu 后缀落在"前缀"这一档,欧区/加区多数落在"相等/标准化相等"。
func segmentMatchScore(optionCode, fqnSeg string) int {
	opt := strings.ToLower(strings.TrimSpace(optionCode))
	seg := strings.ToLower(strings.TrimSpace(fqnSeg))
	if opt == "" || seg == "" {
		return segScoreNoMatch
	}
	if opt == seg {
		return segScoreEqual
	}
	if strings.HasPrefix(opt, seg+"-") && IsAllowedOptionSuffix(strings.TrimPrefix(opt, seg+"-")) {
		return segScorePrefix + len(seg)
	}
	if strings.HasPrefix(seg, opt+"-") && IsAllowedOptionSuffix(strings.TrimPrefix(seg, opt+"-")) {
		return segScorePrefix + len(opt)
	}
	optStd, segStd := StandardizeConfig(opt), StandardizeConfig(seg)
	if optStd != "" && optStd == segStd {
		return segScoreStdEqual
	}
	if optStd != "" && segStd != "" {
		if strings.HasPrefix(optStd, segStd+"-") && IsAllowedOptionSuffix(strings.TrimPrefix(optStd, segStd+"-")) {
			return segScoreStdPrefix
		}
		if strings.HasPrefix(segStd, optStd+"-") && IsAllowedOptionSuffix(strings.TrimPrefix(segStd, optStd+"-")) {
			return segScoreStdPrefix
		}
	}
	return segScoreNoMatch
}

// fqnRelevantOption 只有内存/存储(含系统盘)/GPU 这几类硬件 addon 会出现在 FQN 里,
// bandwidth / vrack / OS / 许可证不会。把它们一起拿去匹配 FQN 会导致任何机型都匹配不上。
func fqnRelevantOption(opt string) bool {
	o := strings.ToLower(opt)
	if strings.Contains(o, "ram-") || strings.Contains(o, "memory") || strings.Contains(o, "gpu") {
		return true
	}
	for _, kw := range []string{"raid", "disk", "nvme", "ssd", "hdd", "sas", "storage"} {
		if strings.Contains(o, kw) {
			return true
		}
	}
	return false
}

// fqnSegmentsOf 取一条 availability 记录里参与配置匹配的段。
// 用 memory/storage/systemStorage/gpu 四个字段而不是切 FQN 字符串:FQN 第一段是 planCode,
// 而美区 planCode 自带 -us/-ca 后缀,按 "." 切完还要再判断哪段是什么,反而容易错。
func fqnSegmentsOf(item map[string]interface{}) []string {
	segs := []string{}
	for _, key := range []string{"memory", "storage", "systemStorage", "gpu"} {
		if v := getString(item, key, ""); v != "" && v != "N/A" {
			segs = append(segs, v)
		}
	}
	return segs
}

// CheckServerAvailability 查某机型各机房的可用性（带 options 精确匹配）。
// accountID 决定用哪个账户的 client 查:EU / US / CA 三个 endpoint 各自维护独立的
// /dedicated/server/datacenter/availabilities 库存视图,写死默认账户会让前端切账户后
// 看到的是别的大区的库存。空 = 默认账户。
func CheckServerAvailability(state *app.State, planCode string, options []string, accountID string) (map[string]string, error) {
	client, err := state.OVH.IsolatedClientFor(accountID)
	if err != nil {
		return nil, err
	}

	state.Logger.Info(fmt.Sprintf("查询 %s 的可用性...", planCode), "")

	var availabilities []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", planCode)
	if err := client.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &availabilities); err != nil {
		state.Logger.Error(fmt.Sprintf("Failed to check availability for %s: %s", planCode, err.Error()), "")
		return nil, err
	}

	state.Logger.Info(fmt.Sprintf("OVH API 返回 %d 个配置组合", len(availabilities)), "")
	if len(availabilities) == 0 {
		// 空数组是"这个 planCode 不属于本站点",不是"全线无货"。以前直接回空 map,
		// 前端/脚本看到的和"所有机房 unavailable"一模一样,跨区拿错 planCode 永远查不出来。
		acc, _ := state.FindAccount(accountID)
		sub := SubsidiaryOfAccount(acc)
		state.Logger.Warn(fmt.Sprintf("%s 站点(子公司 %s)没有 %s 的任何可用性记录", ovh.SubsidiaryRegion(sub), sub, planCode), "")
		return nil, fmt.Errorf("%w：%s 站点(子公司 %s)查不到 %s，"+
			"三个站点的机型目录互不相通(美区机型带 -us/-ca/-eu/-sgp 后缀),请确认 planCode 与账户所在区一致",
			ErrPlanNotInRegion, ovh.SubsidiaryRegion(sub), sub, planCode)
	}

	// 用户提供配置：精确匹配
	if len(options) > 0 {
		state.Logger.Info(fmt.Sprintf("查询 %s 的配置选项可用性: %v", planCode, options), "")
		// 只挑会出现在 FQN 里的 addon(内存/存储/系统盘);带宽、vrack、OS 之类不参与匹配。
		wanted := []string{}
		for _, opt := range options {
			if fqnRelevantOption(opt) {
				wanted = append(wanted, opt)
			} else {
				state.Logger.Debug(fmt.Sprintf("选项 %s 不出现在 FQN 里，跳过配置匹配", opt), "")
			}
		}

		if len(wanted) == 0 {
			// 传进来的 option 全是带宽/vrack/OS 这类不进 FQN 的项:没法用它们挑配置组合,
			// 退回"未指定 options"的默认分支,而不是报"配置不存在"(那是误报)。
			state.Logger.Warn(fmt.Sprintf("%s 的 options %v 里没有内存/存储项，无法定位配置组合，退回默认配置", planCode, options), "")
		} else {
			var matchedConfig map[string]interface{}
			bestScore := 0
			for _, item := range availabilities {
				segs := fqnSegmentsOf(item)
				itemFQN := getString(item, "fqn", "")
				total := 0
				ok := true
				for _, opt := range wanted {
					best := segScoreNoMatch
					for _, seg := range segs {
						if sc := segmentMatchScore(opt, seg); sc > best {
							best = sc
						}
					}
					if best <= segScoreNoMatch {
						ok = false
						break
					}
					total += best
				}
				if !ok {
					continue
				}
				state.Logger.Debug(fmt.Sprintf("候选配置 %s 匹配分 %d (段: %v)", itemFQN, total, segs), "")
				// 取分最高的那条:同一个 plan 里存在互为前缀的存储配置,
				// "第一个匹配就 break" 会挑到粒度更粗的那条,返回的是别的配置的库存。
				if total > bestScore {
					bestScore = total
					matchedConfig = item
				}
			}

			if matchedConfig != nil {
				state.Logger.Info(fmt.Sprintf("✅ 找到匹配配置: %s (匹配分 %d)", getString(matchedConfig, "fqn", ""), bestScore), "")
				result := map[string]string{}
				if dcsRaw, ok := matchedConfig["datacenters"].([]interface{}); ok {
					for _, dcRaw := range dcsRaw {
						dc, ok := dcRaw.(map[string]interface{})
						if !ok {
							continue
						}
						dcName := getString(dc, "datacenter", "")
						availability := getString(dc, "availability", "unknown")
						if dcName == "" {
							continue
						}
						if availability == "" || availability == "unknown" {
							result[dcName] = "unknown"
						} else if availability == "unavailable" {
							result[dcName] = "unavailable"
						} else {
							result[dcName] = availability
						}
					}
				}
				state.Logger.Info(fmt.Sprintf("配置 %s 的可用性: %v", getString(matchedConfig, "fqn", ""), result), "")
				return result, nil
			}

			// 没匹配上要报错而不是回空 map:空 map 会被前端画成"所有机房都没货",
			// 而实际情况是这套配置在本区根本不存在(常见于跨区套用别的区的 addon planCode)。
			fqns := []string{}
			for _, item := range availabilities {
				fqns = append(fqns, getString(item, "fqn", ""))
			}
			acc, _ := state.FindAccount(accountID)
			sub := SubsidiaryOfAccount(acc)
			state.Logger.Warn(fmt.Sprintf("❌ 未找到匹配的配置组合！请求: %v，该 plan 现有组合: %v", options, fqns), "")
			return nil, fmt.Errorf("%w：%s 站点(子公司 %s)的 %s 没有这套配置(%v)，现有组合 %d 种",
				ErrConfigNotMatched, ovh.SubsidiaryRegion(sub), sub, planCode, wanted, len(fqns))
		}
	}

	// 未指定 options：使用第一个
	defaultConfig := availabilities[0]
	defaultFQN := getString(defaultConfig, "fqn", "")
	state.Logger.Info(fmt.Sprintf("使用默认配置: %s", defaultFQN), "")

	result := map[string]string{}
	if dcsRaw, ok := defaultConfig["datacenters"].([]interface{}); ok {
		for _, dcRaw := range dcsRaw {
			dc, ok := dcRaw.(map[string]interface{})
			if !ok {
				continue
			}
			dcName := getString(dc, "datacenter", "")
			availability := getString(dc, "availability", "unknown")
			if dcName == "" {
				continue
			}
			if availability == "" || availability == "unknown" {
				result[dcName] = "unknown"
			} else if availability == "unavailable" {
				result[dcName] = "unavailable"
			} else {
				result[dcName] = availability
			}
		}
	}
	state.Logger.Info(fmt.Sprintf("默认配置 %s 的可用性: %v", defaultFQN, result), "")
	return result, nil
}

// PassthroughAvailability 用 SDK 直接传请求（用于 sniper 监控）
func PassthroughAvailability(client *ovhsdk.Client, planCode string) ([]map[string]interface{}, error) {
	var out []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", planCode)
	err := client.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &out)
	return out, err
}
