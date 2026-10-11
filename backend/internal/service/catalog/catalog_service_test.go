package catalog_test

import (
	"testing"

	"github.com/ovh-webui/server/internal/service/catalog"
)

type mockCatalogRepo struct {
	cache map[string]string
}

func (m *mockCatalogRepo) GetCatalog(sub string) (string, int64, bool, error) {
	val, ok := m.cache[sub]
	return val, 123456789, ok, nil
}

func (m *mockCatalogRepo) UpsertCatalog(sub string, rawJSON string) error {
	m.cache[sub] = rawJSON
	return nil
}

func (m *mockCatalogRepo) ClearCatalogs() error {
	m.cache = make(map[string]string)
	return nil
}

func TestCatalogService(t *testing.T) {
	repo := &mockCatalogRepo{cache: make(map[string]string)}
	svc := catalog.NewService(repo)

	// Save catalog
	if err := svc.SaveCatalog("fr", `{"plans":[]}`); err != nil {
		t.Fatalf("unexpected error saving catalog: %v", err)
	}

	// Get catalog with lowercase conversion
	raw, ts, ok, err := svc.GetCatalog("FR")
	if err != nil || !ok || raw != `{"plans":[]}` || ts != 123456789 {
		t.Fatalf("unexpected catalog result: raw=%s, ok=%v, ts=%d, err=%v", raw, ok, ts, err)
	}

	// Clear catalogs
	if err := svc.ClearCatalogs(); err != nil {
		t.Fatalf("unexpected error clearing catalogs: %v", err)
	}
	_, _, ok, _ = svc.GetCatalog("FR")
	if ok {
		t.Errorf("expected ok false after clear, got true")
	}
}
