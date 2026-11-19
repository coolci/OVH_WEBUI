package datacenter

import "testing"

func TestCanonicalFamilyAndBidirectionalMappings(t *testing.T) {
	equivalentPairs := [][2]string{
		{"mum", "ynm"},
		{"YNM", "ap-south-mum"},
		{"yyz", "tor"},
		{"ca-east-tor-a", "tor"},
		{"yyz", "ca-east-tor-a"},
		{"eu-west-par-a", "par-a"},
		{"eu-west-par-b", "par-b"},
		{"eu-west-par-c", "par-c"},
		{"eu-west-par-a", "par"},
		{"eu-west-lim", "de"},
		{"lim", "de"},
		{"fra", "lim"},
		{"eu-west-eri", "uk"},
		{"eri", "uk"},
		{"lon", "eri"},
		{"ap-southeast-sgp", "sgp"},
		{"ap-southeast-syd", "syd"},
		{"us-east-vin", "vin"},
		{"us-east-vin-a", "vin"},
		{"US-EAST-VA", "vin"},
		{"us-west-hil", "hil"},
		{"US-WEST-OR", "hil"},
		{"gra3", "gra"},
		{"rbx8", "eu-west-rbx"},
		{"bhs5", "ca-east-bhs"},
	}
	for _, pair := range equivalentPairs {
		if !SameZoneOrAlias(pair[0], pair[1]) {
			t.Errorf("expected SameZoneOrAlias(%q, %q) == true (families: %q vs %q)",
				pair[0], pair[1], CanonicalFamily(pair[0]), CanonicalFamily(pair[1]))
		}
		if Contradicts(pair[0], pair[1]) {
			t.Errorf("expected Contradicts(%q, %q) == false", pair[0], pair[1])
		}
	}

	contradictingPairs := [][2]string{
		{"gra", "rbx"},
		{"ynm", "sgp"},
		{"par-a", "par-b"},
		{"eu-west-par-a", "eu-west-par-b"},
		{"eu-west-lz-ams", "eu-west-lz-mad"},
		{"vin", "hil"},
		{"de", "uk"},
	}
	for _, pair := range contradictingPairs {
		if SameZoneOrAlias(pair[0], pair[1]) {
			t.Errorf("expected SameZoneOrAlias(%q, %q) == false", pair[0], pair[1])
		}
		if !Contradicts(pair[0], pair[1]) {
			t.Errorf("expected Contradicts(%q, %q) == true", pair[0], pair[1])
		}
	}
}

func TestInventoryDisplayOrderAndVPSConversions(t *testing.T) {
	if got := InventoryCode(""); got != "" {
		t.Fatalf("InventoryCode empty = %q, want empty", got)
	}
	if got := InventoryCode("MUM"); got != "ynm" {
		t.Fatalf("InventoryCode(MUM) = %q, want ynm", got)
	}
	if got := InventoryCode("tor"); got != "ca-east-tor-a" {
		t.Fatalf("InventoryCode(tor) = %q, want ca-east-tor-a", got)
	}
	if got := InventoryCode("yyz"); got != "ca-east-tor-a" {
		t.Fatalf("InventoryCode(yyz) = %q, want ca-east-tor-a", got)
	}
	if got := InventoryCode("par-b"); got != "eu-west-par-b" {
		t.Fatalf("InventoryCode(par-b) = %q, want eu-west-par-b", got)
	}
	if got := DisplayCode("ynm"); got != "mum" {
		t.Fatalf("DisplayCode(ynm) = %q, want mum", got)
	}
	if got := DisplayCode("eu-west-par-c"); got != "par-c" {
		t.Fatalf("DisplayCode(eu-west-par-c) = %q, want par-c", got)
	}
	if got := DisplayCode("ca-east-tor-a"); got != "tor" {
		t.Fatalf("DisplayCode(ca-east-tor-a) = %q, want tor", got)
	}
	if got := OrderCode("eu-west-par-a"); got != "par" {
		t.Fatalf("OrderCode(eu-west-par-a) = %q, want par", got)
	}
	if got := OrderCode("mum"); got != "ynm" {
		t.Fatalf("OrderCode(mum) = %q, want ynm", got)
	}
	if got := VPSShortCode("eu-west-lim"); got != "DE" {
		t.Fatalf("VPSShortCode(eu-west-lim) = %q, want DE", got)
	}
	if got := VPSShortCode("eu-west-eri"); got != "UK" {
		t.Fatalf("VPSShortCode(eu-west-eri) = %q, want UK", got)
	}
	if got := VPSShortCode("ap-south-mum"); got != "YNM" {
		t.Fatalf("VPSShortCode(ap-south-mum) = %q, want YNM", got)
	}
}
