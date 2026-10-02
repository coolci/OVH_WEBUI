package account

import (
	"testing"
	"time"
)

func TestDeriveEndpointAndIAM(t *testing.T) {
	tests := []struct {
		sub          Subsidiary
		wantEndpoint Endpoint
		wantIAM      IAM
		wantErr      bool
	}{
		{"FR", EndpointEU, IAMFR, false},
		{"IE", EndpointEU, IAMFR, false},
		{"DE", EndpointEU, IAMFR, false},
		{"GB", EndpointEU, IAMFR, false},
		{"US", EndpointUS, IAMUS, false},
		{"CA", EndpointCA, IAMCA, false},
		{"QC", EndpointCA, IAMCA, false},
		{"SG", EndpointCA, IAMCA, false},
		{"AU", EndpointCA, IAMCA, false},
		{"UNKNOWN", "", "", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.sub), func(t *testing.T) {
			ep, err := DeriveEndpoint(tt.sub)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DeriveEndpoint(%s) err = %v, wantErr %v", tt.sub, err, tt.wantErr)
			}
			if !tt.wantErr && ep != tt.wantEndpoint {
				t.Errorf("DeriveEndpoint(%s) = %v, want %v", tt.sub, ep, tt.wantEndpoint)
			}

			iam, err := DeriveIAM(tt.sub)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DeriveIAM(%s) err = %v, wantErr %v", tt.sub, err, tt.wantErr)
			}
			if !tt.wantErr && iam != tt.wantIAM {
				t.Errorf("DeriveIAM(%s) = %v, want %v", tt.sub, iam, tt.wantIAM)
			}
		})
	}
}

func TestAccountCredentialStateMachine(t *testing.T) {
	acc, err := NewAccount("acc_test", "Test FR Account", "FR", "ak123", "as123", "ck123", true)
	if err != nil {
		t.Fatalf("NewAccount failed: %v", err)
	}

	if acc.CredentialState != StateUnverified {
		t.Errorf("Expected initial state %s, got %s", StateUnverified, acc.CredentialState)
	}
	if acc.Endpoint != EndpointEU {
		t.Errorf("Expected endpoint %s, got %s", EndpointEU, acc.Endpoint)
	}
	if acc.IAM != IAMFR {
		t.Errorf("Expected IAM %s, got %s", IAMFR, acc.IAM)
	}

	now := time.Now()
	acc.MarkInvalid("400 Invalid signature", now)
	if acc.CredentialState != StateInvalid {
		t.Errorf("Expected state %s, got %s", StateInvalid, acc.CredentialState)
	}
	if acc.VerificationReason != "400 Invalid signature" {
		t.Errorf("Expected reason '400 Invalid signature', got '%s'", acc.VerificationReason)
	}

	acc.MarkVerified(now)
	if acc.CredentialState != StateVerified {
		t.Errorf("Expected state %s, got %s", StateVerified, acc.CredentialState)
	}
	if acc.VerificationReason != "" {
		t.Errorf("Expected cleared reason on verified, got '%s'", acc.VerificationReason)
	}
}
