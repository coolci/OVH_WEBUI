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

func TestIsKnownCommand(t *testing.T) {
	if !IsKnownCommand("buy") || !IsKnownCommand("STOCK") {
		t.Fatal("expected known")
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
	if !strings.Contains(HelpMessage(), "/buy") {
		t.Fatal("help should mention /buy")
	}
}
