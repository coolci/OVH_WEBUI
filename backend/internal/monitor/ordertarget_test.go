package monitor

import "testing"

// selectOrderTargets 复刻 check.go 里挑选下单目标的判据。
// 抽出来是为了能单独测 —— 这段逻辑决定"补货了到底下不下单",
// 而它出过两次问题,每次都是静默漏抢。
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

// 关掉「有货时提醒」不能影响下单。
//
// 以前 9 处 statusChanged 全被 if cfg.NotifyAvailable 包着,而下单链路是
// statusChanged → notifications → availables → orderTargets → batchOrder,
// 于是关掉提醒 = 关掉自动下单。界面上这是三个独立复选框,用户为了不被
// TG 刷屏关掉提醒,得到一个看着在跑、永远不下单的监控。
func TestOrderTargets_不受通知开关影响(t *testing.T) {
	ns := []notification{
		{dc: "gra", changeType: "available", hasOld: true, oldStatus: "unavailable", notify: false},
	}
	got := selectOrderTargets(ns)
	if len(got) != 1 {
		t.Fatalf("notify=false(用户关了有货提醒)时仍然应该下单,实际挑出 %d 个目标", len(got))
	}
}

// 价格校验抖一下不能让整个补货窗口都不下单。
//
// verifyPriceAvailable 把本地 HTTP 异常 / 30 秒超时 / OVH 429 全算失败,
// 而补货那一刻正是 OVH 最容易抖的时候。路径:
//
//	unavailable → price_check_failed(抖) → available
//
// 以前只认 oldStatus=="unavailable",最后那步跳变被过滤掉 ——
// 通知发了「🎉 上架」,一单没下,此后 lastStatus 停在 available 再无跳变。
func TestOrderTargets_价格校验抖动后恢复也要下单(t *testing.T) {
	ns := []notification{
		{dc: "gra", changeType: "available", hasOld: true, oldStatus: "price_check_failed", notify: true},
	}
	if got := selectOrderTargets(ns); len(got) != 1 {
		t.Errorf("price_check_failed → available 是补货,必须下单,实际挑出 %d 个", len(got))
	}
}

// 边界:这几种不该下单
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

// 首次检查就有货要下单(hasOld=false)
func TestOrderTargets_首次检查有货(t *testing.T) {
	ns := []notification{{dc: "gra", changeType: "available", hasOld: false, notify: true}}
	if got := selectOrderTargets(ns); len(got) != 1 {
		t.Errorf("首次检查发现有货应该下单,实际 %d 个", len(got))
	}
}
