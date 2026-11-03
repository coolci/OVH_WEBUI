package handlers

import (
	"strings"
	"testing"

	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

func TestHudDashboardMessageAndKeyboard(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	_ = telegram.ClearActiveCoupon(st)

	// 1. 无任务、无优惠码的基础 HUD
	hud1 := hudDashboardMessage(st, mon)
	if !strings.Contains(hud1, "智能控制中枢") {
		t.Fatalf("expected HUD title, got %q", hud1)
	}
	if !strings.Contains(hud1, "未设置") {
		t.Fatalf("expected unset coupon notice, got %q", hud1)
	}
	if !strings.Contains(hud1, "队列空闲") {
		t.Fatalf("expected idle queue, got %q", hud1)
	}

	kb1 := dashboardKeyboard(st, mon)
	if kb1 == nil || kb1["inline_keyboard"] == nil {
		t.Fatalf("expected valid inline keyboard markup, got %v", kb1)
	}

	// 2. 设置活跃优惠码与队列任务后，验证 HUD 动态渲染与按钮角标
	_ = telegram.SetActiveCoupon(st, "PROMO99")
	st.QueueMu.Lock()
	st.Queue = append(st.Queue, types.QueueItem{
		ID:         "task-1",
		PlanCode:   "24ska01",
		Datacenter: "gra",
		Status:     "running",
	})
	st.QueueMu.Unlock()

	hud2 := hudDashboardMessage(st, mon)
	if !strings.Contains(hud2, "PROMO99") {
		t.Fatalf("expected coupon PROMO99 in HUD, got %q", hud2)
	}
	if !strings.Contains(hud2, "1 个任务挂机抢购中") {
		t.Fatalf("expected 1 active task in HUD, got %q", hud2)
	}
	if !strings.Contains(hud2, "coupon=PROMO99") {
		t.Fatalf("expected coupon appended to quick example command, got %q", hud2)
	}

	kb2 := dashboardKeyboard(st, mon)
	rows, ok := kb2["inline_keyboard"].([][]map[string]string)
	if !ok || len(rows) < 4 {
		t.Fatalf("expected 4 rows in keyboard, got %v", kb2)
	}

	// 验证按钮角标
	foundTaskBadge := false
	foundCouponBadge := false
	for _, row := range rows {
		for _, btn := range row {
			if strings.Contains(btn["text"], "抢购任务 (1)") {
				foundTaskBadge = true
			}
			if strings.Contains(btn["text"], "优惠码 (PROMO99)") {
				foundCouponBadge = true
			}
		}
	}
	if !foundTaskBadge {
		t.Fatalf("expected task badge (1), rows=%v", rows)
	}
	if !foundCouponBadge {
		t.Fatalf("expected coupon badge (PROMO99), rows=%v", rows)
	}
}

func TestPersistentReplyKeyboard(t *testing.T) {
	kb := telegram.PersistentReplyKeyboard()
	if kb == nil {
		t.Fatal("expected non-nil persistent keyboard")
	}
	if kb["resize_keyboard"] != true || kb["is_persistent"] != true {
		t.Fatalf("expected resize_keyboard and is_persistent true, got %v", kb)
	}
	rows, ok := kb["keyboard"].([][]map[string]string)
	if !ok || len(rows) != 2 {
		t.Fatalf("expected 2 rows in persistent reply keyboard, got %v", kb)
	}
	if rows[0][0]["text"] != "⚡ 快速下单" {
		t.Fatalf("expected first button to be ⚡ 快速下单, got %s", rows[0][0]["text"])
	}
}

