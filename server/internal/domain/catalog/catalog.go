// Package catalog provides pure domain entities and invariants for OVH product catalog and pricing.
// Pure domain: zero external dependencies outside standard library.
package catalog

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrEmptyPlanCode = errors.New("plan code cannot be empty")
	ErrInvalidPrice  = errors.New("price cannot be negative")
)

// DatacenterRegion maps OVH datacenter codes to geographic regions.
var DatacenterRegion = map[string]string{
	"GRA": "FR", "SBG": "FR", "RBX": "FR", "PAR": "FR",
	"BHS": "CA", "WAW": "PL", "FRA": "DE", "LON": "GB",
	"SYD": "AU", "SGP": "SG", "VIN": "US", "HIL": "US",
}

// PlanSpec represents parsed characteristics of an OVH server plan.
type PlanSpec struct {
	PlanCode  string `json:"planCode"`
	Line      string `json:"line"`   // kimsufi, soyoustart, rise, advance, scale, hgr
	Family    string `json:"family"` // sk, sys, rise, etc.
	MemoryGB  int    `json:"memoryGb"`
	StorageGB int    `json:"storageGb"`
}

// NormalizePlanCode trims whitespace and standardizes plan code casing.
func NormalizePlanCode(code string) (string, error) {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "" {
		return "", ErrEmptyPlanCode
	}
	return c, nil
}

// RegionOfDatacenter derives geographic country code from datacenter identifier.
func RegionOfDatacenter(dc string) string {
	dcUpper := strings.ToUpper(strings.TrimSpace(dc))
	if reg, ok := DatacenterRegion[dcUpper]; ok {
		return reg
	}
	return "UNKNOWN"
}

// PriceBreakdown encapsulates tax-exclusive and tax-inclusive pricing amounts.
type PriceBreakdown struct {
	Currency string  `json:"currency"`
	Net      float64 `json:"net"`
	Gross    float64 `json:"gross"`
	TaxRate  float64 `json:"taxRate"`
}

// NewPriceBreakdown validates and creates a PriceBreakdown domain entity.
func NewPriceBreakdown(currency string, net, taxRate float64) (*PriceBreakdown, error) {
	if net < 0 {
		return nil, ErrInvalidPrice
	}
	if taxRate < 0 {
		taxRate = 0
	}
	gross := net * (1 + taxRate)
	return &PriceBreakdown{
		Currency: strings.ToUpper(strings.TrimSpace(currency)),
		Net:      net,
		Gross:    gross,
		TaxRate:  taxRate,
	}, nil
}

// Format formats the net price with currency symbol.
func (p *PriceBreakdown) Format() string {
	return fmt.Sprintf("%.2f %s", p.Net, p.Currency)
}
