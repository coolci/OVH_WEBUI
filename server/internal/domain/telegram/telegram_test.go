package telegram_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/domain/telegram"
)

func TestNewSession(t *testing.T) {
	now := time.Now()
	_, err := telegram.NewSession(0, "start", now, 10*time.Minute)
	if err == nil {
		t.Fatalf("expected error for zero chat ID")
	}

	sess, err := telegram.NewSession(123456, "start", now, 10*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ChatID != 123456 {
		t.Errorf("expected chat ID 123456, got %d", sess.ChatID)
	}
	if sess.IsExpired(now.Add(5 * time.Minute)) {
		t.Errorf("session should not be expired after 5 minutes")
	}
	if !sess.IsExpired(now.Add(11 * time.Minute)) {
		t.Errorf("session should be expired after 11 minutes")
	}

	err = sess.SetStep("dc_select", now.Add(5*time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatalf("failed to transition step: %v", err)
	}
	if sess.Step != "dc_select" {
		t.Errorf("expected step dc_select, got %s", sess.Step)
	}
}

func TestNewCommand(t *testing.T) {
	cmd, err := telegram.NewCommand("start", "Start the bot", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Name != "/start" {
		t.Errorf("expected command /start, got %s", cmd.Name)
	}

	_, err = telegram.NewCommand("bad command!", "invalid", false)
	if err == nil {
		t.Fatalf("expected error for command with spaces and punctuation")
	}

	_, err = telegram.NewCommand("", "empty", false)
	if err == nil {
		t.Fatalf("expected error for empty command")
	}
}

func TestTruncateMessage(t *testing.T) {
	short := "Hello world"
	if telegram.TruncateMessage(short) != short {
		t.Errorf("expected short message unchanged")
	}

	long := strings.Repeat("A", 5000)
	truncated := telegram.TruncateMessage(long)
	if len(truncated) != telegram.MaxMessageLength {
		t.Errorf("expected length %d, got %d", telegram.MaxMessageLength, len(truncated))
	}
	if !strings.HasSuffix(truncated, "...") {
		t.Errorf("expected ellipsis suffix")
	}
}
