// Package account provides domain models and business invariants for OVH accounts.
// Pure domain: zero external dependencies outside standard library.
package account

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Subsidiary string
type Endpoint string
type IAM string
type CredentialState string

const (
	EndpointEU Endpoint = "ovh-eu"
	EndpointUS Endpoint = "ovh-us"
	EndpointCA Endpoint = "ovh-ca"

	IAMFR IAM = "go-ovh-fr"
	IAMUS IAM = "go-ovh-us"
	IAMCA IAM = "go-ovh-ca"

	StateUnverified CredentialState = "unverified"
	StateVerified   CredentialState = "verified"
	StateInvalid    CredentialState = "invalid"
)

var (
	ErrUnknownSubsidiary = errors.New("unknown or unsupported subsidiary")
	ErrEmptyAccountName  = errors.New("account name cannot be empty")
	ErrInvalidAppKey     = errors.New("application key cannot be empty")
	ErrInvalidSecret     = errors.New("application secret cannot be empty")
	ErrInvalidConsumer   = errors.New("consumer key cannot be empty")
)

var euSubsidiaries = map[string]struct{}{
	"IE": {}, "FR": {}, "DE": {}, "GB": {}, "IT": {},
	"ES": {}, "PL": {}, "NL": {}, "PT": {}, "FI": {}, "CZ": {},
	"EU": {},
}

var caSubsidiaries = map[string]struct{}{
	"CA": {}, "QC": {}, "ASIA": {}, "SG": {}, "AU": {}, "IN": {},
}

// DeriveEndpoint derives the official OVH API endpoint from subsidiary code.
func DeriveEndpoint(s Subsidiary) (Endpoint, error) {
	code := strings.ToUpper(strings.TrimSpace(string(s)))
	if _, ok := euSubsidiaries[code]; ok {
		return EndpointEU, nil
	}
	if code == "US" {
		return EndpointUS, nil
	}
	if _, ok := caSubsidiaries[code]; ok {
		return EndpointCA, nil
	}
	return "", fmt.Errorf("%w: %s", ErrUnknownSubsidiary, s)
}

// DeriveIAM derives the IAM policy prefix according to PRD §2.2.2.
// Note: For ovh-eu, the IAM convention is go-ovh-fr (France is the baseline EU region name).
func DeriveIAM(s Subsidiary) (IAM, error) {
	ep, err := DeriveEndpoint(s)
	if err != nil {
		return "", err
	}
	switch ep {
	case EndpointEU:
		return IAMFR, nil
	case EndpointUS:
		return IAMUS, nil
	case EndpointCA:
		return IAMCA, nil
	default:
		return "", fmt.Errorf("no IAM mapping for endpoint: %s", ep)
	}
}

// Account represents a configured OVH Account entity.
type Account struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Subsidiary         Subsidiary      `json:"subsidiary"`
	Endpoint           Endpoint        `json:"endpoint"`
	IAM                IAM             `json:"iam"`
	AppKey             string          `json:"appKey"`
	AppSecret          string          `json:"appSecret,omitempty"`
	ConsumerKey        string          `json:"consumerKey,omitempty"`
	IsDefault          bool            `json:"isDefault"`
	CredentialState    CredentialState `json:"credentialState"`
	LastVerifiedAt     *time.Time      `json:"lastVerifiedAt,omitempty"`
	VerificationReason string          `json:"verificationReason,omitempty"`
}

// NewAccount creates an account with derived endpoint and IAM.
func NewAccount(id, name string, sub Subsidiary, appKey, appSecret, consumerKey string, isDefault bool) (*Account, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyAccountName
	}
	if strings.TrimSpace(appKey) == "" {
		return nil, ErrInvalidAppKey
	}
	if strings.TrimSpace(appSecret) == "" {
		return nil, ErrInvalidSecret
	}
	if strings.TrimSpace(consumerKey) == "" {
		return nil, ErrInvalidConsumer
	}

	ep, err := DeriveEndpoint(sub)
	if err != nil {
		return nil, err
	}
	iam, err := DeriveIAM(sub)
	if err != nil {
		return nil, err
	}

	return &Account{
		ID:              id,
		Name:            name,
		Subsidiary:      sub,
		Endpoint:        ep,
		IAM:             iam,
		AppKey:          appKey,
		AppSecret:       appSecret,
		ConsumerKey:     consumerKey,
		IsDefault:       isDefault,
		CredentialState: StateUnverified,
	}, nil
}

// MarkVerified updates the account state when OVH validation succeeds.
func (a *Account) MarkVerified(now time.Time) {
	a.CredentialState = StateVerified
	a.LastVerifiedAt = &now
	a.VerificationReason = ""
}

// MarkInvalid records validation failure with diagnostic reason.
func (a *Account) MarkInvalid(reason string, now time.Time) {
	a.CredentialState = StateInvalid
	a.LastVerifiedAt = &now
	a.VerificationReason = reason
}
