package telegram

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func newTelegramRegressionState(t *testing.T) *app.State {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	lg := logger.New(filepath.Join(dir, "t.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
}

func TestRegression_BUG_BE_11_ControlCharTokenNoPanicAndPollerRestart(t *testing.T) {
	st := newTelegramRegressionState(t)
	st.Config.Set(types.Config{
		TgToken:  "123456:ABCDEF\n",
		TgChatID: "99999",
	})

	// Part 1: SendReply & AnswerCallback must NOT panic when TgToken contains control characters
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("SendReply panicked with %v", rec)
			}
		}()
		SendReply(st, "99999", "hello", 1)
	}()
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("AnswerCallback panicked with %v", rec)
			}
		}()
		AnswerCallback(st, "cb-1", "ok", false)
	}()

	// Part 2: StopPoller must reset pollerStarted so subsequent StartPoller starts a fresh loop
	st2 := newTelegramRegressionState(t)
	st2.Config.Set(types.Config{TgToken: ""})

	pollerMu.Lock()
	origStarted := pollerStarted
	pollerStarted = false
	pollerMu.Unlock()
	defer func() {
		StopPoller(200 * time.Millisecond)
		pollerMu.Lock()
		pollerStarted = origStarted
		pollerMu.Unlock()
	}()

	StartPoller(st2, func(map[string]interface{}) {})
	StopPoller(500 * time.Millisecond)

	pollerMu.Lock()
	startedAfterStop := pollerStarted
	pollerMu.Unlock()
	if startedAfterStop {
		t.Fatal("expected pollerStarted to be reset to false after StopPoller")
	}

	StartPoller(st2, func(map[string]interface{}) {})
	pollerMu.Lock()
	restarted := pollerStarted
	stopCh := pollerStop
	pollerMu.Unlock()
	if !restarted || stopCh == nil {
		t.Fatalf("expected StartPoller to restart cleanly after StopPoller: restarted=%v stopCh=%v", restarted, stopCh)
	}
	select {
	case <-stopCh:
		t.Fatal("expected restarted pollerStop channel to be open, got closed")
	default:
	}
}
