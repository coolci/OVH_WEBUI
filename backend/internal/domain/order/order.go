// Package order provides pure domain entities and business invariants for purchase orders and queue management.
// Pure domain: zero external dependencies outside standard library.
package order

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type QueueStatus string

const (
	StatusQueued     QueueStatus = "queued"
	StatusProcessing QueueStatus = "processing"
	StatusSuccess    QueueStatus = "success"
	StatusFailed     QueueStatus = "failed"
	StatusCancelled  QueueStatus = "cancelled"
	StatusPaused     QueueStatus = "paused"

	MinIntervalMs = 500
	MaxIntervalMs = 3600000 // 1 hour
	MaxQuantity   = 20
	MinQuantity   = 1
)

var (
	ErrEmptyPlanCode     = errors.New("plan code cannot be empty")
	ErrEmptyAccountID    = errors.New("account ID cannot be empty")
	ErrInvalidQuantity   = fmt.Errorf("quantity must be between %d and %d", MinQuantity, MaxQuantity)
	ErrInvalidInterval   = fmt.Errorf("check interval must be between %d ms and %d ms", MinIntervalMs, MaxIntervalMs)
	ErrIllegalTransition = errors.New("illegal queue status transition")
	ErrQueueItemTerminal = errors.New("cannot modify terminal queue item")
)

// QueueItem represents an automated purchasing task in the queue.
type QueueItem struct {
	ID         string      `json:"id"`
	AccountID  string      `json:"accountId"`
	PlanCode   string      `json:"planCode"`
	Datacenter string      `json:"datacenter"`
	Quantity   int         `json:"quantity"`
	IntervalMs int         `json:"intervalMs"`
	AutoPay    bool        `json:"autoPay"`
	Status     QueueStatus `json:"status"`
	CreatedAt  time.Time   `json:"createdAt"`
	UpdatedAt  time.Time   `json:"updatedAt"`
	Attempts   int         `json:"attempts"`
	LastError  string      `json:"lastError,omitempty"`
	OrderId    string      `json:"orderId,omitempty"`
}

// NewQueueItem creates a validated QueueItem domain entity.
func NewQueueItem(id, accountID, planCode, datacenter string, quantity, intervalMs int, autoPay bool, now time.Time) (*QueueItem, error) {
	planCode = strings.TrimSpace(planCode)
	if planCode == "" {
		return nil, ErrEmptyPlanCode
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, ErrEmptyAccountID
	}
	if quantity < MinQuantity || quantity > MaxQuantity {
		return nil, ErrInvalidQuantity
	}
	if intervalMs < MinIntervalMs || intervalMs > MaxIntervalMs {
		return nil, ErrInvalidInterval
	}

	return &QueueItem{
		ID:         id,
		AccountID:  accountID,
		PlanCode:   planCode,
		Datacenter: strings.ToUpper(strings.TrimSpace(datacenter)),
		Quantity:   quantity,
		IntervalMs: intervalMs,
		AutoPay:    autoPay,
		Status:     StatusQueued,
		CreatedAt:  now,
		UpdatedAt:  now,
		Attempts:   0,
	}, nil
}

// IsTerminal returns true if the queue item is in a terminal state.
func (q *QueueItem) IsTerminal() bool {
	return q.Status == StatusSuccess || q.Status == StatusCancelled
}

// TransitionTo validates and executes a legal state transition.
func (q *QueueItem) TransitionTo(target QueueStatus, now time.Time) error {
	if q.IsTerminal() {
		return fmt.Errorf("%w: current status %s", ErrQueueItemTerminal, q.Status)
	}

	switch target {
	case StatusProcessing:
		if q.Status != StatusQueued && q.Status != StatusPaused && q.Status != StatusFailed {
			return fmt.Errorf("%w: cannot transition from %s to %s", ErrIllegalTransition, q.Status, target)
		}
	case StatusSuccess:
		if q.Status != StatusProcessing {
			return fmt.Errorf("%w: cannot transition from %s to %s", ErrIllegalTransition, q.Status, target)
		}
	case StatusFailed:
		if q.Status != StatusProcessing {
			return fmt.Errorf("%w: cannot transition from %s to %s", ErrIllegalTransition, q.Status, target)
		}
	case StatusPaused:
		if q.Status != StatusQueued && q.Status != StatusFailed {
			return fmt.Errorf("%w: cannot transition from %s to %s", ErrIllegalTransition, q.Status, target)
		}
	case StatusQueued:
		if q.Status != StatusPaused && q.Status != StatusFailed {
			return fmt.Errorf("%w: cannot transition from %s to %s", ErrIllegalTransition, q.Status, target)
		}
	case StatusCancelled:
		// Can cancel from any non-terminal state
	default:
		return fmt.Errorf("%w: unknown target status %s", ErrIllegalTransition, target)
	}

	q.Status = target
	q.UpdatedAt = now
	return nil
}

// RecordAttempt updates attempt count, last error and timestamps.
func (q *QueueItem) RecordAttempt(errMessage string, now time.Time) {
	q.Attempts++
	q.LastError = errMessage
	q.UpdatedAt = now
}

// RecordSuccess completes the queue item with order ID.
func (q *QueueItem) RecordSuccess(orderID string, now time.Time) error {
	if err := q.TransitionTo(StatusSuccess, now); err != nil {
		return err
	}
	q.OrderId = orderID
	q.LastError = ""
	return nil
}

// SetInterval updates the polling interval with domain validation.
func (q *QueueItem) SetInterval(intervalMs int, now time.Time) error {
	if q.IsTerminal() {
		return ErrQueueItemTerminal
	}
	if intervalMs < MinIntervalMs || intervalMs > MaxIntervalMs {
		return ErrInvalidInterval
	}
	q.IntervalMs = intervalMs
	q.UpdatedAt = now
	return nil
}
