package monitor

import "testing"

// 代际号:停止后立刻启动,旧循环必须退出。
//
// 以前 Start/Stop 只翻 running 布尔,也不等旧 goroutine 退出:
// Start → loop1 进入等待 → Stop(running=false) → 立刻 Start(running=true)
// → loop1 下一个检查点看到 running 又是 true,不退出 → 两个循环并存。
// 后果是同一次补货被判定两次、下两次单(skipDuplicateCheck 已关掉去重),
// 以及 LastStatus 互相覆盖。
//
// VPS 侧界面就一个「停止/启动」按钮,点两下即可复现。
func TestGeneration_旧循环在重启后失效(t *testing.T) {
	m := &Monitor{}

	// 第一代
	m.subsMu.Lock()
	m.running = true
	m.generation++
	gen1 := m.generation
	m.subsMu.Unlock()

	if !m.stillMine(gen1) {
		t.Fatal("第一代刚启动就应该是当前代")
	}

	// 停止
	m.subsMu.Lock()
	m.running = false
	m.subsMu.Unlock()
	if m.stillMine(gen1) {
		t.Error("已停止时不该继续跑")
	}

	// 立刻再启动 = 第二代
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
