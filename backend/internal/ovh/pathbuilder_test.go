package ovh

import (
	"net/url"
	"testing"
)

func TestBuildPathAndWithQuery(t *testing.T) {
	p, err := BuildPath("me", "order", "12345", "details")
	if err != nil || p != "/me/order/12345/details" {
		t.Fatalf("BuildPath normal = (%q, %v)", p, err)
	}

	// CIDR / special characters are safely escaped within a single segment
	pCIDR, err := BuildPath("ip", "198.51.100.0/24", "mitigation", "198.51.100.1?foo=bar#frag")
	if err != nil {
		t.Fatalf("BuildPath CIDR unexpected error: %v", err)
	}
	wantCIDR := "/ip/198.51.100.0%2F24/mitigation/198.51.100.1%3Ffoo=bar%23frag"
	if pCIDR != wantCIDR {
		t.Fatalf("BuildPath escaped = %q, want %q", pCIDR, wantCIDR)
	}

	// Reject empty, dot, double-dot, percent-encoded dot, and control chars
	badInputs := [][]string{
		{},
		{"me", "", "order"},
		{"me", ".", "order"},
		{"me", "..", "order"},
		{"me", "%2e%2e", "order"},
		{"me", "%2E", "order"},
		{"me", "order\n123"},
	}
	for _, segs := range badInputs {
		if got, err := BuildPath(segs...); err == nil {
			t.Fatalf("expected error for segments %v, got %q", segs, got)
		}
	}

	// WithQuery
	if got := WithQuery("/me/order", nil); got != "/me/order" {
		t.Fatalf("WithQuery nil = %q", got)
	}
	q := url.Values{}
	q.Set("block", "198.51.100.0/24#evil&x=1")
	gotQ := WithQuery("/ip/198.51.100.0%2F24/mitigation", q)
	wantQ := "/ip/198.51.100.0%2F24/mitigation?block=198.51.100.0%2F24%23evil%26x%3D1"
	if gotQ != wantQ {
		t.Fatalf("WithQuery = %q, want %q", gotQ, wantQ)
	}
}
