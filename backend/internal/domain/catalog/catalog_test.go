package catalog_test

import (
	"testing"

	"github.com/ovh-webui/server/internal/domain/catalog"
)

func TestNormalizePlanCode(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr error
	}{
		{" 24SKA01 ", "24ska01", nil},
		{"24SYS01", "24sys01", nil},
		{"", "", catalog.ErrEmptyPlanCode},
		{"   ", "", catalog.ErrEmptyPlanCode},
	}

	for _, tt := range tests {
		got, err := catalog.NormalizePlanCode(tt.input)
		if tt.wantErr != nil {
			if err != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			continue
		}
		if got != tt.want {
			t.Errorf("NormalizePlanCode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRegionOfDatacenter(t *testing.T) {
	if reg := catalog.RegionOfDatacenter("gra"); reg != "FR" {
		t.Errorf("expected FR for gra, got %s", reg)
	}
	if reg := catalog.RegionOfDatacenter("bhs"); reg != "CA" {
		t.Errorf("expected CA for bhs, got %s", reg)
	}
	if reg := catalog.RegionOfDatacenter("xyz"); reg != "UNKNOWN" {
		t.Errorf("expected UNKNOWN for xyz, got %s", reg)
	}
}

func TestNewPriceBreakdown(t *testing.T) {
	p, err := catalog.NewPriceBreakdown("EUR", 25.50, 0.20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Gross < 30.59 || p.Gross > 30.61 {
		t.Errorf("expected gross ~30.60, got %.2f", p.Gross)
	}
	if p.Format() != "25.50 EUR" {
		t.Errorf("expected '25.50 EUR', got %s", p.Format())
	}

	// Negative net price rejected
	if _, err := catalog.NewPriceBreakdown("EUR", -10, 0.20); err != catalog.ErrInvalidPrice {
		t.Fatalf("expected ErrInvalidPrice, got %v", err)
	}
}
