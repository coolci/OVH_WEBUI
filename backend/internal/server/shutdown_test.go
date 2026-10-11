package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/logger"
)

type closeSpy struct {
	closed bool
	err    error
}

func (c *closeSpy) Close() error { c.closed = true; return c.err }

type stopSpy struct {
	stopped bool
}

func (s *stopSpy) Stop() { s.stopped = true }

func TestGracefulShutdownFlushesLogsAndClosesDB(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log.json")
	lg := logger.New(logPath, slog.New(slog.NewTextHandler(io.Discard, nil)))

	lg.Info("抢购任务 t1 开始", "queue")
	lg.Info("抢购任务 t1 无货,等待下一轮", "queue")

	db := &closeSpy{}
	sup := &stopSpy{}

	GracefulShutdown(nil, db, sup, lg, nil, "terminated", time.Second)

	if !db.closed {
		t.Error("数据库没有被关闭")
	}
	if !sup.stopped {
		t.Error("后台协程没有被停止")
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("日志没有落盘: %v", err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("日志文件不是合法 JSON: %v", err)
	}
	joined := string(raw)
	for _, want := range []string{"抢购任务 t1 开始", "正在优雅退出", "正在关闭数据库"} {
		if !strings.Contains(joined, want) {
			t.Errorf("日志里缺少 %q", want)
		}
	}
}

func TestGracefulShutdownRecordsCloseFailure(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log.json")
	lg := logger.New(logPath, slog.New(slog.NewTextHandler(io.Discard, nil)))

	db := &closeSpy{err: errors.New("database is locked")}
	GracefulShutdown(nil, db, nil, lg, nil, "interrupt", time.Second)

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("日志没有落盘: %v", err)
	}
	if !strings.Contains(string(raw), "database is locked") {
		t.Error("关库失败没有记进日志")
	}
}
