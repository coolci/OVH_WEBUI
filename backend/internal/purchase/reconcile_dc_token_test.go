package purchase

import "testing"

// 对账判"订单属于别的机房"必须按完整机房记号,不能按子串:
// 子串误判会让一张真实建成的订单被忽略,结账超时后的重试就会再买一台。
func TestOrderDetailContradictsDCUsesWholeTokens(t *testing.T) {
	cases := []struct {
		name, desc, domain, target string
		want                       bool
	}{
		{"graphics is not gra", "Server with graphics card", "", "bhs", false},
		{"along is not lon", "Delivered along with backup", "", "rbx", false},
		{"saving is not vin", "Monthly saving plan", "", "gra", false},
		{"frame is not fra", "Rack frame installation", "", "sbg", false},
		{"other dc token contradicts", "KS-A | rbx8", "", "gra", true},
		{"own dc with number suffix", "KS-A | gra3", "", "gra", false},
		{"domain carries the dc", "", "ns123.ip-1-2-3.eu.gra", "gra", false},
		{"other dc in domain", "", "server.bhs5", "gra", true},
		{"no dc info at all", "Eco server", "", "gra", false},
		{"empty target never contradicts", "rbx", "", "", false},
	}
	for _, c := range cases {
		detail := map[string]interface{}{"description": c.desc, "domain": c.domain}
		if got := orderDetailContradictsDC(detail, c.target); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
