package telegram

import (
	"testing"
)

func TestHumanizeHardware(t *testing.T) {
	tests := []struct {
		mem  string
		stor string
		want string
	}{
		{
			mem:  "ram-32g-ecc-2133",
			stor: "softraid-2x450nvme",
			want: "32G ECC ｜ 2×450G NVMe",
		},
		{
			mem:  "ram-32g-ecc-2133",
			stor: "softraid-4x2000sa",
			want: "32G ECC ｜ 4×2TB SATA",
		},
		{
			mem:  "ram-64g-ecc-2133",
			stor: "softraid-2x450nvme",
			want: "64G ECC ｜ 2×450G NVMe",
		},
		{
			mem:  "ram-128g-noecc-2666",
			stor: "softraid-2x960nvme",
			want: "128G Non-ECC ｜ 2×960G NVMe",
		},
		{
			mem:  "ram-64g",
			stor: "hybridsoftraid-2x2000sa-2x450nvme",
			want: "64G ｜ 2×2TB SATA + 2×450G NVMe",
		},
		{
			mem:  "",
			stor: "softraid-2x480ssd",
			want: "2×480G SSD",
		},
		{
			mem:  "32 GB",
			stor: "2x450nvme",
			want: "32 GB ｜ 2×450G NVMe",
		},
	}

	for _, tt := range tests {
		got := HumanizeHardware(tt.mem, tt.stor)
		if got != tt.want {
			t.Errorf("HumanizeHardware(%q, %q) = %q, want %q", tt.mem, tt.stor, got, tt.want)
		}
	}
}

func TestHumanizeOptionCode(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"ram-32g-ecc-2133", "32G ECC"},
		{"softraid-2x450nvme", "2×450G NVMe"},
		{"softraid-4x2000sa", "4×2TB SATA"},
		{"bandwidth-1000", "1 Gbps 带宽"},
		{"bandwidth-500", "500 Mbps 带宽"},
		{"traffic-25tb-1000", "25TB @ 1Gbps"},
		{"vrack-bandwidth-1000", "vRack 1 Gbps"},
	}

	for _, c := range cases {
		got := HumanizeOptionCode(c.code)
		if got != c.want {
			t.Errorf("HumanizeOptionCode(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}
