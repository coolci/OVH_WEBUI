package monitor

import "testing"

func selectOrderTargets(ns []notification) []notification {
	out := []notification{}
	for _, n := range ns {
		if n.changeType != "available" || n.priceCheckFailed {
			continue
		}
		if !n.hasOld || n.oldStatus == "unavailable" || n.oldStatus == "price_check_failed" {
			out = append(out, n)
		}
	}
	return out
}

func TestOrderTargets_不受通知开关影响(t *testing.T) {
	ns := []notification{
		{dc: "gra", changeType: "available", hasOld: true, oldStatus: "unavailable", notify: false},
	}
	got := selectOrderTargets(ns)
	if len(got) != 1 {
		t.Fatalf("notify=false(用户关了有货提醒)时仍然应该下单,实际挑出 %d 个目标", len(got))
	}
}

func TestOrderTargets_价格校验抖动后恢复也要下单(t *testing.T) {
	ns := []notification{
		{dc: "gra", changeType: "available", hasOld: true, oldStatus: "price_check_failed", notify: true},
	}
	if got := selectOrderTargets(ns); len(got) != 1 {
		t.Errorf("price_check_failed → available 是补货,必须下单,实际挑出 %d 个", len(got))
	}
}

func TestOrderTargets_不该下单的情况(t *testing.T) {
	cases := []struct {
		name string
		n    notification
	}{
		{"本轮价格校验就是失败的", notification{changeType: "available", priceCheckFailed: true, hasOld: true, oldStatus: "unavailable"}},
		{"变成无货", notification{changeType: "unavailable", hasOld: true, oldStatus: "available"}},
		{"一直有货(没跳变)", notification{changeType: "available", hasOld: true, oldStatus: "available"}},
		{"跳到价格校验失败", notification{changeType: "price_check_failed", hasOld: true, oldStatus: "unavailable"}},
	}
	for _, c := range cases {
		if got := selectOrderTargets([]notification{c.n}); len(got) != 0 {
			t.Errorf("%s:不该下单,却挑出了 %d 个", c.name, len(got))
		}
	}
}

func TestOrderTargets_首次检查有货(t *testing.T) {
	ns := []notification{{dc: "gra", changeType: "available", hasOld: false, notify: true}}
	if got := selectOrderTargets(ns); len(got) != 1 {
		t.Errorf("首次检查发现有货应该下单,实际 %d 个", len(got))
	}
}
