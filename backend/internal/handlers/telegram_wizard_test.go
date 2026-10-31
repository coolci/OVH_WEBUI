package handlers

import (
	"testing"
	"time"
)

func TestGetCfgOffer_OneTimeUse(t *testing.T) {
	tok := putCfgOffer(&cfgOffer{
		PlanCode: "24sk202",
		Mode:     "q",
		Expires:  time.Now().Add(time.Minute),
	})

	// 第一次获取应该成功
	o1, ok1 := getCfgOffer(tok)
	if !ok1 || o1 == nil || o1.PlanCode != "24sk202" {
		t.Fatalf("第一次调用 getCfgOffer 应当成功, got ok=%v, offer=%v", ok1, o1)
	}

	// 第二次获取应该已作废（阅后即焚）
	o2, ok2 := getCfgOffer(tok)
	if ok2 || o2 != nil {
		t.Fatalf("第二次调用 getCfgOffer 应当失效(防止重复点击重复下单), got ok=%v, offer=%v", ok2, o2)
	}
}

func TestEnqueueFromNotifyButton_MemoryCacheDeleted(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	st.TelegramRepo = nil // 模拟未落库或 fallback 到内存缓存的场景

	btnID := "test-mem-btn-uuid"
	mon.AddMessageUUID(btnID, "24sk202", "gra", []string{"ram-32g"}, nil)

	// 确认内存缓存已添加
	if cached := mon.MessageUUIDCacheLookup(btnID); cached == nil {
		t.Fatal("expected cache entry to exist before enqueue")
	}

	// 触发处理
	enqueueFromNotifyButton(st, mon, "cb-123", int64(12345), 100, btnID, "quick", "", false)

	// 处理后内存缓存应当被立即删除
	if cached := mon.MessageUUIDCacheLookup(btnID); cached != nil {
		t.Fatalf("expected cache entry to be deleted after use, got: %+v", cached)
	}
}

func TestHandleInlineCallback_AllDCs_NoValidDCs_RefusesBlindEnqueue(t *testing.T) {
	st, mon := newWatchTestMonitor(t)
	// 没有任何可售机房的虚拟型号
	handled := handleInlineCallback(st, mon, "cb-z", int64(12345), 10, "i:Z:q:nonexistent-plan")
	if !handled {
		t.Fatal("expected handleInlineCallback to return true")
	}

	// 确认没有盲目向队列塞入 StandardDCs 的 12 个任务
	st.QueueMu.Lock()
	qLen := len(st.Queue)
	st.QueueMu.Unlock()

	if qLen != 0 {
		t.Fatalf("expected 0 queue items for plan with no valid DCs, got %d", qLen)
	}
}

