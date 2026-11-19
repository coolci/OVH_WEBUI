package server_test

import (
	"testing"

	"github.com/ovh-webui/server/internal/service/server"
)

type mockAliasRepo struct {
	aliases map[string]map[string]string
}

func (m *mockAliasRepo) ListAliasesByAccount(accID string) (map[string]string, error) {
	if m.aliases[accID] == nil {
		return map[string]string{}, nil
	}
	res := make(map[string]string)
	for k, v := range m.aliases[accID] {
		res[k] = v
	}
	return res, nil
}

func (m *mockAliasRepo) UpsertAlias(accID, svc, alias string) error {
	if m.aliases[accID] == nil {
		m.aliases[accID] = make(map[string]string)
	}
	m.aliases[accID][svc] = alias
	return nil
}

func (m *mockAliasRepo) DeleteAlias(accID, svc string) error {
	if m.aliases[accID] != nil {
		delete(m.aliases[accID], svc)
	}
	return nil
}

func TestServerService(t *testing.T) {
	repo := &mockAliasRepo{
		aliases: make(map[string]map[string]string),
	}
	svc := server.NewService(repo)

	// Set alias
	if err := svc.SetAlias("acc-1", "ns123.ovh.net", "Production Database"); err != nil {
		t.Fatalf("unexpected error setting alias: %v", err)
	}

	// List aliases
	m, err := svc.ListAliases("acc-1")
	if err != nil || m["ns123.ovh.net"] != "Production Database" {
		t.Fatalf("unexpected aliases: %+v, err=%v", m, err)
	}

	// Delete alias
	if err := svc.DeleteAlias("acc-1", "ns123.ovh.net"); err != nil {
		t.Fatalf("unexpected error deleting alias: %v", err)
	}
	m, _ = svc.ListAliases("acc-1")
	if len(m) != 0 {
		t.Errorf("expected 0 aliases after delete, got %d", len(m))
	}
}
