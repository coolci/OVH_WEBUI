package catalogwatch

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/purchase"
	"github.com/ovh-webui/server/internal/types"
)

func prepareLocations(t *testing.T, w *Watcher, plans, options, locations []string) map[string]Candidate {
	t.Helper()
	current := snapshot("baseline")
	w.fetch = func(context.Context, *app.State, string) (map[string]Candidate, error) { return current, nil }
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	current = snapshot("baseline")
	for _, p := range plans {
		for _, o := range options {
			c := Candidate{PlanCode: p, Options: []string{o}, Datacenters: locations}
			current[key("", c)] = c
		}
	}
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	return current
}

func TestPlanOptionsLocationsProductAndRestartDedup(t *testing.T) {
	w, s := fixture(t)
	prepareLocations(t, w, []string{"p1", "p2"}, []string{"ram-16", "ram-32"}, []string{"gra", "waw"})
	if len(w.data.Jobs) != 8 {
		t.Fatalf("want 2*2*2=8, got %d", len(w.data.Jobs))
	}
	calls := map[string]int{}
	buy := func(_ context.Context, _ *app.State, i *types.QueueItem) purchase.Outcome {
		if i.AutoPay {
			t.Fatal("autopay")
		}
		k := i.PlanCode + "/" + i.Options[0] + "/" + i.Datacenter
		calls[k]++
		return purchase.Outcome{Success: true}
	}
	w.buy = buy
	for i := 0; i < 12; i++ {
		w.Attempt(context.Background())
	}
	if len(calls) != 8 {
		t.Fatalf("bought %d combinations", len(calls))
	}
	for k, n := range calls {
		if n != 1 {
			t.Fatalf("duplicate %s", k)
		}
	}
	restored := New(s)
	restored.buy = buy
	for i := 0; i < 12; i++ {
		restored.Attempt(context.Background())
	}
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	for k, n := range calls {
		if n != 1 {
			t.Fatalf("duplicate after restart %s", k)
		}
	}
	for _, j := range restored.data.Jobs {
		if j.Status != "completed" {
			t.Fatal("lost completion")
		}
	}
}

func TestLocationsRunConcurrentlyAndEachOnlyOnce(t *testing.T) {
	w, _ := fixture(t)
	prepareLocations(t, w, []string{"p"}, []string{"ram"}, []string{"gra", "waw", "bhs"})
	started := make(chan string, 3)
	release := make(chan struct{})
	var wg sync.WaitGroup
	w.buy = func(_ context.Context, _ *app.State, i *types.QueueItem) purchase.Outcome {
		started <- i.Datacenter
		<-release
		return purchase.Outcome{Success: true}
	}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); w.Attempt(context.Background()) }()
	}
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		select {
		case dc := <-started:
			if seen[dc] {
				t.Fatal("duplicate location")
			}
			seen[dc] = true
		case <-time.After(3 * time.Second):
			close(release)
			wg.Wait()
			t.Fatal("locations serialized")
		}
	}
	// All three are in flight: a fourth attempt cannot duplicate any claim.
	w.Attempt(context.Background())
	close(release)
	wg.Wait()
	if len(w.active) != 0 {
		t.Fatal("active claim leaked")
	}
}

func TestAddedLocationAndReappearance(t *testing.T) {
	w, _ := fixture(t)
	current := prepareLocations(t, w, []string{"p"}, []string{"ram"}, []string{"gra"})
	c := Candidate{PlanCode: "p", Options: []string{"ram"}, Datacenters: []string{"gra", "waw"}}
	current[key("", c)] = c
	if err := w.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(w.data.Jobs) != 2 {
		t.Fatal("new location missing")
	}
	c.Datacenters = []string{"gra"}
	current[key("", c)] = c
	_ = w.Poll(context.Background())
	c.Datacenters = []string{"gra", "waw"}
	current[key("", c)] = c
	_ = w.Poll(context.Background())
	if len(w.data.Jobs) != 2 {
		t.Fatal("location reappearance duplicated")
	}
}

func TestLegacyMigrationNeverReplaysSuccessOrUncertain(t *testing.T) {
	for _, status := range []string{"pending", "completed", "attempting", "uncertain", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			w, s := fixture(t)
			ready(t, w)
			j := sole(t, w)
			j.PerDatacenter = false
			j.Datacenters = []string{"gra", "waw"}
			j.Status = status
			if err := w.save(); err != nil {
				t.Fatal(err)
			}
			restored := New(s)
			if len(restored.data.Jobs) != 2 {
				t.Fatal("migration lost locations")
			}
			calls := 0
			restored.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
				calls++
				return purchase.Outcome{Success: true}
			}
			restored.Attempt(context.Background())
			restored.Attempt(context.Background())
			want := 0
			if status == "pending" {
				want = 2
			}
			if calls != want {
				t.Fatalf("calls %d want %d", calls, want)
			}
		})
	}
}

func TestConcurrencyBoundAndDisableCancelsAll(t *testing.T) {
	w, _ := fixture(t)
	plans := []string{}
	for i := 0; i < maxConcurrentAttempts+1; i++ {
		plans = append(plans, fmt.Sprintf("p-%03d", i))
	}
	prepareLocations(t, w, plans, []string{"ram"}, []string{"gra"})
	started := make(chan struct{}, maxConcurrentAttempts+1)
	w.buy = func(ctx context.Context, _ *app.State, _ *types.QueueItem) purchase.Outcome {
		started <- struct{}{}
		<-ctx.Done()
		return purchase.Outcome{Cancelled: true}
	}
	var wg sync.WaitGroup
	for i := 0; i < maxConcurrentAttempts; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); w.Attempt(context.Background()) }()
	}
	for i := 0; i < maxConcurrentAttempts; i++ {
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			_ = w.SetEnabled(context.Background(), false)
			wg.Wait()
			t.Fatal("worker did not start")
		}
	}
	w.Attempt(context.Background())
	select {
	case <-started:
		t.Fatal("exceeded concurrency bound")
	default:
	}
	if err := w.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	for _, j := range w.data.Jobs {
		if j.Status != "cancelled" {
			t.Fatalf("job not cancelled: %s", j.Status)
		}
	}
	if len(w.active) != 0 {
		t.Fatal("inflight claim leaked")
	}
}

func TestUnsupportedAndSlowLocationsDoNotBlockHealthyLocations(t *testing.T) {
	w, _ := fixture(t)
	prepareLocations(t, w, []string{"p"}, []string{"ram"}, []string{"bad", "slow", "good"})
	entered := make(chan string, 3)
	release := make(chan struct{})
	var wg sync.WaitGroup
	w.buy = func(ctx context.Context, _ *app.State, item *types.QueueItem) purchase.Outcome {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > attemptTimeout {
			t.Error("missing attempt deadline")
		}
		entered <- item.Datacenter
		switch item.Datacenter {
		case "bad":
			return purchase.Outcome{Fatal: true, Reason: "unsupported datacenter"}
		case "slow":
			<-release
			return purchase.Outcome{}
		default:
			return purchase.Outcome{Success: true}
		}
	}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); w.Attempt(context.Background()) }()
	}
	for i := 0; i < 3; i++ {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			close(release)
			wg.Wait()
			t.Fatal("slow location blocked others")
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		w.mu.Lock()
		good, bad := false, false
		for _, j := range w.data.Jobs {
			good = good || (j.Datacenters[0] == "good" && j.Status == "completed")
			bad = bad || (j.Datacenters[0] == "bad" && j.Status == "failed")
		}
		w.mu.Unlock()
		if good && bad {
			break
		}
		if time.Now().After(deadline) {
			close(release)
			wg.Wait()
			t.Fatal("healthy location did not complete independently")
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	wg.Wait()
}

func TestDeadlineReleasesClaimAndPreservesCheckoutOutcome(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		out          purchase.Outcome
	}{
		{"before submission", "pending", purchase.Outcome{Cancelled: true}},
		{"checkout uncertain", "uncertain", purchase.Outcome{PauseTask: true}},
		{"checkout completed", "completed", purchase.Outcome{Success: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := fixture(t)
			ready(t, w)
			w.buy = func(ctx context.Context, _ *app.State, _ *types.QueueItem) purchase.Outcome {
				<-ctx.Done()
				return tc.out
			}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
			defer cancel()
			w.Attempt(ctx)
			if len(w.active) != 0 || sole(t, w).Status != tc.status {
				t.Fatalf("deadline outcome lost: %+v", sole(t, w))
			}
			w.buy = func(context.Context, *app.State, *types.QueueItem) purchase.Outcome {
				t.Fatal("premature/duplicate retry")
				return purchase.Outcome{}
			}
			w.Attempt(context.Background())
		})
	}
}
