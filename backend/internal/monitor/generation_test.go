package monitor

import "testing"

func TestGeneration_旧循环在重启后失效(t *testing.T) {
	m := &Monitor{}

	m.subsMu.Lock()
	m.running = true
	m.generation++
	gen1 := m.generation
	m.subsMu.Unlock()

	if !m.stillMine(gen1) {
		t.Fatal("第一代刚启动就应该是当前代")
	}

	m.subsMu.Lock()
	m.running = false
	m.subsMu.Unlock()
	if m.stillMine(gen1) {
		t.Error("已停止时不该继续跑")
	}

	m.subsMu.Lock()
	m.running = true
	m.generation++
	gen2 := m.generation
	m.subsMu.Unlock()

	if m.stillMine(gen1) {
		t.Error("重启之后旧循环(第一代)必须退出,否则两个循环并存会重复下单")
	}
	if !m.stillMine(gen2) {
		t.Error("新循环(第二代)应该继续跑")
	}
	if gen1 == gen2 {
		t.Error("两次 Start 的代际号必须不同")
	}
}
