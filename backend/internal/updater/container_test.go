package updater

import "testing"

// 容器检测错了后果是双向的：
// 漏判 → 容器里给出一个点了必然失败（或者"成功"之后版本回滚）的更新按钮；
// 误判 → 裸机用户的自更新被无故停掉。
func TestInContainerEnvFlag(t *testing.T) {
	cases := map[string]bool{
		"1":     true,
		"true":  true,
		"yes":   true,
		"0":     false,
		"false": false,
		"FALSE": false,
		"":      false, // 空 = 交给别的判据
	}
	for v, want := range cases {
		t.Setenv("OVH_IN_CONTAINER", v)
		got := InContainer()
		// 空串和显式否定时会落到文件/cgroup 判据上，
		// 本机跑测试时那些都不该命中
		if want && !got {
			t.Errorf("OVH_IN_CONTAINER=%q 应当判为容器内", v)
		}
		if !want && got {
			t.Errorf("OVH_IN_CONTAINER=%q 不该判为容器内（会无故停掉裸机的自更新）", v)
		}
	}
}

// 更新指引必须把"为什么"说清楚，否则用户只会觉得功能坏了。
func TestContainerUpdateHintIsActionable(t *testing.T) {
	for _, must := range []string{"docker compose pull", "docker pull", "卷", "可写层"} {
		if !contains(ContainerUpdateHint, must) {
			t.Errorf("更新指引里缺少 %q", must)
		}
	}
}

func contains(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}
