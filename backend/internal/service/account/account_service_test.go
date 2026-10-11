package account_test

import (
	"testing"

	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/service/account"
	"github.com/ovh-webui/server/internal/types"
)

type mockAccountRepo struct {
	accounts map[string]types.OVHAccount
}

func (m *mockAccountRepo) ListAccounts() ([]types.OVHAccount, error) {
	var list []types.OVHAccount
	for _, a := range m.accounts {
		list = append(list, a)
	}
	return list, nil
}

func (m *mockAccountRepo) GetAccount(id string) (types.OVHAccount, bool, error) {
	a, ok := m.accounts[id]
	return a, ok, nil
}

func (m *mockAccountRepo) GetAccountRaw(id string) (db.RawAccount, bool, error) {
	return db.RawAccount{}, false, nil
}

func (m *mockAccountRepo) CountAccounts() (int, error) {
	return len(m.accounts), nil
}

func (m *mockAccountRepo) UpsertAccount(acc types.OVHAccount) error {
	m.accounts[acc.ID] = acc
	return nil
}

func (m *mockAccountRepo) DeleteAccount(id string) error {
	delete(m.accounts, id)
	return nil
}

func (m *mockAccountRepo) SetDefaultAccount(id string) error {
	for k, a := range m.accounts {
		a.IsDefault = (k == id)
		m.accounts[k] = a
	}
	return nil
}

func (m *mockAccountRepo) UpdateAccountCredState(id string, state string, verifiedAt string, reason string) error {
	if a, ok := m.accounts[id]; ok {
		a.CredState = state
		m.accounts[id] = a
	}
	return nil
}

func TestAccountService(t *testing.T) {
	repo := &mockAccountRepo{
		accounts: map[string]types.OVHAccount{
			"acc-1": {
				ID:          "acc-1",
				Name:        "Test Account",
				Endpoint:    "ovh-eu",
				Zone:        "FR",
				AppKey:      "ak12345678",
				AppSecret:   "sec87654321",
				ConsumerKey: "ck11223344",
				IsDefault:   true,
			},
		},
	}

	svc := account.NewService(repo)

	// Test ListAccounts (masking secrets)
	list, err := svc.ListAccounts()
	if err != nil {
		t.Fatalf("unexpected error listing accounts: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 account, got %d", len(list))
	}
	if list[0].AppSecret == "sec87654321" {
		t.Errorf("AppSecret should be masked, got raw secret: %s", list[0].AppSecret)
	}

	// Test GetAccount
	item, err := svc.GetAccount("acc-1")
	if err != nil {
		t.Fatalf("unexpected error getting account: %v", err)
	}
	if item.Name != "Test Account" {
		t.Errorf("expected name 'Test Account', got %s", item.Name)
	}

	// Test DeleteAccount
	if err := svc.DeleteAccount("acc-1"); err != nil {
		t.Fatalf("unexpected error deleting account: %v", err)
	}
	if _, err := svc.GetAccount("acc-1"); err != account.ErrAccountNotFound {
		t.Fatalf("expected ErrAccountNotFound after delete, got %v", err)
	}
}
