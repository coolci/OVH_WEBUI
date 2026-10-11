// Package order provides application business services for automated purchase queue and history.
package order

import (
	"errors"
	"time"

	domainOrder "github.com/ovh-webui/server/internal/domain/order"
	"github.com/ovh-webui/server/internal/repository"
	"github.com/ovh-webui/server/internal/types"
)

var (
	ErrItemNotFound = errors.New("queue item not found")
)

// Service defines application operations for the order queue and history.
type Service interface {
	ListQueue() ([]types.QueueItem, error)
	RemoveQueueItem(id string) error
	ClearQueue() (int64, error)
	UpdateQueueInterval(id string, intervalMs int) error
	UpdateQueueStatus(id string, status string) error
	ListHistory() ([]types.PurchaseHistoryEntry, error)
	ClearHistory() (int64, error)
}

type orderService struct {
	queueRepo   repository.QueueRepository
	historyRepo repository.HistoryRepository
}

// NewService creates a new order application service.
func NewService(qRepo repository.QueueRepository, hRepo repository.HistoryRepository) Service {
	return &orderService{
		queueRepo:   qRepo,
		historyRepo: hRepo,
	}
}

func (s *orderService) ListQueue() ([]types.QueueItem, error) {
	return s.queueRepo.ListQueue()
}

func (s *orderService) RemoveQueueItem(id string) error {
	return s.queueRepo.DeleteQueueItem(id)
}

func (s *orderService) ClearQueue() (int64, error) {
	return s.queueRepo.ClearQueue()
}

func (s *orderService) UpdateQueueInterval(id string, intervalMs int) error {
	// Validate bounds against pure domain rule
	if intervalMs < domainOrder.MinIntervalMs || intervalMs > domainOrder.MaxIntervalMs {
		return domainOrder.ErrInvalidInterval
	}

	items, err := s.queueRepo.ListQueue()
	if err != nil {
		return err
	}

	found := false
	for i := range items {
		if items[i].ID == id {
			items[i].RetryInterval = intervalMs / 1000
			items[i].UpdatedAt = types.NowISO()
			found = true
			break
		}
	}
	if !found {
		return ErrItemNotFound
	}

	return s.queueRepo.ReplaceQueue(items)
}

func (s *orderService) UpdateQueueStatus(id string, status string) error {
	items, err := s.queueRepo.ListQueue()
	if err != nil {
		return err
	}

	found := false
	for i := range items {
		if items[i].ID == id {
			// Validate state transition using domain model
			domItem, err := domainOrder.NewQueueItem(
				items[i].ID,
				items[i].AccountID,
				items[i].PlanCode,
				items[i].Datacenter,
				1,
				1000,
				items[i].AutoPay,
				time.Now(),
			)
			if err == nil {
				domItem.Status = domainOrder.QueueStatus(items[i].Status)
				if err := domItem.TransitionTo(domainOrder.QueueStatus(status), time.Now()); err != nil {
					return err
				}
			}

			items[i].Status = status
			items[i].UpdatedAt = types.NowISO()
			found = true
			break
		}
	}
	if !found {
		return ErrItemNotFound
	}

	return s.queueRepo.ReplaceQueue(items)
}

func (s *orderService) ListHistory() ([]types.PurchaseHistoryEntry, error) {
	return s.historyRepo.ListHistory()
}

func (s *orderService) ClearHistory() (int64, error) {
	return s.historyRepo.ClearHistory()
}
