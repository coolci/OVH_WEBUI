package vps

import (
	"sync"
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

func TestCloneSubsDoesNotShareMaps(t *testing.T) {
	src := []types.VPSSubscription{{
		ID:          "s1",
		PlanCode:    "vps-2025-model1",
		LastStatus:  map[string]string{"gra": "available"},
		Datacenters: []string{"gra", "sbg"},
		History:     []map[string]interface{}{{"k": "v"}},
	}}

	out := cloneSubs(src)

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
	src[0].LastStatus["sbg"] = "available"
	if _, ok := out[0].LastStatus["sbg"]; ok {
		t.Fatal("改原件影响到了快照")
	}
}

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
			mu.Lock()
			snap := cloneSubs(live)
			mu.Unlock()
			snap[0].LastStatus["gra"] = "available"
			snap[0].LastStatus["dc"] = "x"
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
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
