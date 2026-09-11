package handlers

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/ovh-webui/server/internal/types"
)

// TestShortID_RoundTripAndRestartSimulation 测试正常生成解析，以及模拟进程重启（内存缓存清空）后从 SQLite 恢复
func TestShortID_RoundTripAndRestartSimulation(t *testing.T) {
	state := newTestState(t)

	testUUID := uuid.NewString() // 完整 36 位 UUID
	short := rememberShort(state, testUUID, "test")
	if short == "" {
		t.Fatalf("expected non-empty short id")
	}
	if len(short) < 8 {
		t.Fatalf("expected short id length >= 8, got %s", short)
	}

	// 1. 内存层命中
	resolved := resolveShort(state, short)
	if resolved != testUUID {
		t.Fatalf("expected %s, got %s", testUUID, resolved)
	}

	// 2. 模拟重启：清空内存 Map
	shortMu.Lock()
	shortToID = map[string]string{}
	shortMu.Unlock()

	// 3. 重启后从 SQLite 读取
	resolvedAfterRestart := resolveShort(state, short)
	if resolvedAfterRestart != testUUID {
		t.Fatalf("expected %s after restart simulation, got %s", testUUID, resolvedAfterRestart)
	}
}

// TestShortID_CollisionAutoExtension 测试两不同 ID 在前缀相同时自动伸展短 ID 长度，避免碰撞覆盖
func TestShortID_CollisionAutoExtension(t *testing.T) {
	state := newTestState(t)

	id1 := "abcdef1234567890abcdef1234567890"
	id2 := "abcdef1299999999abcdef9999999999"

	short1 := rememberShort(state, id1, "test")
	short2 := rememberShort(state, id2, "test")

	if short1 == short2 {
		t.Fatalf("expected different short IDs for collision case, got %s and %s", short1, short2)
	}

	if resolveShort(state, short1) != id1 {
		t.Fatalf("failed to resolve id1")
	}
	if resolveShort(state, short2) != id2 {
		t.Fatalf("failed to resolve id2")
	}
}

// TestShortID_FallbackHeuristics 测试若 Short ID 丢失，业务前缀（账户与任务）智能回溯
func TestShortID_FallbackHeuristics(t *testing.T) {
	state := newTestState(t)

	accID := "acc-11223344-5566-7788-9900-aabbccddeeff"
	state.AccountsMu.Lock()
	state.Accounts = []types.OVHAccount{
		{ID: accID, Name: "Test Account"},
	}
	state.AccountsMu.Unlock()

	taskID := "task-99887766-5544-3322-1100-ffeeddccbbaa"
	state.QueueMu.Lock()
	state.Queue = []types.QueueItem{
		{ID: taskID, PlanCode: "24ska01"},
	}
	state.QueueMu.Unlock()

	// 清空内存和 DB，模拟极端丢失情况
	shortMu.Lock()
	shortToID = map[string]string{}
	shortMu.Unlock()

	// 使用前 8 位无连字符字符串作为 short id 进行回溯
	accShort := "acc11223"
	resolvedAcc := resolveShort(state, accShort)
	if resolvedAcc != accID {
		t.Fatalf("expected heuristic account resolution to match %s, got %s", accID, resolvedAcc)
	}

	taskShort := "task9988"
	resolvedTask := resolveShort(state, taskShort)
	if resolvedTask != taskID {
		t.Fatalf("expected heuristic task resolution to match %s, got %s", taskID, resolvedTask)
	}
}

// TestShortID_CallbackLengthWithinTelegramLimit 测试所有按钮生成的 callback_data 均严格小于 64 字节
func TestShortID_CallbackLengthWithinTelegramLimit(t *testing.T) {
	state := newTestState(t)

	btnID := uuid.NewString()
	accID := uuid.NewString()
	taskID := uuid.NewString()

	btnShort := rememberShort(state, btnID, "btn")
	accShort := rememberShort(state, accID, "acc")
	taskShort := rememberShort(state, taskID, "task")

	cbAccountPick := "i:C:" + btnShort + ":" + accShort
	cbTaskCancel := "i:T:one:" + taskShort
	cbTaskStop := "i:Tk:" + taskShort
	cbAccountSwitch := "i:S:" + accShort

	for name, data := range map[string]string{
		"AccountPick":   cbAccountPick,
		"TaskCancel":    cbTaskCancel,
		"TaskStop":      cbTaskStop,
		"AccountSwitch": cbAccountSwitch,
	} {
		if len(data) > 64 {
			t.Fatalf("Telegram callback_data '%s' exceeds 64 bytes: len=%d, data=%s", name, len(data), data)
		}
		if !strings.HasPrefix(data, "i:") {
			t.Fatalf("invalid prefix for %s: %s", name, data)
		}
	}
}
