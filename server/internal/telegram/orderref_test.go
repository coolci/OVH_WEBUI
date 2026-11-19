package telegram

import "testing"

// @账户 可以出现在任何位置 —— 手机上很容易顺手打在末尾，
// 而末尾正好是 options 的地盘。解析错了会把账户名当成配置项发给 OVH。
func TestParseOrderMessageAccountRef(t *testing.T) {
	cases := []struct {
		in       string
		plan     string
		dc       string
		qty      int
		ref      string
		optCount int
	}{
		{"24sk602", "24sk602", "", 1, "", 0},
		{"24sk602 @us", "24sk602", "", 1, "us", 0},
		{"24sk602 gra @us", "24sk602", "gra", 1, "us", 0},
		{"24sk602 gra 2 @us", "24sk602", "gra", 2, "us", 0},
		{"24sk602 @us gra 2", "24sk602", "gra", 2, "us", 0},
		{"24sk602 @all", "24sk602", "", 1, "all", 0},
		{"24sk602 @2", "24sk602", "", 1, "2", 0},
		{"24sk602 @US", "24sk602", "", 1, "us", 0}, // 大小写不敏感
		// 带 options 时 @ 不能被当成配置项
		{"24sk602 gra 2 ram-64g,softraid-2x480ssd @us", "24sk602", "gra", 2, "us", 2},
		{"24sk602 @us ram-64g,softraid-2x480ssd", "24sk602", "", 1, "us", 2},
	}
	for _, c := range cases {
		got := ParseOrderMessage(c.in)
		if got == nil {
			t.Errorf("%q 解析失败", c.in)
			continue
		}
		if got.PlanCode != c.plan || got.Datacenter != c.dc || got.Quantity != c.qty {
			t.Errorf("%q → plan=%q dc=%q qty=%d，期望 %q/%q/%d",
				c.in, got.PlanCode, got.Datacenter, got.Quantity, c.plan, c.dc, c.qty)
		}
		if got.AccountRef != c.ref {
			t.Errorf("%q → AccountRef=%q，期望 %q", c.in, got.AccountRef, c.ref)
		}
		if len(got.Options) != c.optCount {
			t.Errorf("%q → %d 个 options，期望 %d：%v", c.in, len(got.Options), c.optCount, got.Options)
		}
		for _, o := range got.Options {
			if len(o) > 0 && o[0] == '@' {
				t.Errorf("%q 把账户当成了配置项：%v", c.in, got.Options)
			}
		}
	}
}

// 光一个 @ 不算账户引用，不能把它吃掉
func TestParseOrderMessageBareAt(t *testing.T) {
	got := ParseOrderMessage("24sk602 @")
	if got == nil {
		t.Fatal("应当能解析")
	}
	if got.AccountRef != "" {
		t.Fatalf("裸 @ 不该被当成账户引用，实际 %q", got.AccountRef)
	}
}
