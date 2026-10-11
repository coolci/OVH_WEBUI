// Package server provides application business services for dedicated server operations.
package server

import (
	"strings"

	domainServer "github.com/ovh-webui/server/internal/domain/server"
	"github.com/ovh-webui/server/internal/repository"
)

// Service defines application operations for dedicated servers.
type Service interface {
	ListAliases(accountID string) (map[string]string, error)
	SetAlias(accountID, serviceName, alias string) error
	DeleteAlias(accountID, serviceName string) error
}

type serverService struct {
	aliasRepo repository.ServerAliasRepository
}

// NewService creates a new server application service.
func NewService(aliasRepo repository.ServerAliasRepository) Service {
	return &serverService{aliasRepo: aliasRepo}
}

func (s *serverService) ListAliases(accountID string) (map[string]string, error) {
	return s.aliasRepo.ListAliasesByAccount(accountID)
}

func (s *serverService) SetAlias(accountID, serviceName, alias string) error {
	serviceName = strings.TrimSpace(serviceName)
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return s.aliasRepo.DeleteAlias(accountID, serviceName)
	}
	return s.aliasRepo.UpsertAlias(accountID, serviceName, alias)
}

func (s *serverService) DeleteAlias(accountID, serviceName string) error {
	return s.aliasRepo.DeleteAlias(accountID, strings.TrimSpace(serviceName))
}

// Domain assertion
var _ = domainServer.NewDedicatedServer
