package catalog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ovh-webui/server/internal/app"
)

// ResolvePreorderOptions 把预订任务保存的硬件段拆成两份(移植上游 4f1c4e6 / 1205120):
//
//   - stock:库存匹配条件。内存、存储、GPU 必须与库存 FQN 段逐字相同 ——
//     型号、内存容量与频率、完整硬盘组合都不允许替换。CPU 不参与匹配
//     (预售阶段 CPU 常未公布,而 CPU 本来就随型号走)。
//   - order:下单时要加的 addon planCode。目录已收录时映射成目录里的 addon,
//     并补上必选 family 的默认项;目录尚未收录(或拉不到)时原样保留,
//     由真实下单接口决定是否开放。
//
// 不认识的硬件段一律保留为匹配条件,绝不静默丢弃。
func ResolvePreorderOptions(state *app.State, accountID, planCode string, wanted []string) (stock, order []string, err error) {
	var pc planConfig
	account, ok := state.FindAccount(accountID)
	if !ok {
		return nil, nil, fmt.Errorf("下单账户不存在")
	}
	if cat, loadErr := loadSubsidiaryCatalogStale(state, SubsidiaryOfAccount(account), PreorderCheckInterval); loadErr == nil && cat != nil {
		pc = cat.plans[planCode]
	}
	selected := map[string]bool{}
	for _, option := range OrderOptions(wanted) {
		if preorderCPUOption(option) {
			continue
		}
		rawWanted := preorderProductCode(option, planCode)
		code, family, product := "", "", rawWanted
		for f, codes := range pc.addonFamilies {
			for _, candidate := range codes {
				raw := pc.addonProducts[candidate]
				if strings.EqualFold(candidate, option) || (strings.EqualFold(candidate, rawWanted+"-"+planCode) && (raw == "" || strings.EqualFold(raw, rawWanted))) {
					code, family = candidate, f
					if raw != "" {
						product = raw
					}
					break
				}
			}
			if code != "" {
				break
			}
		}
		if code == "" {
			// 只接受产品规格完全相同的别名;频率不同、多一块盘都不算同一配置。
			for f, codes := range pc.addonFamilies {
				for _, candidate := range codes {
					if raw := pc.addonProducts[candidate]; raw != "" && strings.EqualFold(raw, rawWanted) {
						if code != "" && code != candidate {
							return nil, nil, fmt.Errorf("预订选项 %s 对应多个销售选项，等待明确匹配", option)
						}
						code, family, product = candidate, f, raw
					}
				}
			}
		}
		if cpuFamily(family) {
			continue
		}
		if code == "" {
			code = option
		}
		order = append(order, code)
		selected[family] = true
		if stockFamily(family) || (family == "" && !preorderNetworkOption(option)) {
			stock = append(stock, product)
		}
	}
	var defaults []string
	for family, mandatory := range pc.mandatoryFamilies {
		if mandatory && !selected[family] && !stockFamily(family) {
			code := pc.addonDefaults[family]
			if code != "" && containsStr(pc.addonFamilies[family], code) {
				defaults = append(defaults, code)
			}
		}
	}
	sort.Strings(defaults)
	order = append(order, defaults...)
	return OrderOptions(stock), OrderOptions(order), nil
}

func stockFamily(family string) bool {
	return family == "memory" || family == "storage" || family == "system-storage" || family == "gpu"
}

func cpuFamily(family string) bool {
	return family == "cpu" || family == "processor" || family == "processors"
}

func preorderCPUOption(option string) bool {
	option = strings.ToLower(option)
	return cpuFamily(option) || strings.HasPrefix(option, "cpu-") || strings.HasPrefix(option, "processor-")
}

func preorderNetworkOption(option string) bool {
	option = strings.ToLower(option)
	return strings.HasPrefix(option, "bandwidth-") || strings.HasPrefix(option, "traffic-") || strings.HasPrefix(option, "vrack-")
}

// 目录 SKU 可能带这个型号(或去掉区域后缀的基础型号)作后缀,只剥掉本型号自己的后缀。
func preorderProductCode(option, plan string) string {
	suffixes := []string{plan}
	if last := strings.LastIndex(plan, "-"); last > 0 {
		region := plan[last+1:]
		if region != "" && strings.Trim(region, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") == "" {
			suffixes = append(suffixes, plan[:last])
		}
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(strings.ToLower(option), "-"+strings.ToLower(suffix)) {
			return option[:len(option)-len(suffix)-1]
		}
	}
	return option
}
