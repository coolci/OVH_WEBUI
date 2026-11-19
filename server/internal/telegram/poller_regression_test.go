package telegram

import (
	"fmt"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/storage"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type auditPollTransport func(*http.Request) (*http.Response, error)

func (f auditPollTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPollerErrorsUpdateHealthAndBackOff(t *testing.T) {
	for _, code := range []int{401, 409, 429, 503} {
		t.Run(fmt.Sprint(code), func(t *testing.T) { testPollerFailure(t, code) })
	}
}
func testPollerFailure(t *testing.T, code int) {
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	lg := logger.New(filepath.Join(dir, "audit.log"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	st := app.NewState(storage.Paths{DataDir: dir}, config.New(database), lg, database)
	cfg := st.Config.Get()
	cfg.TgToken = "123:test"
	cfg.TgChatID = "123"
	if err := st.Config.Set(cfg); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	kick := make(chan struct{})
	var requests atomic.Int32
	restore := netfp.SwapSharedTransportForTesting(auditPollTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"ok":true,"result":{"username":"auditbot"}}`
		status := 200
		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			body = `{"ok":false,"description":"fixture failure"}`
			status = code
			requests.Add(1)
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}))
	defer restore()
	patchPoller(func(s *PollerStatus) { *s = PollerStatus{} })
	done := make(chan struct{})
	go func() { defer close(done); runPoller(st, func(map[string]interface{}) {}, kick, stop) }()
	defer func() {
		close(stop)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("poller did not stop promptly")
		}
	}()
	deadline := time.Now().Add(time.Second)
	for SnapshotPoller().LastError == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	status := SnapshotPoller()
	if status.LastError == "" || status.Running {
		t.Fatalf("failure not reflected: %+v", status)
	}
	time.Sleep(100 * time.Millisecond)
	if requests.Load() != 1 {
		t.Fatalf("busy loop made %d requests", requests.Load())
	}
}
