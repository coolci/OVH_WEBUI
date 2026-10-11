// Package catalog provides application business services for product catalog and pricing cache.
package catalog

import (
	"strings"

	domainCatalog "github.com/ovh-webui/server/internal/domain/catalog"
	"github.com/ovh-webui/server/internal/repository"
)

// Service defines application operations for catalog caching.
type Service interface {
	GetCatalog(subsidiary string) (string, int64, bool, error)
	SaveCatalog(subsidiary string, rawJSON string) error
	ClearCatalogs() error
}

type catalogService struct {
	repo repository.CatalogRepository
}

// NewService creates a new catalog application service.
func NewService(repo repository.CatalogRepository) Service {
	return &catalogService{repo: repo}
}

func (s *catalogService) GetCatalog(subsidiary string) (string, int64, bool, error) {
	sub := strings.ToUpper(strings.TrimSpace(subsidiary))
	return s.repo.GetCatalog(sub)
}

func (s *catalogService) SaveCatalog(subsidiary string, rawJSON string) error {
	sub := strings.ToUpper(strings.TrimSpace(subsidiary))
	return s.repo.UpsertCatalog(sub, rawJSON)
}

func (s *catalogService) ClearCatalogs() error {
	return s.repo.ClearCatalogs()
}

// Reference domain for plan normalization
var _ = domainCatalog.NormalizePlanCode
