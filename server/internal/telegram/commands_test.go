package telegram

import (
	"strings"
	"testing"
)

func TestParseBotCommand(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantArgs []string
		wantNil  bool
	}{
		{"", "", nil, true},
		{"24ska01 gra", "", nil, true},
		{"/start", "start", nil, false},
		{"/help", "help", nil, false},
		{"/buy 24ska01 gra", "buy", []string{"24ska01", "gra"}, false},
		{"/buy@MyBot 24ska01 gra 2", "buy", []string{"24ska01", "gra", "2"}, false},
		{"/STOCK 24ska01", "stock", []string{"24ska01"}, false},
		{"/queue 24ska01 gra 1 ram-32g,softraid-2x512nvme", "queue",
			[]string{"24ska01", "gra", "1", "ram-32g,softraid-2x512nvme"}, false},
		{"/price 24ska01 gra", "price", []string{"24ska01", "gra"}, false},
		{"/monitor 24ska01 gra rbx", "monitor", []string{"24ska01", "gra", "rbx"}, false},
		{"/unknown foo", "unknown", []string{"foo"}, false},
	}
	for _, tc := range cases {
		got := ParseBotCommand(tc.in)
		if tc.wantNil {
			if got != nil {
				t.Errorf("ParseBotCommand(%q) = %+v, want nil", tc.in, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("ParseBotCommand(%q) = nil, want name=%s", tc.in, tc.wantName)
			continue
		}
		if got.Name != tc.wantName {
			t.Errorf("ParseBotCommand(%q).Name = %q, want %q", tc.in, got.Name, tc.wantName)
		}
		if len(got.Args) != len(tc.wantArgs) {
			t.Errorf("ParseBotCommand(%q).Args = %v, want %v", tc.in, got.Args, tc.wantArgs)
			continue
		}
		for i := range tc.wantArgs {
			if got.Args[i] != tc.wantArgs[i] {
				t.Errorf("ParseBotCommand(%q).Args[%d] = %q, want %q", tc.in, i, got.Args[i], tc.wantArgs[i])
			}
		}
	}
}

func TestParseOrderArgs(t *testing.T) {
	info := ParseOrderArgs([]string{"24ska01", "gra", "2"})
	if info == nil {
		t.Fatal("expected OrderInfo")
	}
	if info.PlanCode != "24ska01" || info.Datacenter != "gra" || info.Quantity != 2 {
		t.Fatalf("got %+v", info)
	}
}

func TestParseOrderCoupon(t *testing.T) {
	cases := []struct {
		input      string
		wantCoupon string
		wantPlan   string
		wantDc     string
		wantQty    int
	}{
		{"24ska01 gra 1 coupon=SAVE10", "SAVE10", "24ska01", "gra", 1},
		{"24ska01 gra 2 #BF2026", "BF2026", "24ska01", "gra", 2},
		{"24ska01 gra code:DISCOUNT20", "DISCOUNT20", "24ska01", "gra", 1},
		{"24ska01 gra coupon PROMO50", "PROMO50", "24ska01", "gra", 1},
		{"24ska01 gra 优惠码 666ABC", "666ABC", "24ska01", "gra", 1},
		{"24ska01 gra c=SUPERDEAL", "SUPERDEAL", "24ska01", "gra", 1},
	}
	for _, tc := range cases {
		info := ParseOrderMessage(tc.input)
		if info == nil {
			t.Fatalf("ParseOrderMessage(%q) returned nil", tc.input)
		}
		if info.Coupon != tc.wantCoupon {
			t.Errorf("ParseOrderMessage(%q).Coupon = %q, want %q", tc.input, info.Coupon, tc.wantCoupon)
		}
		if info.PlanCode != tc.wantPlan {
			t.Errorf("ParseOrderMessage(%q).PlanCode = %q, want %q", tc.input, info.PlanCode, tc.wantPlan)
		}
		if info.Datacenter != tc.wantDc {
			t.Errorf("ParseOrderMessage(%q).Datacenter = %q, want %q", tc.input, info.Datacenter, tc.wantDc)
		}
		if info.Quantity != tc.wantQty {
			t.Errorf("ParseOrderMessage(%q).Quantity = %d, want %d", tc.input, info.Quantity, tc.wantQty)
		}
	}
}

func TestIsKnownCommand(t *testing.T) {
	if !IsKnownCommand("buy") || !IsKnownCommand("STOCK") {
		t.Fatal("expected known")
	}
	if !IsKnownCommand("interval") || !IsKnownCommand("iv") {
		t.Fatal("expected interval aliases")
	}
	if IsKnownCommand("foo") {
		t.Fatal("expected unknown")
	}
}

func TestChatIDToString(t *testing.T) {
	if got := chatIDToString(float64(123456789)); got != "123456789" {
		t.Fatalf("float64: got %q", got)
	}
	if got := chatIDToString(int64(-100123)); got != "-100123" {
		t.Fatalf("int64: got %q", got)
	}
	if got := chatIDToString(" 42 "); got != "42" {
		t.Fatalf("string: got %q", got)
	}
}

func TestHelpMessageNonEmpty(t *testing.T) {
	help := HelpMessage()
	if !strings.Contains(help, "/buy") {
		t.Fatal("help should mention /buy")
	}
	if !strings.Contains(help, "/interval") {
		t.Fatal("help should mention /interval")
	}
}
