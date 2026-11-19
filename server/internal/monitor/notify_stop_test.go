package monitor

import (
	"testing"
	"time"
)

func TestCheckNotifyOrStop_NoChannelsDoesNotStop(t *testing.T) {
	m, _ := setupTestMonitor(t)
	m.running = true

	// 强制满足 checkNotifyOrStop 的检测时机
	m.tgCheckMu.Lock()
	m.lastTGCheck = time.Now().Add(-10 * time.Minute)
	m.tgCheckMu.Unlock()

	cont := m.checkNotifyOrStop()
	if !cont {
		t.Fatal("未配置通知通道时 checkNotifyOrStop 应该返回 true（继续监控运行）")
	}
	if !m.Running() {
		t.Fatal("未配置通知通道时不应该自动调用 m.Stop() 停掉监控")
	}
}
