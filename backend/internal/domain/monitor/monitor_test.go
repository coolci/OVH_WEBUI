package monitor_test

import (
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/domain/monitor"
)

func TestNewSubscription(t *testing.T) {
	now := time.Now()

	t.Run("valid subscription", func(t *testing.T) {
		sub, err := monitor.NewSubscription("24ska01", []string{"gra", "rbx"}, 30, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sub.Datacenters) != 2 || sub.Datacenters[0] != "GRA" || sub.Datacenters[1] != "RBX" {
			t.Errorf("expected normalized datacenters [GRA RBX], got %v", sub.Datacenters)
		}
		if !sub.Enabled {
			t.Errorf("expected enabled by default")
		}
	})

	t.Run("empty plan code error", func(t *testing.T) {
		_, err := monitor.NewSubscription("", []string{"gra"}, 30, now)
		if err != monitor.ErrEmptyPlanCode {
			t.Fatalf("expected ErrEmptyPlanCode, got %v", err)
		}
	})

	t.Run("invalid interval error", func(t *testing.T) {
		_, err := monitor.NewSubscription("24ska01", []string{"gra"}, 2, now)
		if err == nil {
			t.Fatalf("expected error for interval < 5, got nil")
		}
	})
}

func TestSubscription_UpdateAvailability(t *testing.T) {
	now := time.Now()
	sub, err := monitor.NewSubscription("24ska01", []string{"GRA", "SBG"}, 30, now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	// 1. Initial poll: unavailable everywhere -> no transition alerts
	state1 := map[string]monitor.DatacenterAvailability{
		"GRA": monitor.AvailUnavailable,
		"SBG": monitor.AvailUnavailable,
		"RBX": monitor.AvailUnavailable,
	}
	alerted := sub.UpdateAvailability(state1, now, 300)
	if len(alerted) != 0 {
		t.Errorf("expected 0 alerts on initial unavailable state, got %v", alerted)
	}

	// 2. Second poll: GRA becomes in stock (24H)
	now = now.Add(35 * time.Second)
	state2 := map[string]monitor.DatacenterAvailability{
		"GRA": monitor.Avail24H,
		"SBG": monitor.AvailUnavailable,
		"RBX": monitor.AvailInstant, // Not tracked in subscription
	}
	alerted = sub.UpdateAvailability(state2, now, 300)
	if len(alerted) != 1 || alerted[0] != "GRA" {
		t.Errorf("expected alert for GRA only, got %v", alerted)
	}

	// 3. Third poll immediately after: GRA still in stock -> debounce suppresses alert
	now = now.Add(10 * time.Second)
	alerted = sub.UpdateAvailability(state2, now, 300)
	if len(alerted) != 0 {
		t.Errorf("expected debounce to suppress duplicate alert, got %v", alerted)
	}

	// 4. Fourth poll: SBG becomes in stock after debounce interval passes
	now = now.Add(350 * time.Second)
	state3 := map[string]monitor.DatacenterAvailability{
		"GRA": monitor.Avail24H,
		"SBG": monitor.AvailInstant,
	}
	alerted = sub.UpdateAvailability(state3, now, 300)
	if len(alerted) != 1 || alerted[0] != "SBG" {
		t.Errorf("expected alert for newly stocked SBG, got %v", alerted)
	}
}
