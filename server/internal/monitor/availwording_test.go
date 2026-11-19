package monitor

import "testing"

// 原始可用性值决定"要不要现在就抢":1H-low 是马上能发货但库存告急,
// 720H 是三十天后才交付。译错等于给出错误的紧迫感。
func TestAvailabilityCN(t *testing.T) {
	cases := map[string]string{
		"1H-low":      "1小时内有货 - 低库存",
		"1H-high":     "1小时内有货 - 高库存",
		"24H":         "24小时内有货",
		"72H":         "72小时内有货（约3天）",
		"120H":        "120小时内有货（约5天）",
		"720H":        "720小时内有货（约30天）",
		"1440H":       "1440小时内有货（约60天）",
		"unavailable": "无货",
		"comingSoon":  "即将上线（还不能下单）",
		"unknown":     "OVH 未提供状态",
		"":            "",
		// 认不出来的原样返回,不瞎猜
		"someNewValue": "someNewValue",
	}
	for in, want := range cases {
		if got := AvailabilityCN(in); got != want {
			t.Errorf("AvailabilityCN(%q) = %q, 期望 %q", in, got, want)
		}
	}
}
