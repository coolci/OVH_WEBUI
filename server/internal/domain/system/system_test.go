package system_test

import (
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/domain/system"
)

func TestNewLogEntry(t *testing.T) {
	now := time.Now()
	_, err := system.NewLogEntry(system.LogLevelInfo, "", "test", now)
	if err == nil {
		t.Fatalf("expected error for empty log message")
	}

	_, err = system.NewLogEntry("", "something happened", "test", now)
	if err == nil {
		t.Fatalf("expected error for empty log level")
	}

	entry, err := system.NewLogEntry(system.LogLevelInfo, "server booted", "system", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Level != system.LogLevelInfo {
		t.Errorf("expected level INFO, got %s", entry.Level)
	}
	if entry.Message != "server booted" {
		t.Errorf("expected message 'server booted', got %s", entry.Message)
	}
}

func TestCheckUpdate(t *testing.T) {
	info := system.CheckUpdate("3.0.0", "3.0.1", "stable")
	if !info.UpdateAvailable {
		t.Errorf("expected update to be available")
	}

	infoSame := system.CheckUpdate("3.0.0", "3.0.0", "stable")
	if infoSame.UpdateAvailable {
		t.Errorf("expected no update when versions are equal")
	}

	infoEmpty := system.CheckUpdate("3.0.0", "", "stable")
	if infoEmpty.UpdateAvailable {
		t.Errorf("expected no update when latest version is empty")
	}
}
