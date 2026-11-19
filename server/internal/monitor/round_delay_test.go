package monitor

import (
	"testing"
	"time"
)

// 检查周期按"上一轮开始 + 间隔"定点起跑:本轮跑了 1.5 秒、间隔 5 秒 → 再等 3.5 秒,
// 而不是以前的再睡满 5 秒(实际周期 6.5 秒);本轮超过间隔就立即开始下一轮。
func TestNextRoundDelayIsFixedRate(t *testing.T) {
	start := time.Unix(1_000, 0)
	if d := nextRoundDelay(start, start.Add(1500*time.Millisecond), 5); d != 3500*time.Millisecond {
		t.Fatalf("应再等 3.5s, got %v", d)
	}
	if d := nextRoundDelay(start, start.Add(7*time.Second), 5); d != 0 {
		t.Fatalf("本轮超时应立即开始下一轮, got %v", d)
	}
}
