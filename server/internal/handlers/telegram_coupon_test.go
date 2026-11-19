package handlers

import (
	"strings"
	"testing"

	"github.com/ovh-webui/server/internal/telegram"
)

func TestDispatchCouponCommand(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	_ = telegram.ClearActiveCoupon(st)

	// 1. 查看未设置状态
	cmd := telegram.ParseBotCommand("/coupon")
	reply := dispatchTelegramCommand(st, mon, cmd)
	if !strings.Contains(reply, "当前未设置默认优惠码") {
		t.Fatalf("expected unconfigured notice, got %q", reply)
	}

	// 2. 设置优惠码
	cmdSet := telegram.ParseBotCommand("/coupon SAVE2026")
	replySet := dispatchTelegramCommand(st, mon, cmdSet)
	if !strings.Contains(replySet, "SAVE2026") || !strings.Contains(replySet, "默认优惠码设置成功") {
		t.Fatalf("expected success setting coupon, got %q", replySet)
	}
	if got := telegram.ActiveCoupon(st); got != "SAVE2026" {
		t.Fatalf("expected SAVE2026, got %q", got)
	}

	// 3. 查看已激活状态
	replyCheck := dispatchTelegramCommand(st, mon, cmd)
	if !strings.Contains(replyCheck, "SAVE2026") || !strings.Contains(replyCheck, "激活中") {
		t.Fatalf("expected active notice, got %q", replyCheck)
	}

	// 4. 清除优惠码
	cmdClear := telegram.ParseBotCommand("/coupon clear")
	replyClear := dispatchTelegramCommand(st, mon, cmdClear)
	if !strings.Contains(replyClear, "已清除默认优惠码") {
		t.Fatalf("expected clear notice, got %q", replyClear)
	}
	if got := telegram.ActiveCoupon(st); got != "" {
		t.Fatalf("expected empty coupon after clear, got %q", got)
	}
}
