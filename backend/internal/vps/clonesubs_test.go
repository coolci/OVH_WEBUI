package vps

import (
	"sync"
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// 检查循环以前用 copy() 拍快照 —— 结构体复制了,但 LastStatus 这个 map
// 还是同一个底层对象。循环在锁外往里写,SaveSubscriptions 同时在序列化它。
// Go 遇到 map 并发读写不是 data race 那么客气,是 fatal error 直接把进程打死,
// recover 也拦不住:抢购工具半夜静默退出,就没有然后了。
func TestCloneSubsDoesNotShareMaps(t *testing.T) {
	src := []types.VPSSubscription{{
		ID:          "s1",
		PlanCode:    "vps-2025-model1",
		LastStatus:  map[string]string{"gra": "available"},
		Datacenters: []string{"gra", "sbg"},
		History:     []map[string]interface{}{{"k": "v"}},
	}}

	out := cloneSubs(src)

	// 改快照不能影响原件
	out[0].LastStatus["gra"] = "unavailable"
	out[0].LastStatus["rbx"] = "available"
	out[0].Datacenters[0] = "xxx"
	if src[0].LastStatus["gra"] != "available" {
		t.Fatal("改快照的 LastStatus 影响到了原件 —— 还是同一个 map")
	}
	if _, ok := src[0].LastStatus["rbx"]; ok {
		t.Fatal("往快照里加 key 影响到了原件")
	}
	if src[0].Datacenters[0] != "gra" {
		t.Fatal("改快照的 Datacenters 影响到了原件 —— 还是同一个底层数组")
	}
	// 反向也要独立
	src[0].LastStatus["sbg"] = "available"
	if _, ok := out[0].LastStatus["sbg"]; ok {
		t.Fatal("改原件影响到了快照")
	}
}

// 真正的形状:一边拿快照写状态,一边序列化原件。修复前这里会 fatal。
func TestCloneSubsConcurrentWriteAndRead(t *testing.T) {
	live := []types.VPSSubscription{{
		ID: "s1", PlanCode: "vps-2025-model1",
		LastStatus: map[string]string{"gra": "unavailable"},
	}}
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			// 检查循环:锁内拍快照,锁外改
			mu.Lock()
			snap := cloneSubs(live)
			mu.Unlock()
			snap[0].LastStatus["gra"] = "available"
			snap[0].LastStatus["dc"] = "x"
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			// 保存侧:锁内拍快照,锁外遍历(相当于 JSON 序列化)
			mu.Lock()
			snap := cloneSubs(live)
			mu.Unlock()
			total := 0
			for range snap[0].LastStatus {
				total++
			}
			_ = total
		}()
	}
	wg.Wait()
}
