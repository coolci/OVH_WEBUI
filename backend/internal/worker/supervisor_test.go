package worker

import (
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/storage"
)

func TestSupervisorStopIdempotent(t *testing.T) {
	st := &app.State{
		Paths: storage.DefaultPaths(),
	}
	sup := NewSupervisor(st, nil)
	// Stop should be safe to call multiple times without panicking
	sup.Stop()
	sup.Stop()
}

func TestApplySharedProxyNilSafe(t *testing.T) {
	// Should not panic on nil state
	ApplySharedProxy(nil)

	st := &app.State{}
	ApplySharedProxy(st)
}
