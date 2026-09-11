package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTgBtnLabelFitsTelegramLimit(t *testing.T) {
	long := "💾 " + strings.Repeat("128GB DDR5 ECC / 2x 3.84TB NVMe ", 5) + "(🟢 12机房有货)"
	got := tgBtnLabel(long)
	if n := utf8.RuneCountInString(got); n > 64 {
		t.Fatalf("按钮文字 %d 个字符，超过 Telegram 64 上限: %q", n, got)
	}
	short := "💾 64GB / 2x480SSD (🟢 3机房有货)"
	if tgBtnLabel(short) != short {
		t.Fatalf("短标签不该被截断: %q", tgBtnLabel(short))
	}
}

func TestCfgAndNarrowCallbackDataFits64Bytes(t *testing.T) {
	tok := fmt.Sprintf("%x", time.Now().UnixNano())
	cases := []string{
		fmt.Sprintf("i:cfg:%s:%d", tok, 7),
		fmt.Sprintf("i:cfg:%s:any", tok),
		fmt.Sprintf("i:mon:n:%s:%d", tok, 7),
		fmt.Sprintf("i:mon:n:%s:all", tok),
	}
	for _, data := range cases {
		if n := len(data); n > 64 {
			t.Fatalf("callback_data %d 字节超过 64：%s", n, data)
		}
	}
}
