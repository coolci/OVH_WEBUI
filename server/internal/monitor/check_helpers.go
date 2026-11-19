package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/ovh-webui/server/internal/domain/datacenter"
)

func copyStatus(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func statusEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func matchesMonitoredDC(monitored []string, dc string) bool {
	for _, m := range monitored {
		if strings.EqualFold(strings.TrimSpace(m), strings.TrimSpace(dc)) || datacenter.SameZoneOrAlias(m, dc) {
			return true
		}
	}
	return false
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// calcDuration 计算最近一次相反状态到现在的历时
func (m *Monitor) calcDuration(sub *Subscription, dc, configDisplay string, targetChangeTypes []string) string {
	// 取副本再倒着找:检查 goroutine 之外还有 HTTP 侧在读同一条订阅,
	// 而 History 正被本轮 appendHistory 追加,直接遍历真身就是竞争。
	history := sub.historySnapshot()
	var lastTS string
	for i := len(history) - 1; i >= 0; i-- {
		entry := history[i]
		if sub.checkParent != nil && entry.Config["account_id"] != sub.AutoOrderAccountID {
			continue
		}
		if entry.Datacenter != dc {
			continue
		}
		matched := false
		for _, t := range targetChangeTypes {
			if entry.ChangeType == t {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if configDisplay != "" && entry.Config != nil {
			if d, ok := entry.Config["display"].(string); ok && d != configDisplay {
				continue
			}
		}
		lastTS = entry.Timestamp
		if lastTS != "" {
			break
		}
	}
	if lastTS == "" {
		return ""
	}
	startDT, err := time.Parse(time.RFC3339Nano, lastTS)
	if err != nil {
		startDT, err = time.Parse(time.RFC3339, lastTS)
		if err != nil {
			return ""
		}
	}
	delta := m.nowBeijing().Sub(startDT)
	totalSec := int(delta.Seconds())
	if totalSec < 0 {
		totalSec = 0
	}
	days := totalSec / 86400
	rem := totalSec % 86400
	hours := rem / 3600
	minutes := (rem % 3600) / 60
	seconds := rem % 60
	if days > 0 {
		return fmt.Sprintf("历时 %d天%d小时%d分%d秒", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("历时 %d小时%d分%d秒", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("历时 %d分%d秒", minutes, seconds)
	}
	return fmt.Sprintf("历时 %d秒", seconds)
}

// configMatchesFilter 判断一套配置是否落在订阅指定的配置范围内。
//
// want 为空 = 不筛,盯全部配置(老行为)。
// 非空时要求硬件项都出现在库存 options 里;带宽/vRack 仅在询价及下单时校验 —— 用"子集"而不是
// "完全相等":用户在 TG 上只会挑内存和存储两项,而一套配置的 options 还包含
// 带宽、vRack 之类他没挑也不关心的东西,要求相等会一个都匹配不上。
func configMatchesFilter(want, have []string) bool {
	if len(want) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(have))
	for _, h := range have {
		set[h] = struct{}{}
	}
	for _, w := range want {
		// 库存仅区分硬件；网络 addon 留给询价和下单校验。
		if strings.HasPrefix(w, "bandwidth-") || strings.HasPrefix(w, "vrack-") {
			continue
		}
		if _, ok := set[w]; !ok {
			return false
		}
	}
	return true
}

// monitoringOptions 保留库存硬件及用户选定的带宽/vRack,避免按默认网络规格下单。
func monitoringOptions(hardware, selected []string) []string {
	result := append([]string(nil), hardware...)
	seen := map[string]bool{}
	for _, code := range result {
		seen[code] = true
	}
	for _, code := range selected {
		if !seen[code] {
			result = append(result, code)
			seen[code] = true
		}
	}
	return result
}

// ConfigMatchesFilter 导出版，给测试和别的包用。语义见 configMatchesFilter。
func ConfigMatchesFilter(want, have []string) bool { return configMatchesFilter(want, have) }

// NeedsPriceCheck 判定当前机房是否需要执行购物车价格校验：
// 仅在上架时（首检有货、此前无货、或此前验价失败重试）执行验价；
// 持续在售（oldStatus=="available"）与下架（orderable=false）时不重复验价，
// 避免对 OVH 购物车接口过度请求限流，以及避免在下架被抢空瞬间报 500 误告警。
func NeedsPriceCheck(orderable bool, hasOld bool, oldStatus string) bool {
	return orderable && (!hasOld || oldStatus != "available")
}
