package catalog

import "strings"

// OrderOptions 去重并保序;每个请求的 addon 都保留,支持与否交给 OVH 的选项列表判断。
func OrderOptions(options []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, option := range options {
		option = strings.TrimSpace(option)
		key := strings.ToLower(option)
		if option != "" && !seen[key] {
			result = append(result, option)
			seen[key] = true
		}
	}
	return result
}

// 预订硬件只接受原始规格加机型后缀,不按标准化结果替换成另一套配置。
func MatchPreorderEcoOption(opts []map[string]interface{}, wanted string) (map[string]interface{}, string, string) {
	w := strings.ToLower(strings.TrimSpace(wanted))
	filtered := []map[string]interface{}{}
	for _, opt := range opts {
		code, _ := opt["planCode"].(string)
		c := strings.ToLower(code)
		suffix := strings.TrimPrefix(c, w+"-")
		if c == w || (suffix != c && (ecoStockPlanCode.MatchString(suffix) || suffix == "rise" || suffix == "sys" || suffix == "sk" || suffix == "adv" || suffix == "ks")) {
			filtered = append(filtered, opt)
		}
	}
	return MatchEcoOption(filtered, wanted)
}

// MatchEcoOption 把用户/FQN 给的 addon 码匹配到 OVH /order/cart/{id}/eco/options
// 返回的那批可选项上,返回命中的选项、它的完整 planCode、以及命中档次(日志用)。
//
// **这个函数直接决定账单**:选错 addon 就是下单到别的配置、付另一个价钱
// (实测 26sk50a-v1 上,纯 2x960NVMe 是 €0/月,而多编了一套混合盘的那条是 €24/月)。
//
// 严禁任何反向前缀降配匹配（如双盘组误命中单盘组），正向前缀强制 "-" 边界且后缀只允许为机型/版本/区域标识，
// 存储介质（nvme/ssd/hdd/sas/sa）全程保留不擦除。
func MatchEcoOption(opts []map[string]interface{}, wanted string) (map[string]interface{}, string, string) {
	w := strings.ToLower(strings.TrimSpace(wanted))
	if w == "" {
		return nil, "", ""
	}
	wStd := StandardizeConfig(w)

	type cand struct {
		opt      map[string]interface{}
		code     string
		strength int // 同档内:命中的公共前缀越长越可信
		size     int // 同档同强度:整体越短越可信(多出来的段=多编了一套配置)
	}
	tierNames := [4]string{"原样相等", "原始码前缀", "标准化相等", "标准化前缀"}
	var tiers [4]*cand

	for _, o := range opts {
		code, _ := o["planCode"].(string)
		c := strings.ToLower(strings.TrimSpace(code))
		if c == "" {
			continue
		}
		cStd := StandardizeConfig(c)
		tier, strength := -1, 0
		switch {
		case c == w:
			tier, strength = 0, len(c)
		case strings.HasPrefix(c, w+"-") && IsAllowedOptionSuffix(strings.TrimPrefix(c, w+"-")):
			tier, strength = 1, len(w)
		case cStd != "" && cStd == wStd:
			tier, strength = 2, len(cStd)
		case cStd != "" && wStd != "" && strings.HasPrefix(cStd, wStd+"-") && IsAllowedOptionSuffix(strings.TrimPrefix(cStd, wStd+"-")):
			tier, strength = 3, len(wStd)
		}
		if tier < 0 {
			continue
		}
		cur := tiers[tier]
		if cur == nil || strength > cur.strength || (strength == cur.strength && len(c) < cur.size) {
			tiers[tier] = &cand{opt: o, code: code, strength: strength, size: len(c)}
		}
	}
	for i, t := range tiers {
		if t != nil {
			return t.opt, t.code, tierNames[i]
		}
	}
	return nil, "", ""
}
