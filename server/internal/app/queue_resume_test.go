package app

import (
	"errors"
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// 移植自上游 v0.4.14:恢复落库失败时,任务不能在内存里变成可运行,停止记录也不能被解除。
func TestResumeFailureCannotPublishRunnableTask(t *testing.T) {
	state := enqueueTestState(t)
	state.Queue = []types.QueueItem{{ID: "resume", PlanCode: "plan", Datacenter: "gra", Status: "paused"}}
	if err := state.SaveQueue(); err != nil {
		t.Fatal(err)
	}
	state.QueueMu.Lock()
	err := state.SetAutomaticOrderBlockedLocked(state.Queue[0], true)
	state.QueueMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.DB.Exec(`CREATE TRIGGER reject_queue_resume BEFORE UPDATE ON queue BEGIN SELECT RAISE(FAIL, 'fixture write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ResumeQueueItem("resume", "running"); err == nil {
		t.Fatal("failed resume reported success")
	}
	if state.Queue[0].Status != "paused" {
		t.Fatal("unsaved task became runnable")
	}
	rows, err := state.DB.ListQueue()
	if err != nil || len(rows) != 1 || rows[0].Status != "paused" {
		t.Fatal(rows, err)
	}
	var blocked bool
	if found, err := state.DB.GetKV(state.automaticOrderKey(state.Queue[0]), &blocked); err != nil || !found || !blocked {
		t.Fatal("failed resume unblocked automatic orders", err)
	}
}

// 成功恢复:状态与许可一起落库,失败计数和重试计时清零;已下单的任务拒绝恢复。
func TestResumePersistsAndResetsRetryState(t *testing.T) {
	state := enqueueTestState(t)
	state.Queue = []types.QueueItem{
		{ID: "resume", PlanCode: "plan", Datacenter: "gra", Status: "failed", FailureCount: 5, LastCheckTime: 12345},
		{ID: "done", PlanCode: "plan", Datacenter: "rbx", Status: "completed"},
	}
	if err := state.SaveQueue(); err != nil {
		t.Fatal(err)
	}
	state.QueueMu.Lock()
	_ = state.SetAutomaticOrderBlockedLocked(state.Queue[0], true)
	state.QueueMu.Unlock()
	got, err := state.ResumeQueueItem("resume", "running")
	if err != nil || got.Status != "running" || got.FailureCount != 0 || got.LastCheckTime != 0 {
		t.Fatalf("resume result: %+v %v", got, err)
	}
	rows, _ := state.DB.ListQueue()
	for _, r := range rows {
		if r.ID == "resume" && (r.Status != "running" || r.FailureCount != 0 || r.LastCheckTime != 0) {
			t.Fatalf("resume not persisted: %+v", r)
		}
	}
	var blocked bool
	if _, err := state.DB.GetKV(state.automaticOrderKey(state.Queue[0]), &blocked); err != nil || blocked {
		t.Fatal("resume did not lift the automatic-order stop", err)
	}
	if _, err := state.ResumeQueueItem("done", "running"); !errors.Is(err, ErrQueueAlreadyOrdered) {
		t.Fatalf("completed task resumed: %v", err)
	}
	if _, err := state.ResumeQueueItem("missing", "running"); !errors.Is(err, ErrQueueNotFound) {
		t.Fatalf("missing task: %v", err)
	}
}
