package app

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func newTestState(t *testing.T) *State {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	lg := logger.New(filepath.Join(dir, "t.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
}

func TestSaveHistoryConcurrentLastWriteWins(t *testing.T) {
	s := newTestState(t)

	var wg sync.WaitGroup
	const n = 60
	for i := 0; i < n; i++ {
		s.HistoryMu.Lock()
		s.History = append(s.History, types.PurchaseHistoryEntry{
			ID: fmt.Sprintf("id-%03d", i), TaskID: fmt.Sprintf("task-%03d", i),
			PlanCode: "24sk602", Status: "success", PurchaseTime: types.NowISO(),
		})
		s.HistoryMu.Unlock()
		wg.Add(1)
		go func() { defer wg.Done(); _ = s.SaveHistory() }()
	}
	wg.Wait()

	got, err := s.DB.ListHistory()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	s.HistoryMu.Lock()
	want := len(s.History)
	s.HistoryMu.Unlock()

	t.Logf("内存 %d 条,库里 %d 条", want, len(got))
	if len(got) != want {
		t.Errorf("并发保存后数据不一致:内存 %d 条,库里只有 %d 条 —— "+
			"晚拍的快照被早拍的覆盖了(丢 %d 条抢购历史)", want, len(got), want-len(got))
	}
}

func TestSaveQueueConcurrentLastWriteWins(t *testing.T) {
	s := newTestState(t)

	var wg sync.WaitGroup
	const n = 60
	for i := 0; i < n; i++ {
		s.QueueMu.Lock()
		s.Queue = append(s.Queue, types.QueueItem{
			ID: fmt.Sprintf("q-%03d", i), AccountID: "acc", PlanCode: "24sk602",
			Datacenter: "gra", Status: "running", CreatedAt: types.NowISO(),
			UpdatedAt: types.NowISO(), RetryInterval: 60,
		})
		s.QueueMu.Unlock()
		wg.Add(1)
		go func() { defer wg.Done(); _ = s.SaveQueue() }()
	}
	wg.Wait()

	got, err := s.DB.ListQueue()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	s.QueueMu.Lock()
	want := len(s.Queue)
	s.QueueMu.Unlock()
	if len(got) != want {
		t.Errorf("并发保存后队列不一致:内存 %d 条,库里 %d 条(丢了 %d 个抢购任务)",
			want, len(got), want-len(got))
	}
}
