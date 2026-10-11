// Package server provides pure domain entities and invariants for dedicated server management.
// Pure domain: zero external dependencies outside standard library.
package server

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type BootType string

const (
	BootLocal   BootType = "local"
	BootRescue  BootType = "rescue"
	BootNetwork BootType = "network"
	BootIPXE    BootType = "ipxe"

	// Retraction statutory window in European Union: 14 calendar days
	RetractionMaxDays = 14
)

var (
	ErrEmptyServiceName    = errors.New("server service name cannot be empty")
	ErrRescueMailRequired   = errors.New("rescue mail address is required for rescue boot mode")
	ErrInvalidEmail         = errors.New("invalid email address format")
	ErrRetractionExpired    = fmt.Errorf("statutory retraction period (%d days) has expired", RetractionMaxDays)
	ErrRetractionNotStarted = errors.New("retraction eligibility cannot precede server delivery date")
)

// HardwareSpec represents normalized physical specs of a dedicated server.
type HardwareSpec struct {
	CPUModel  string `json:"cpuModel"`
	CPUCores  int    `json:"cpuCores"`
	RAMGB     int    `json:"ramGb"`
	DiskType  string `json:"diskType"` // NVMe, SATA, SSD
	DiskCount int    `json:"diskCount"`
	DiskGB    int    `json:"diskGb"`
}

// LotteryResult compares actual hardware against guaranteed catalog baseline.
type LotteryResult struct {
	CPUUpgrade  bool `json:"cpuUpgrade"`
	RAMUpgrade  bool `json:"ramUpgrade"`
	DiskUpgrade bool `json:"diskUpgrade"`
}

// CheckLottery evaluates whether actual delivered hardware exceeds baseline catalog specs.
func CheckLottery(baseline, actual HardwareSpec) LotteryResult {
	return LotteryResult{
		CPUUpgrade:  actual.CPUCores > baseline.CPUCores,
		RAMUpgrade:  actual.RAMGB > baseline.RAMGB,
		DiskUpgrade: (actual.DiskCount * actual.DiskGB) > (baseline.DiskCount * baseline.DiskGB),
	}
}

// DedicatedServer represents a bare-metal server entity.
type DedicatedServer struct {
	ServiceName string       `json:"serviceName"`
	Datacenter  string       `json:"datacenter"`
	BootType    BootType     `json:"bootType"`
	RescueMail  string       `json:"rescueMail,omitempty"`
	DeliveredAt *time.Time   `json:"deliveredAt,omitempty"`
	Hardware    HardwareSpec `json:"hardware"`
}

// NewDedicatedServer creates and validates a DedicatedServer domain entity.
func NewDedicatedServer(serviceName, datacenter string, boot BootType, deliveredAt *time.Time) (*DedicatedServer, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil, ErrEmptyServiceName
	}
	return &DedicatedServer{
		ServiceName: serviceName,
		Datacenter:  strings.ToUpper(strings.TrimSpace(datacenter)),
		BootType:    boot,
		DeliveredAt: deliveredAt,
	}, nil
}

// SetBootMode applies a boot mode change with domain validation.
func (s *DedicatedServer) SetBootMode(target BootType, rescueMail string) error {
	if target == BootRescue {
		rescueMail = strings.TrimSpace(rescueMail)
		if rescueMail == "" {
			return ErrRescueMailRequired
		}
		if !strings.Contains(rescueMail, "@") || !strings.Contains(rescueMail, ".") {
			return ErrInvalidEmail
		}
		s.RescueMail = rescueMail
	} else {
		s.RescueMail = ""
	}
	s.BootType = target
	return nil
}

// CheckRetractionEligibility evaluates statutory 14-day retraction eligibility.
func (s *DedicatedServer) CheckRetractionEligibility(now time.Time) (bool, int, error) {
	if s.DeliveredAt == nil {
		return false, 0, errors.New("server has not been delivered yet")
	}
	if now.Before(*s.DeliveredAt) {
		return false, 0, ErrRetractionNotStarted
	}

	elapsed := now.Sub(*s.DeliveredAt)
	elapsedDays := int(elapsed.Hours() / 24)
	remainingDays := RetractionMaxDays - elapsedDays

	if remainingDays < 0 {
		return false, 0, ErrRetractionExpired
	}

	return true, remainingDays, nil
}
