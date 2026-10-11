// Package vps provides pure domain entities and invariants for Virtual Private Server lifecycle management.
// Pure domain: zero external dependencies outside standard library.
package vps

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type State string

const (
	StateRunning     State = "running"
	StateStopped     State = "stopped"
	StateStopping    State = "stopping"
	StateRebooting   State = "rebooting"
	StateInstalling  State = "installing"
	StateRescued     State = "rescued"
	StateMaintenance State = "maintenance"
)

var (
	ErrEmptyServiceName    = errors.New("vps service name cannot be empty")
	ErrInvalidPowerAction   = errors.New("invalid power transition for current VPS state")
	ErrMissingRebuildTarget = errors.New("rebuild requires either imageId or templateId")
	ErrConflictingTarget    = errors.New("cannot specify both imageId and templateId simultaneously")
	ErrWeakPassword         = errors.New("root password must be at least 12 characters and contain uppercase, lowercase, and digits")
)

// RebuildSpec specifies OS reinstallation or image rebuild parameters.
type RebuildSpec struct {
	ImageID    string `json:"imageId,omitempty"`
	TemplateID string `json:"templateId,omitempty"`
	SSHKey     string `json:"sshKey,omitempty"`
	Password   string `json:"password,omitempty"`
}

// Validate ensures rebuild parameter invariants according to OVH API specifications.
func (r *RebuildSpec) Validate() error {
	img := strings.TrimSpace(r.ImageID)
	tmpl := strings.TrimSpace(r.TemplateID)

	if img == "" && tmpl == "" {
		return ErrMissingRebuildTarget
	}
	if img != "" && tmpl != "" {
		return ErrConflictingTarget
	}

	if r.Password != "" {
		pass := r.Password
		if len(pass) < 12 {
			return ErrWeakPassword
		}
		var hasUpper, hasLower, hasDigit bool
		for _, ch := range pass {
			switch {
			case ch >= 'A' && ch <= 'Z':
				hasUpper = true
			case ch >= 'a' && ch <= 'z':
				hasLower = true
			case ch >= '0' && ch <= '9':
				hasDigit = true
			}
		}
		if !hasUpper || !hasLower || !hasDigit {
			return ErrWeakPassword
		}
	}

	return nil
}

// Vps represents an owned Virtual Private Server.
type Vps struct {
	ServiceName string    `json:"serviceName"`
	DisplayName string    `json:"displayName"`
	State       State     `json:"state"`
	Zone        string    `json:"zone"`
	Model       string    `json:"model"`
	VCores      int       `json:"vcores"`
	MemoryMB    int       `json:"memoryMb"`
	DiskGB      int       `json:"diskGb"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// NewVps creates and validates a Vps domain entity.
func NewVps(serviceName, zone, model string, vcores, memoryMB, diskGB int, now time.Time) (*Vps, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil, ErrEmptyServiceName
	}
	return &Vps{
		ServiceName: serviceName,
		Zone:        strings.ToLower(strings.TrimSpace(zone)),
		Model:       strings.TrimSpace(model),
		State:       StateRunning,
		VCores:      vcores,
		MemoryMB:    memoryMB,
		DiskGB:      diskGB,
		UpdatedAt:   now,
	}, nil
}

// CanStart returns true if VPS can receive a start signal.
func (v *Vps) CanStart() bool {
	return v.State == StateStopped
}

// CanStop returns true if VPS can receive a stop signal.
func (v *Vps) CanStop() bool {
	return v.State == StateRunning
}

// CanReboot returns true if VPS can receive a reboot signal.
func (v *Vps) CanReboot() bool {
	return v.State == StateRunning || v.State == StateRescued
}

// TransitionPower updates the VPS state based on power action.
func (v *Vps) TransitionPower(action string, now time.Time) error {
	switch strings.ToLower(action) {
	case "start":
		if !v.CanStart() {
			return fmt.Errorf("%w: cannot start VPS in %s state", ErrInvalidPowerAction, v.State)
		}
		v.State = StateRunning
	case "stop":
		if !v.CanStop() {
			return fmt.Errorf("%w: cannot stop VPS in %s state", ErrInvalidPowerAction, v.State)
		}
		v.State = StateStopped
	case "reboot":
		if !v.CanReboot() {
			return fmt.Errorf("%w: cannot reboot VPS in %s state", ErrInvalidPowerAction, v.State)
		}
		v.State = StateRebooting
	default:
		return fmt.Errorf("unknown power action: %s", action)
	}

	v.UpdatedAt = now
	return nil
}
