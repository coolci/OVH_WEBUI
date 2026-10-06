package ovh

import "testing"

// 三个大区的控制面板互不认对方的订单号,发错区用户只会看到"订单不存在"。
func TestManagerOrderURLPerRegion(t *testing.T) {
	cases := []struct{ endpoint, want string }{
		{"ovh-eu", "https://manager.eu.ovhcloud.com/dedicated/#/billing/order?orderId=123"},
		{"ovh-us", "https://manager.us.ovhcloud.com/dedicated/#/billing/order?orderId=123"},
		{"ovh-ca", "https://manager.ca.ovhcloud.com/dedicated/#/billing/order?orderId=123"},
		{"", "https://manager.eu.ovhcloud.com/dedicated/#/billing/order?orderId=123"},
	}
	for _, c := range cases {
		if got := ManagerOrderURL(c.endpoint, "123"); got != c.want {
			t.Fatalf("ManagerOrderURL(%q) = %q, 期望 %q", c.endpoint, got, c.want)
		}
	}
}

// 没有订单号时返回空,让调用方去说人话,而不是拼出一个打不开的链接
func TestManagerOrderURLEmptyID(t *testing.T) {
	if got := ManagerOrderURL("ovh-eu", ""); got != "" {
		t.Fatalf("空订单号应返回空串,实际 %q", got)
	}
}
