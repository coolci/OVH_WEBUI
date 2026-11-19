package datacenter

import "strings"

// Code represents a canonical OVH datacenter site family identifier.
type Code string

// Spec describes the canonical forms of an OVH datacenter or availability zone.
type Spec struct {
	Family        Code
	InventoryCode string
	OrderCode     string
	DisplayCode   string
	VPSShortCode  string
}

var familySpecs = map[Code]Spec{
	"gra": {Family: "gra", InventoryCode: "gra", OrderCode: "gra", DisplayCode: "gra", VPSShortCode: "GRA"},
	"sbg": {Family: "sbg", InventoryCode: "sbg", OrderCode: "sbg", DisplayCode: "sbg", VPSShortCode: "SBG"},
	"rbx": {Family: "rbx", InventoryCode: "rbx", OrderCode: "rbx", DisplayCode: "rbx", VPSShortCode: "EU-WEST-RBX"},
	"par": {Family: "par", InventoryCode: "par", OrderCode: "par", DisplayCode: "par", VPSShortCode: "PAR"},
	"bhs": {Family: "bhs", InventoryCode: "bhs", OrderCode: "bhs", DisplayCode: "bhs", VPSShortCode: "BHS"},
	"tor": {Family: "tor", InventoryCode: "ca-east-tor-a", OrderCode: "tor", DisplayCode: "tor", VPSShortCode: "TOR"},
	"ynm": {Family: "ynm", InventoryCode: "ynm", OrderCode: "ynm", DisplayCode: "mum", VPSShortCode: "YNM"},
	"waw": {Family: "waw", InventoryCode: "waw", OrderCode: "waw", DisplayCode: "waw", VPSShortCode: "WAW"},
	"de":  {Family: "de", InventoryCode: "fra", OrderCode: "de", DisplayCode: "fra", VPSShortCode: "DE"},
	"uk":  {Family: "uk", InventoryCode: "lon", OrderCode: "uk", DisplayCode: "lon", VPSShortCode: "UK"},
	"mil": {Family: "mil", InventoryCode: "mil", OrderCode: "mil", DisplayCode: "mil", VPSShortCode: "EU-SOUTH-MIL"},
	"sgp": {Family: "sgp", InventoryCode: "sgp", OrderCode: "sgp", DisplayCode: "sgp", VPSShortCode: "SGP"},
	"syd": {Family: "syd", InventoryCode: "syd", OrderCode: "syd", DisplayCode: "syd", VPSShortCode: "SYD"},
	"vin": {Family: "vin", InventoryCode: "vin", OrderCode: "vin", DisplayCode: "vin", VPSShortCode: "US-EAST-VA"},
	"hil": {Family: "hil", InventoryCode: "hil", OrderCode: "hil", DisplayCode: "hil", VPSShortCode: "US-WEST-OR"},
}

var exactAliasToFamily = map[string]Code{
	// Gravelines
	"gra":         "gra",
	"eu-west-gra": "gra",
	// Strasbourg
	"sbg":         "sbg",
	"eu-west-sbg": "sbg",
	// Roubaix
	"rbx":         "rbx",
	"rbx-hz":      "rbx",
	"eu-west-rbx": "rbx",
	// Paris
	"par":           "par",
	"par-a":         "par",
	"par-b":         "par",
	"par-c":         "par",
	"eu-west-par":   "par",
	"eu-west-par-a": "par",
	"eu-west-par-b": "par",
	"eu-west-par-c": "par",
	// Beauharnois
	"bhs":         "bhs",
	"ca-east-bhs": "bhs",
	// Toronto
	"tor":           "tor",
	"yyz":           "tor",
	"ca-east-tor":   "tor",
	"ca-east-tor-a": "tor",
	"ca-east-tor-b": "tor",
	"ca-east-tor-c": "tor",
	// Mumbai
	"ynm":            "ynm",
	"mum":            "ynm",
	"ap-south-mum":   "ynm",
	"ap-south-mum-a": "ynm",
	// Warsaw
	"waw":            "waw",
	"eu-central-waw": "waw",
	// Germany (Limburg / Frankfurt)
	"de":             "de",
	"lim":            "de",
	"fra":            "de",
	"eu-west-lim":    "de",
	"eu-central-lim": "de",
	"eu-central-fra": "de",
	// UK (Erith / London)
	"uk":          "uk",
	"gb":          "uk",
	"eri":         "uk",
	"lon":         "uk",
	"eu-west-eri": "uk",
	"eu-west-lon": "uk",
	// Milan
	"mil":          "mil",
	"eu-south-mil": "mil",
	// Singapore
	"sgp":                "sgp",
	"sg":                 "sgp",
	"ap-southeast-sgp":   "sgp",
	"ap-southeast-sgp-a": "sgp",
	// Sydney
	"syd":                "syd",
	"au":                 "syd",
	"ap-southeast-syd":   "syd",
	"ap-southeast-syd-a": "syd",
	// Vint Hill (Virginia, US East)
	"vin":           "vin",
	"us-east-vin":   "vin",
	"us-east-vin-a": "vin",
	"us-east-va":    "vin",
	// Hillsboro (Oregon, US West)
	"hil":           "hil",
	"us-west-hil":   "hil",
	"us-west-hil-a": "hil",
	"us-west-or":    "hil",
}

// Parse normalizes raw into a canonical site family Code and optional availability zone suffix ("a", "b", "c", ...).
func Parse(raw string) (family Code, az string) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", ""
	}
	// Preserve Local Zone distinctness (e.g. eu-west-lz-ams, eu-west-lz-mad).
	if strings.Contains(s, "-lz-") {
		return Code(s), ""
	}

	az = extractAZ(s)
	if fam, ok := exactAliasToFamily[s]; ok {
		return fam, az
	}

	// Strip trailing digits from numbered rack/building codes such as gra3, rbx8, bhs5, sgp02, lim1.
	stripped := stripTrailingDigits(s)
	if stripped != s && len(stripped) >= 3 {
		if fam, ok := exactAliasToFamily[stripped]; ok {
			return fam, az
		}
	}

	// Multi-segment region-city(-az) codes such as ap-southeast-sgp-b or eu-west-par-d.
	parts := strings.Split(s, "-")
	if len(parts) >= 3 {
		idx := len(parts) - 1
		if len(parts[idx]) == 1 && parts[idx][0] >= 'a' && parts[idx][0] <= 'z' {
			idx--
		}
		if idx >= 0 && len(parts[idx]) >= 3 {
			city := stripTrailingDigits(parts[idx])
			if fam, ok := exactAliasToFamily[city]; ok {
				return fam, az
			}
		}
	}

	return Code(s), az
}

func extractAZ(lower string) string {
	parts := strings.Split(lower, "-")
	if len(parts) < 2 {
		return ""
	}
	last := parts[len(parts)-1]
	if len(last) == 1 && last[0] >= 'a' && last[0] <= 'z' {
		return last
	}
	return ""
}

func stripTrailingDigits(s string) string {
	end := len(s)
	for end > 0 && s[end-1] >= '0' && s[end-1] <= '9' {
		end--
	}
	return s[:end]
}

// CanonicalFamily returns the canonical site family Code (e.g. "mum"/"ynm"/"ap-south-mum" -> "ynm",
// "eu-west-par-a"/"par-a"/"par" -> "par", "yyz"/"ca-east-tor-a"/"tor" -> "tor").
func CanonicalFamily(raw string) Code {
	fam, _ := Parse(raw)
	return fam
}

// InventoryCode converts a display/short/alias code into the OVH dedicated server inventory/cart API code,
// preserving availability zones (e.g. "par-a" -> "eu-west-par-a", "mum" -> "ynm", "tor"/"yyz" -> "ca-east-tor-a").
// Empty input returns "".
func InventoryCode(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	if strings.Contains(s, "-lz-") {
		return s
	}
	switch s {
	case "mum", "ynm", "ap-south-mum", "ap-south-mum-a":
		return "ynm"
	case "tor", "yyz", "ca-east-tor", "ca-east-tor-a":
		return "ca-east-tor-a"
	case "ca-east-tor-b", "ca-east-tor-c":
		return s
	case "par-a", "eu-west-par-a":
		return "eu-west-par-a"
	case "par-b", "eu-west-par-b":
		return "eu-west-par-b"
	case "par-c", "eu-west-par-c":
		return "eu-west-par-c"
	case "eu-west-par":
		return "par"
	case "ap-southeast-sgp", "ap-southeast-sgp-a":
		return "sgp"
	case "ap-southeast-syd", "ap-southeast-syd-a":
		return "syd"
	case "us-east-vin", "us-east-va":
		return "vin"
	case "us-west-hil", "us-west-or":
		return "hil"
	case "eu-west-gra":
		return "gra"
	case "eu-west-rbx":
		return "rbx"
	case "eu-west-sbg":
		return "sbg"
	case "ca-east-bhs":
		return "bhs"
	case "eu-central-waw":
		return "waw"
	case "eu-south-mil":
		return "mil"
	case "eu-west-lim", "eu-central-lim":
		return "lim"
	case "eu-west-eri":
		return "eri"
	}
	return s
}

// OrderCode returns the canonical OVH order site code (e.g. "ynm", "tor", "par", "de", "uk", "sgp", "syd", "vin", "hil").
func OrderCode(raw string) string {
	fam, _ := Parse(raw)
	if fam == "" {
		return ""
	}
	if spec, ok := familySpecs[fam]; ok {
		return spec.OrderCode
	}
	return string(fam)
}

// DisplayCode converts an API/order/VPS datacenter code into the frontend canonical display code
// (e.g. "ynm" -> "mum", "ca-east-tor-a"/"yyz" -> "tor", "eu-west-par-a" -> "par-a").
func DisplayCode(raw string) string {
	fam, az := Parse(raw)
	if fam == "" {
		return ""
	}
	if fam == "par" && az != "" {
		return "par-" + az
	}
	if spec, ok := familySpecs[fam]; ok {
		return spec.DisplayCode
	}
	return string(fam)
}

// VPSShortCode returns the OVH VPS catalog datacenter code (e.g. "DE", "UK", "YNM", "SGP", "SYD", "US-EAST-VA").
func VPSShortCode(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.Contains(strings.ToLower(s), "-lz-") {
		return strings.ToUpper(s)
	}
	fam, _ := Parse(s)
	if spec, ok := familySpecs[fam]; ok && spec.VPSShortCode != "" {
		return spec.VPSShortCode
	}
	return strings.ToUpper(s)
}

// SameFamily reports whether two datacenter codes belong to the same site family (ignoring AZ sub-zones).
func SameFamily(a, b string) bool {
	fa, _ := Parse(a)
	fb, _ := Parse(b)
	return fa != "" && fb != "" && fa == fb
}

// SameZoneOrAlias reports whether a and b refer to the same datacenter/alias,
// respecting explicit availability zones when BOTH inputs specify an AZ (e.g. "par-a" == "eu-west-par-a",
// "par" == "eu-west-par-a", "tor" == "ca-east-tor-a", but "par-a" != "eu-west-par-b").
func SameZoneOrAlias(a, b string) bool {
	fa, azA := Parse(a)
	fb, azB := Parse(b)
	if fa == "" || fb == "" || fa != fb {
		return false
	}
	if azA != "" && azB != "" && azA != azB {
		return false
	}
	return true
}

// Contradicts reports whether an observed datacenter token in an order/inventory record
// contradicts targetDC. Empty observed or target never contradicts.
func Contradicts(observed, targetDC string) bool {
	fa, _ := Parse(observed)
	fb, _ := Parse(targetDC)
	if fa == "" || fb == "" {
		return false
	}
	return !SameZoneOrAlias(observed, targetDC)
}
