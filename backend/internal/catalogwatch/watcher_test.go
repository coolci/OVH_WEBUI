package catalogwatch

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

func fixture(t *testing.T) (*Watcher, *app.State) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	s := app.NewState(storage.Paths{DataDir: dir}, config.New(database), logger.New(filepath.Join(dir, "test.log"), nil), database)
	s.Accounts = []types.OVHAccount{{ID: "a", Name: "Default A", Endpoint: "ovh-eu", Zone: "IE", IsDefault: true}, {ID: "b", Name: "B", Endpoint: "ovh-us", Zone: "US"}}
	return New(s), s
}
func snapshot(codes ...string) map[string]Candidate {
	m := map[string]Candidate{}
	for _, code := range codes {
		c := Candidate{PlanCode: code, Options: []string{"ram-64"}, Datacenters: []string{"gra"}}
		m[key("", c)] = c
	}
	return m
}
func ready(t *testing.T, w *Watcher) {
	t.Helper()
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) { return snapshot("old"), nil }
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) {
		return snapshot("old", "new"), nil
	}
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func sole(t *testing.T, w *Watcher) *Job {
	t.Helper()
	if len(w.data.Jobs) != 1 {
		t.Fatalf("jobs=%d", len(w.data.Jobs))
	}
	for _, j := range w.data.Jobs {
		return j
	}
	return nil
}
func TestDefaultEnabledBaselineAndDefaultAccountPinning(t *testing.T) {
	w, s := fixture(t)
	if !w.data.Enabled {
		t.Fatal("must default on")
	}
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) { return snapshot("old"), nil }
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(w.data.Jobs) != 0 || w.data.BaselineAt == "" {
		t.Fatal("initial snapshot must not order")
	}
	s.Accounts[0].IsDefault = false
	s.Accounts[1].IsDefault = true
	w.fetch = func(_ context.Context, _ *app.State, sub string) (map[string]Candidate, error) {
		if sub != "IE" {
			t.Fatal("account drift")
		}
		return snapshot("old", "new"), nil
	}
	_ = w.Poll(context.Background())
	if sole(t, w).AccountID != "a" {
		t.Fatal("default change must not switch buyer")
	}
}
func TestRetrySameLocationStopAfterOneSuccessAndNeverPay(t *testing.T) {
	w, _ := fixture(t)
	ready(t, w)
	calls := []string{}
	w.buy = func(_ context.Context, _ *app.State, item *types.QueueItem) purchase.Outcome {
		if item.AutoPay || item.AccountID != "a" || len(item.Options) != 1 {
			t.Fatal("wrong purchase scope")
		}
		calls = append(calls, item.Datacenter)
		return purchase.Outcome{Success: len(calls) == 2}
	}
	w.Attempt(context.Background())
	sole(t, w).Due = 0
	w.Attempt(context.Background())
	w.Attempt(context.Background())
	if len(calls) != 2 || calls[0] != "gra" || calls[1] != "gra" || sole(t, w).Status != "completed" {
		t.Fatalf("calls=%v job=%+v", calls, sole(t, w))
	}
}
func TestDisabledSurvivesRestartAndReenableMakesFreshBaseline(t *testing.T) {
	w, s := fixture(t)
	ready(t, w)
	if err := w.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	restored := New(s)
	if restored.data.Enabled {
		t.Fatal("off state lost")
	}
	if err := restored.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	restored.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) {
		return snapshot("old", "new", "while-off"), nil
	}
	_ = restored.Poll(context.Background())
	if len(restored.data.Jobs) != 1 || sole(t, restored).Status != "cancelled" {
		t.Fatal("reenable ordered baseline inventory")
	}
}
func TestUncertainAndInterruptedAttemptsNeverResubmit(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(fmt.Sprint(crash), func(t *testing.T) {
			w, s := fixture(t)
			ready(t, w)
			if crash {
				sole(t, w).Status = "attempting"
				_ = w.save()
			} else {
				w.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
					return purchase.Outcome{PauseTask: true}
				}
				w.Attempt(context.Background())
			}
			restored := New(s)
			var calls int
			restored.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
				calls++
				return purchase.Outcome{Success: true}
			}
			restored.Attempt(context.Background())
			if calls != 0 || sole(t, restored).Status != "uncertain" {
				t.Fatal("uncertain order retried")
			}
		})
	}
}
func TestDisableCancelsInflightAndBlocksReenableUntilSettled(t *testing.T) {
	w, _ := fixture(t)
	ready(t, w)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	w.buy = func(ctx context.Context, _ *app.State, _ *types.QueueItem) purchase.Outcome {
		close(started)
		<-ctx.Done()
		<-release
		return purchase.Outcome{Cancelled: true}
	}
	go func() { w.Attempt(context.Background()); close(done) }()
	<-started
	if err := w.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if err := w.SetEnabled(context.Background(), true); err == nil {
		t.Fatal("reenabled before settlement")
	}
	close(release)
	<-done
	if sole(t, w).Status != "cancelled" {
		t.Fatal("attempt not cancelled")
	}
}
func TestNoConcurrentAttemptsForSameConfiguration(t *testing.T) {
	w, _ := fixture(t)
	ready(t, w)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	var calls atomic.Int32
	w.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
		calls.Add(1)
		close(started)
		<-release
		return purchase.Outcome{Success: true}
	}
	go func() { w.Attempt(context.Background()); close(done) }()
	<-started
	w.Attempt(context.Background())
	close(release)
	<-done
	if calls.Load() != 1 {
		t.Fatal("multiple in-flight orders")
	}
}
func TestPollFailureOrDisableDuringFetchCannotCreateOrders(t *testing.T) {
	w, _ := fixture(t)
	ready(t, w)
	n := len(w.data.Baseline)
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) {
		return nil, fmt.Errorf("offline")
	}
	if w.Poll(context.Background()) == nil || len(w.data.Baseline) != n {
		t.Fatal("bad baseline")
	}
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) {
		close(started)
		<-release
		return snapshot("late"), nil
	}
	go func() { _ = w.Poll(context.Background()); close(done) }()
	<-started
	_ = w.SetEnabled(context.Background(), false)
	close(release)
	<-done
	if len(w.data.Jobs) != 1 {
		t.Fatal("late snapshot created jobs after off")
	}
}
func TestExistingCacheAndMissingAccountRemainIndependent(t *testing.T) {
	w, s := fixture(t)
	stamp := time.Now().Add(-time.Hour)
	s.ServerCache.SetBucketAt("", []types.ServerPlan{{PlanCode: "cached"}}, stamp)
	ready(t, w)
	if !s.ServerCache.TimestampOf("").Equal(stamp) || s.ServerCache.TTL != 120*time.Second {
		t.Fatal("changed list cache")
	}
	s.Accounts = nil
	w.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
		t.Fatal("deleted account purchased")
		return purchase.Outcome{}
	}
	w.Attempt(context.Background())
}
func TestPersistenceFailureStopsBeforePurchase(t *testing.T) {
	w, s := fixture(t)
	ready(t, w)
	_, _ = s.DB.Exec("DROP TABLE catalog_watch")
	w.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
		t.Fatal("purchase before durable claim")
		return purchase.Outcome{}
	}
	w.Attempt(context.Background())
	if w.data.Enabled || w.loadErr == nil {
		t.Fatal("did not stop on save failure")
	}
}
func TestNewConfigOnlyAndReappearanceDedup(t *testing.T) {
	w, _ := fixture(t)
	ready(t, w)
	candidate := Candidate{PlanCode: "old", Options: []string{"ram-128"}, Datacenters: []string{"gra"}}
	current := snapshot("old", "new")
	current[key("", candidate)] = candidate
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) { return current, nil }
	_ = w.Poll(context.Background())
	if len(w.data.Jobs) != 2 {
		t.Fatal("missing added configuration")
	}
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) { return snapshot("old"), nil }
	_ = w.Poll(context.Background())
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) { return current, nil }
	_ = w.Poll(context.Background())
	if len(w.data.Jobs) != 2 {
		t.Fatal("duplicate reappearance")
	}
}

func TestGracefulShutdownCanResumeUnsubmittedAttempt(t *testing.T) {
	w, s := fixture(t)
	ready(t, w)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan struct{})
	w.buy = func(ctx context.Context, _ *app.State, _ *types.QueueItem) purchase.Outcome {
		close(started)
		<-ctx.Done()
		return purchase.Outcome{Cancelled: true}
	}
	go func() { w.Attempt(ctx); close(done) }()
	<-started
	cancel()
	<-done
	if sole(t, New(s)).Status != "pending" {
		t.Fatal("shutdown lost unsubmitted job")
	}
}
func TestCheckoutSuccessAfterSwitchOffRemainsCompleted(t *testing.T) {
	w, _ := fixture(t)
	ready(t, w)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	w.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
		close(started)
		<-release
		return purchase.Outcome{Success: true}
	}
	go func() { w.Attempt(context.Background()); close(done) }()
	<-started
	_ = w.SetEnabled(context.Background(), false)
	close(release)
	<-done
	if sole(t, w).Status != "completed" || w.data.Enabled {
		t.Fatal("lost submitted checkout result after off")
	}
}
