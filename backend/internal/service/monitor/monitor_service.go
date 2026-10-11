// Package monitor provides application business services for stock inventory monitoring.
package monitor

import (
	"strings"
	"time"

	domainMonitor "github.com/ovh-webui/server/internal/domain/monitor"
	"github.com/ovh-webui/server/internal/repository"
	"github.com/ovh-webui/server/internal/types"
)

// Service defines application operations for monitoring subscriptions.
type Service interface {
	ListSubscriptions() ([]types.Subscription, error)
	SaveSubscription(sub types.Subscription) error
	DeleteSubscription(planCode string) error
	ClearSubscriptions() (int64, error)
}

type monitorService struct {
	repo repository.MonitorRepository
}

// NewService creates a new monitor application service.
func NewService(repo repository.MonitorRepository) Service {
	return &monitorService{repo: repo}
}

func (s *monitorService) ListSubscriptions() ([]types.Subscription, error) {
	return s.repo.ListMonitorSubscriptions()
}

func (s *monitorService) SaveSubscription(sub types.Subscription) error {
	// Domain validation
	_, err := domainMonitor.NewSubscription(sub.PlanCode, sub.Datacenters, 30, time.Now())
	if err != nil {
		return err
	}
	return s.repo.UpsertMonitorSubscription(sub)
}

func (s *monitorService) DeleteSubscription(planCode string) error {
	return s.repo.DeleteMonitorSubscription(strings.TrimSpace(planCode))
}

func (s *monitorService) ClearSubscriptions() (int64, error) {
	return s.repo.ClearMonitorSubscriptions()
}
