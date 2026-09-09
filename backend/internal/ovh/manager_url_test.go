package ovh

import "testing"

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

func TestManagerOrderURLEmptyID(t *testing.T) {
	if got := ManagerOrderURL("ovh-eu", ""); got != "" {
		t.Fatalf("空订单号应返回空串,实际 %q", got)
	}
}
