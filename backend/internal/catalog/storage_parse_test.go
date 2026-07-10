package catalog

import "testing"

func TestParseStorage_SASNotSA(t *testing.T) {
	got := ParseStorageAddonDisplay("softraid-2x22000sas-24advstor01-v3")
	if got != "SOFTRAID 2x 22000GB SAS" {
		t.Fatalf("sas misparsed: got %q", got)
	}
	got = ParseStorageAddonDisplay("softraid-4x14000sas-24risestor")
	if got != "SOFTRAID 4x 14000GB SAS" {
		t.Fatalf("got %q", got)
	}
}

func TestParseStorage_SATA_SA(t *testing.T) {
	got := ParseStorageAddonDisplay("softraid-2x4000sa-24rise01-v1")
	if got != "SOFTRAID 2x 4000GB SATA" {
		t.Fatalf("got %q", got)
	}
}

func TestParseStorage_NVMe(t *testing.T) {
	got := ParseStorageAddonDisplay("softraid-2x960nvme-pcie-gen4-24adv01-v3")
	if got != "SOFTRAID 2x 960GB NVMe" {
		t.Fatalf("got %q", got)
	}
}

func TestParseStorage_SSD(t *testing.T) {
	got := ParseStorageAddonDisplay("softraid-2x480ssd-system-24risestor")
	if got != "SOFTRAID 2x 480GB SSD" {
		t.Fatalf("got %q", got)
	}
}

func TestParseStorage_Hybrid_NVMe_then_SA(t *testing.T) {
	got := ParseStorageAddonDisplay("hybridsoftraid-2x450nvme-1x4000sa-24skgame")
	want := "混合RAID 2x 450GB NVMe + 1x 4000GB SATA"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseStorage_Hybrid_DualNVMe(t *testing.T) {
	got := ParseStorageAddonDisplay("hybridsoftraid-2x960nvme-pcie-gen4-2x1920nvme-pcie-gen4-24adv01-v3")
	want := "混合RAID 2x 960GB NVMe + 2x 1920GB NVMe"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseStorage_Hybrid_SA_then_NVMe(t *testing.T) {
	got := ParseStorageAddonDisplay("hybridsoftraid-4x4000sa-1x500nvme-24example")
	want := "混合RAID 4x 4000GB SATA + 1x 500GB NVMe"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseStorage_0Disk(t *testing.T) {
	got := ParseStorageAddonDisplay("softraid-0disk-24rise")
	if got != "未配置数据盘 (0 disk)" {
		t.Fatalf("got %q", got)
	}
}

func TestParseStorage_HardRAID_SAS(t *testing.T) {
	got := ParseStorageAddonDisplay("hardraid-2x22000sas-24advstor01-v3")
	if got != "HARDRAID 2x 22000GB SAS" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatStorageDisplay_Short(t *testing.T) {
	if got := FormatStorageDisplay("2x480ssd"); got != "2x 480GB SSD" {
		t.Fatalf("got %q", got)
	}
	if got := FormatStorageDisplay("2x22000sas"); got != "2x 22000GB SAS" {
		t.Fatalf("got %q", got)
	}
	if got := FormatStorageDisplay("2x4000sa"); got != "2x 4000GB SATA" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatMediaLabel(t *testing.T) {
	if FormatMediaLabel("sa") != "SATA" {
		t.Fatal("sa -> SATA")
	}
	if FormatMediaLabel("sas") != "SAS" {
		t.Fatal("sas -> SAS")
	}
}
