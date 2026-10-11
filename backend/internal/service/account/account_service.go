// Package account provides application business services for OVH account orchestration.
package account

import (
	"errors"
	"strings"

	"github.com/ovh-webui/server/internal/db"
	domainAccount "github.com/ovh-webui/server/internal/domain/account"
	"github.com/ovh-webui/server/internal/netfp"
	"github.com/ovh-webui/server/internal/repository"
	"github.com/ovh-webui/server/internal/types"
)

var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrEmptyAccountName   = errors.New("account name cannot be empty")
	ErrInvalidCredentials = errors.New("appKey, appSecret, and consumerKey are required")
)

// AccountDTO represents a safe, optionally masked account representation for API consumers.
type AccountDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Endpoint      string `json:"endpoint"`
	Zone          string `json:"zone"`
	AppKey        string `json:"appKey"`
	AppSecret     string `json:"appSecret,omitempty"`
	ConsumerKey   string `json:"consumerKey,omitempty"`
	IAM           string `json:"iam"`
	IsDefault     bool   `json:"isDefault"`
	CreatedAt     string `json:"createdAt"`
	ProxyURL      string `json:"proxyUrl,omitempty"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	CredState     string `json:"credState"`
	CredCheckedAt string `json:"credCheckedAt,omitempty"`
	CredEvidence  string `json:"credEvidence,omitempty"`
}

// Service defines application operations on accounts.
type Service interface {
	ListAccounts() ([]AccountDTO, error)
	GetAccount(id string) (AccountDTO, error)
	FindAccount(id string) (types.OVHAccount, bool, error)
	GetAccountRaw(id string) (db.RawAccount, bool, error)
	CountAccounts() (int, error)
	UpsertAccount(acc types.OVHAccount) error
	DeleteAccount(id string) error
	SetDefault(id string) error
	UpdateCredState(id string, state string, verifiedAt string, reason string) error
}

type accountService struct {
	repo repository.AccountRepository
}

// NewService creates a new account application service.
func NewService(repo repository.AccountRepository) Service {
	return &accountService{repo: repo}
}

// maskSecret masks a secret string leaving first 3 and last 3 characters visible.
func maskSecret(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 6 {
		return "******"
	}
	return s[:3] + "..." + s[len(s)-3:]
}

func toDTO(a types.OVHAccount) AccountDTO {
	return AccountDTO{
		ID:            a.ID,
		Name:          a.Name,
		Endpoint:      a.Endpoint,
		Zone:          a.Zone,
		AppKey:        a.AppKey,
		AppSecret:     maskSecret(a.AppSecret),
		ConsumerKey:   maskSecret(a.ConsumerKey),
		IAM:           a.IAM,
		IsDefault:     a.IsDefault,
		CreatedAt:     a.CreatedAt,
		ProxyURL:      netfp.ScrubProxyURL(a.ProxyURL),
		Fingerprint:   a.Fingerprint,
		CredState:     a.CredState,
		CredCheckedAt: a.CredCheckedAt,
		CredEvidence:  a.CredEvidence,
	}
}

func (s *accountService) ListAccounts() ([]AccountDTO, error) {
	accs, err := s.repo.ListAccounts()
	if err != nil {
		return nil, err
	}
	dtos := make([]AccountDTO, 0, len(accs))
	for _, a := range accs {
		dtos = append(dtos, toDTO(a))
	}
	return dtos, nil
}

func (s *accountService) GetAccount(id string) (AccountDTO, error) {
	a, ok, err := s.repo.GetAccount(id)
	if err != nil {
		return AccountDTO{}, err
	}
	if !ok {
		return AccountDTO{}, ErrAccountNotFound
	}
	return toDTO(a), nil
}

func (s *accountService) FindAccount(id string) (types.OVHAccount, bool, error) {
	return s.repo.GetAccount(id)
}

func (s *accountService) GetAccountRaw(id string) (db.RawAccount, bool, error) {
	return s.repo.GetAccountRaw(id)
}

func (s *accountService) CountAccounts() (int, error) {
	return s.repo.CountAccounts()
}

func (s *accountService) UpsertAccount(acc types.OVHAccount) error {
	return s.repo.UpsertAccount(acc)
}

func (s *accountService) DeleteAccount(id string) error {
	return s.repo.DeleteAccount(id)
}

func (s *accountService) SetDefault(id string) error {
	return s.repo.SetDefaultAccount(id)
}

func (s *accountService) UpdateCredState(id string, state string, verifiedAt string, reason string) error {
	return s.repo.UpdateAccountCredState(id, state, verifiedAt, reason)
}

// Ensure domain models can be used for derivation validations
var _ = domainAccount.NewAccount
