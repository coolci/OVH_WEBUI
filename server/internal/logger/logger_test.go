package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoggerConcurrencyAndFlush(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log.json")

	l := New(logFile, nil)

	const goroutines = 20
	const logsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < logsPerGoroutine; i++ {
				l.Info(fmt.Sprintf("goroutine %d message %d", gid, i), "test")
			}
		}(g)
	}

	wg.Wait()
	l.Flush()

	entries := l.Snapshot()
	if len(entries) == 0 {
		t.Fatalf("expected log entries, got 0")
	}

	// Verify log file exists and is not empty
	stat, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("stat log file failed: %v", err)
	}
	if stat.Size() == 0 {
		t.Fatalf("log file is empty")
	}
}
