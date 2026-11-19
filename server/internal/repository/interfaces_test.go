package repository_test

import (
	"testing"

	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/repository"
)

// Compile-time interface assertion
var (
	_ repository.AccountRepository     = (*db.DB)(nil)
	_ repository.QueueRepository       = (*db.DB)(nil)
	_ repository.HistoryRepository     = (*db.DB)(nil)
	_ repository.CatalogRepository     = (*db.DB)(nil)
	_ repository.ServerAliasRepository = (*db.DB)(nil)
	_ repository.MonitorRepository     = (*db.DB)(nil)
)

func TestRepositoryInterfaces(t *testing.T) {
	// The var declarations above ensure static type safety at compile time.
	t.Log("All database repository interfaces are statically verified and satisfied.")
}
