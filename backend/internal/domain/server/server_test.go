package server_test

import (
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/domain/server"
)

func TestDedicatedServer_BootMode(t *testing.T) {
	srv, err := server.NewDedicatedServer("ns12345.ovh.net", "gra", server.BootLocal, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Rescue mode requires valid email
	if err := srv.SetBootMode(server.BootRescue, ""); err != server.ErrRescueMailRequired {
		t.Fatalf("expected ErrRescueMailRequired for empty email, got %v", err)
	}
	if err := srv.SetBootMode(server.BootRescue, "not-an-email"); err != server.ErrInvalidEmail {
		t.Fatalf("expected ErrInvalidEmail for invalid email format, got %v", err)
	}

	// Valid rescue mode
	if err := srv.SetBootMode(server.BootRescue, "admin@example.com"); err != nil {
		t.Fatalf("unexpected error setting valid rescue mode: %v", err)
	}
	if srv.BootType != server.BootRescue || srv.RescueMail != "admin@example.com" {
		t.Errorf("rescue mode state mismatch: boot=%s, mail=%s", srv.BootType, srv.RescueMail)
	}

	// Switching back to local clears rescue mail
	if err := srv.SetBootMode(server.BootLocal, ""); err != nil {
		t.Fatalf("unexpected error switching to local: %v", err)
	}
	if srv.RescueMail != "" || srv.BootType != server.BootLocal {
		t.Errorf("expected cleared rescue mail and BootLocal, got mail=%s, boot=%s", srv.RescueMail, srv.BootType)
	}
}

func TestDedicatedServer_RetractionEligibility(t *testing.T) {
	deliveredAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	srv, err := server.NewDedicatedServer("ns5555.ovh.net", "rbx", server.BootLocal, &deliveredAt)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. Day 5 after delivery: eligible with 9 days left
	now := deliveredAt.Add(5 * 24 * time.Hour)
	eligible, remaining, err := srv.CheckRetractionEligibility(now)
	if err != nil || !eligible || remaining != 9 {
		t.Errorf("expected eligible with 9 days remaining, got eligible=%v, remaining=%d, err=%v", eligible, remaining, err)
	}

	// 2. Day 15 after delivery: expired
	now = deliveredAt.Add(15 * 24 * time.Hour)
	eligible, _, err = srv.CheckRetractionEligibility(now)
	if eligible || err != server.ErrRetractionExpired {
		t.Errorf("expected expired retraction error, got eligible=%v, err=%v", eligible, err)
	}
}

func TestCheckLottery(t *testing.T) {
	baseline := server.HardwareSpec{
		CPUModel:  "Xeon E-2276G",
		CPUCores:  6,
		RAMGB:     32,
		DiskType:  "NVMe",
		DiskCount: 2,
		DiskGB:    500, // Total 1000GB
	}

	actual := server.HardwareSpec{
		CPUModel:  "Xeon E-2288G",
		CPUCores:  8,
		RAMGB:     64,
		DiskType:  "NVMe",
		DiskCount: 2,
		DiskGB:    1000, // Total 2000GB
	}

	lottery := server.CheckLottery(baseline, actual)
	if !lottery.CPUUpgrade || !lottery.RAMUpgrade || !lottery.DiskUpgrade {
		t.Errorf("expected all upgrades true, got %+v", lottery)
	}
}
