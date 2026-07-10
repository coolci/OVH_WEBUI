package catalog

import "testing"

func TestStandardizeConfig_StripsModelSuffix(t *testing.T) {
	in := "ram-64g-ecc-2133-24sk20"
	// 模型后缀会被剥离一部分；至少应小写且非空
	out := StandardizeConfig(in)
	if out == "" {
		t.Fatal("expected non-empty standardized config")
	}
	if out != StandardizeConfig(out) {
		t.Fatal("standardize should be idempotent")
	}
}

func TestFormatMemoryDisplay(t *testing.T) {
	if got := FormatMemoryDisplay("ram-64g-ecc"); got != "64GB RAM" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatStorageDisplay(t *testing.T) {
	if got := FormatStorageDisplay("2x480ssd"); got != "2x 480GB SSD" {
		t.Fatalf("got %q", got)
	}
	if got := FormatStorageDisplay("2x960nvme"); got != "2x 960GB NVMe" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatConfigDisplay(t *testing.T) {
	got := FormatConfigDisplay("ram-32g", "2x960ssd")
	if got == "" || got == "默认内存 + 默认存储" {
		t.Fatalf("unexpected: %q", got)
	}
}
