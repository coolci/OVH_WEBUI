// Package monitor provides pure domain entities and invariants for availability tracking and notifications.
// Pure domain: zero external dependencies outside standard library.
package monitor

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MinIntervalSeconds = 5
	MaxIntervalSeconds = 86400 // 24 hours
	DefaultDebounceSec = 300   // 5 minutes
)

var (
	ErrEmptyPlanCode      = errors.New("subscription plan code cannot be empty")
	ErrInvalidInterval    = fmt.Errorf("monitor interval must be between %d and %d seconds", MinIntervalSeconds, MaxIntervalSeconds)
	ErrEmptyDatacenters   = errors.New("at least one datacenter must be specified")
	ErrDatacenterNotFound = errors.New("datacenter not found in availability snapshot")
)

// DatacenterAvailability maps datacenter codes to OVH availability enumeration.
type DatacenterAvailability string

const (
	AvailUnavailable DatacenterAvailability = "unavailable"
	AvailUnknown     DatacenterAvailability = "unknown"
	Avail24H         DatacenterAvailability = "24H"
	Avail72H         DatacenterAvailability = "72H"
	AvailInstant     DatacenterAvailability = "instant"
)

// IsInStock evaluates whether an OVH availability enum indicates purchaseable inventory.
func (d DatacenterAvailability) IsInStock() bool {
	s := strings.ToLower(string(d))
	if s == "" || s == "unavailable" || s == "unknown" || s == "false" || s == "none" {
		return false
	}
	return true
}

// Subscription represents an active inventory monitoring rule.
type Subscription struct {
	PlanCode        string                            `json:"planCode"`
	Datacenters     []string                          `json:"datacenters"` // Empty slice = all datacenters
	IntervalSeconds int                               `json:"intervalSeconds"`
	Enabled         bool                              `json:"enabled"`
	CreatedAt       time.Time                         `json:"createdAt"`
	LastCheckedAt   *time.Time                        `json:"lastCheckedAt,omitempty"`
	LastNotifiedAt  *time.Time                        `json:"lastNotifiedAt,omitempty"`
	LastState       map[string]DatacenterAvailability `json:"lastState,omitempty"`
}

// NewSubscription creates and validates a monitoring subscription.
func NewSubscription(planCode string, dcs []string, intervalSec int, now time.Time) (*Subscription, error) {
	planCode = strings.TrimSpace(planCode)
	if planCode == "" {
		return nil, ErrEmptyPlanCode
	}
	if intervalSec < MinIntervalSeconds || intervalSec > MaxIntervalSeconds {
		return nil, ErrInvalidInterval
	}

	normalizedDCs := make([]string, 0, len(dcs))
	for _, dc := range dcs {
		trimmed := strings.ToUpper(strings.TrimSpace(dc))
		if trimmed != "" {
			normalizedDCs = append(normalizedDCs, trimmed)
		}
	}

	return &Subscription{
		PlanCode:        planCode,
		Datacenters:     normalizedDCs,
		IntervalSeconds: intervalSec,
		Enabled:         true,
		CreatedAt:       now,
		LastState:       make(map[string]DatacenterAvailability),
	}, nil
}

// UpdateAvailability evaluates current availability against historical state.
// Returns list of datacenters that experienced an "out-of-stock -> in-stock" transition.
func (s *Subscription) UpdateAvailability(current map[string]DatacenterAvailability, now time.Time, debounceSec int) []string {
	if debounceSec <= 0 {
		debounceSec = DefaultDebounceSec
	}

	var newlyAvailable []string

	shouldCheckDC := func(dc string) bool {
		if len(s.Datacenters) == 0 {
			return true
		}
		for _, tracked := range s.Datacenters {
			if strings.EqualFold(tracked, dc) {
				return true
			}
		}
		return false
	}

	for dc, currAvail := range current {
		dcUpper := strings.ToUpper(dc)
		if !shouldCheckDC(dcUpper) {
			continue
		}

		prevAvail := s.LastState[dcUpper]
		wasInStock := prevAvail.IsInStock()
		isInStock := currAvail.IsInStock()

		// Stock transition detection
		if isInStock && !wasInStock {
			// Check debounce
			if s.LastNotifiedAt == nil || now.Sub(*s.LastNotifiedAt) >= time.Duration(debounceSec)*time.Second {
				newlyAvailable = append(newlyAvailable, dcUpper)
			}
		}
	}

	// Update cached state
	newState := make(map[string]DatacenterAvailability, len(current))
	for k, v := range current {
		newState[strings.ToUpper(k)] = v
	}
	s.LastState = newState
	s.LastCheckedAt = &now

	if len(newlyAvailable) > 0 {
		s.LastNotifiedAt = &now
	}

	return newlyAvailable
}
