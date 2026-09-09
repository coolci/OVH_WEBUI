package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

func TestSaveRefusedAfterLoadFailure(t *testing.T) {
	s := newTestState(t)

	msg := "boom"
	s.HistoryMu.Lock()
	for i := 0; i < 10; i++ {
		s.History = append(s.History, types.PurchaseHistoryEntry{
			ID: fmt.Sprintf("h-%02d", i), PlanCode: "24sk602", Status: "success",
			PurchaseTime: types.NowISO(), ErrorMessage: &msg,
		})
	}
	s.HistoryMu.Unlock()
	if err := s.SaveHistory(); err != nil {
		t.Fatalf("初始保存失败: %v", err)
	}
	before, err := s.DB.ListHistory()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(before) != 10 {
		t.Fatalf("准备数据失败,库里应有 10 条,实际 %d", len(before))
	}

	s.MarkLoadFailed("history", errors.New("missing column: retraction_time"))
	s.HistoryMu.Lock()
	s.History = []types.PurchaseHistoryEntry{}
	s.HistoryMu.Unlock()

	if err := s.SaveHistory(); err == nil {
		t.Fatal("读失败后 SaveHistory 应当拒绝写入并返回错误,却成功了")
	}

	after, err := s.DB.ListHistory()
	if err != nil {
		t.Fatalf("list after: %v", err)
	}
	if len(after) != 10 {
		t.Fatalf("磁盘上的历史被空内存覆盖了:保存前 10 条,现在 %d 条", len(after))
	}
}

func TestSaveStillWorksWithoutLoadFailure(t *testing.T) {
	s := newTestState(t)
	s.QueueMu.Lock()
	s.Queue = append(s.Queue, types.QueueItem{ID: "q-1", PlanCode: "24sk602", Status: "pending"})
	s.QueueMu.Unlock()
	if err := s.SaveQueue(); err != nil {
		t.Fatalf("正常保存不该被拦: %v", err)
	}
	got, err := s.DB.ListQueue()
	if err != nil {
		t.Fatalf("list queue: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 条队列,实际 %d", len(got))
	}
}

func TestLoadFailureIsPerTable(t *testing.T) {
	s := newTestState(t)
	s.MarkLoadFailed("history", errors.New("schema drift"))

	s.QueueMu.Lock()
	s.Queue = append(s.Queue, types.QueueItem{ID: "q-1", PlanCode: "24sk602", Status: "pending"})
	s.QueueMu.Unlock()
	if err := s.SaveQueue(); err != nil {
		t.Fatalf("history 读失败不该影响 queue 的保存: %v", err)
	}
	if err := s.SaveHistory(); err == nil {
		t.Fatal("history 应当仍被拦住")
	}
	if f := s.LoadFailures(); len(f) != 1 || f["history"] == "" {
		t.Fatalf("LoadFailures 应只报告 history,实际 %v", f)
	}
}
