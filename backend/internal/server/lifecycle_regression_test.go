package server

import (
	"context"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func lifecycleTestState(t *testing.T) *app.State {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	lg := logger.New(filepath.Join(dir, "logs.json"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return app.NewState(storage.Paths{}, config.New(database), lg, database)
}

func TestRunWaitsForRequestDrain(t *testing.T) {
	s := lifecycleTestState(t)
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := reservation.Addr().String()
	_ = reservation.Close()
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("finished"))
	})
	srv := New(Config{Addr: addr, Handler: handler, State: s, EnableAuth: true, Console: slog.New(slog.NewTextHandler(io.Discard, nil))})
	runDone := make(chan error, 1)
	go func() { runDone <- srv.Run() }()
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		client := &http.Client{Timeout: 3 * time.Second}
		for i := 0; i < 100; i++ {
			resp, e := client.Get("http://" + addr + "/audit")
			if e == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("local handler did not start")
	}
	shutdownDone := make(chan struct{})
	srv.shutdownPending.Store(true)
	go func() {
		defer close(shutdownDone)
		defer close(srv.shutdownDone)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.httpServer.Shutdown(ctx)
	}()
	select {
	case err := <-runDone:
		close(release)
		t.Fatalf("Run returned before draining: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	<-shutdownDone
	<-requestDone
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run failed to return after drain")
	}
}
