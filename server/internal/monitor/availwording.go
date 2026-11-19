package monitor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// OVH 的 dedicated.AvailabilityEnum(三区一致):
//
//	120H 1440H 1H-high 1H-low 2160H 240H 480H 720H 72H 24H
//	comingSoon unavailable unknown
//
// 只有 \d+H(-high|-low)? 这一族是"多久能交付"的承诺,也就是真能下单。
// 这个原始值决定的是"要不要现在就抢":1H-low 是马上能发货但库存告急,
// 720H 是三十天后才交付 —— 两者对用户的意义天差地别,
// 而通知里以前只写"有货",把这个区别整个抹掉了。
var availRe = regexp.MustCompile(`^(\d+)H(?:-(high|low))?$`)

// AvailabilityCN 把 OVH 的可用性取值译成人话。
// 认不出来的原样返回 —— 宁可让用户看到一个陌生代码,也不要瞎猜一个意思。
func AvailabilityCN(raw string) string {
	s := strings.TrimSpace(raw)
	switch s {
	case "":
		return ""
	case "unavailable":
		return "无货"
	case "comingSoon":
		return "即将上线（还不能下单）"
	case "unknown":
		return "OVH 未提供状态"
	}
	m := availRe.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	hours, err := strconv.Atoi(m[1])
	if err != nil {
		return s
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d小时内有货", hours))
	// 超过一天的用天数补一句:没人愿意在手机上心算 1440 小时是多久
	if hours >= 48 {
		b.WriteString(fmt.Sprintf("（约%d天）", hours/24))
	}
	switch m[2] {
	case "low":
		b.WriteString(" - 低库存")
	case "high":
		b.WriteString(" - 高库存")
	}
	return b.String()
}
