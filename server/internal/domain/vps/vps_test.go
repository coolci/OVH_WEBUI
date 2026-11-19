package vps_test

import (
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/domain/vps"
)

func TestRebuildSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    vps.RebuildSpec
		wantErr error
	}{
		{
			name: "valid imageId rebuild",
			spec: vps.RebuildSpec{
				ImageID:  "ubuntu-22-04",
				Password: "StrongPassword123!",
			},
			wantErr: nil,
		},
		{
			name: "valid templateId rebuild",
			spec: vps.RebuildSpec{
				TemplateID: "custom-template-01",
				SSHKey:     "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5...",
			},
			wantErr: nil,
		},
		{
			name:    "empty image and template error",
			spec:    vps.RebuildSpec{},
			wantErr: vps.ErrMissingRebuildTarget,
		},
		{
			name: "both image and template conflict error",
			spec: vps.RebuildSpec{
				ImageID:    "debian-11",
				TemplateID: "my-template",
			},
			wantErr: vps.ErrConflictingTarget,
		},
		{
			name: "weak password error (too short)",
			spec: vps.RebuildSpec{
				ImageID:  "ubuntu-22-04",
				Password: "Short1!",
			},
			wantErr: vps.ErrWeakPassword,
		},
		{
			name: "weak password error (no digits)",
			spec: vps.RebuildSpec{
				ImageID:  "ubuntu-22-04",
				Password: "VeryLongPasswordWithoutDigits!",
			},
			wantErr: vps.ErrWeakPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestVps_PowerTransitions(t *testing.T) {
	now := time.Now()
	machine, err := vps.NewVps("vps-12345.ovh.net", "gra1", "vps-starter-1-2-20", 1, 2048, 20, now)
	if err != nil {
		t.Fatalf("failed to create vps: %v", err)
	}

	// Initial state: running
	if !machine.CanStop() || machine.CanStart() {
		t.Errorf("expected running VPS: canStop=true, canStart=false")
	}

	// Stop VPS
	if err := machine.TransitionPower("stop", now); err != nil {
		t.Fatalf("failed to stop VPS: %v", err)
	}
	if machine.State != vps.StateStopped {
		t.Errorf("expected stopped state, got %s", machine.State)
	}

	// Cannot stop an already stopped VPS
	if err := machine.TransitionPower("stop", now); err == nil {
		t.Fatalf("expected error stopping stopped VPS, got nil")
	}

	// Start VPS
	if err := machine.TransitionPower("start", now); err != nil {
		t.Fatalf("failed to start VPS: %v", err)
	}
	if machine.State != vps.StateRunning {
		t.Errorf("expected running state, got %s", machine.State)
	}

	// Reboot VPS
	if err := machine.TransitionPower("reboot", now); err != nil {
		t.Fatalf("failed to reboot VPS: %v", err)
	}
	if machine.State != vps.StateRebooting {
		t.Errorf("expected rebooting state, got %s", machine.State)
	}
}
