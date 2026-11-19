package catalog

import (
	"reflect"
	"testing"
)

func TestCustomPlanStockHardwarePreservesExactConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, plan, fqn, memory, storage string
		want                             []string
	}{
		{"custom mixed case and dotted code", "custom-NewModel.LZ", "custom-NewModel.LZ.ram-64g.softraid-2x960nvme", "", "", []string{"ram-64g", "softraid-2x960nvme"}},
		{"mixed storage stays distinct", "custom", "custom.ram-64g.softraid-2x960nvme-2x6000sa", "", "", []string{"ram-64g", "softraid-2x960nvme-2x6000sa"}},
		{"fallback only from reported hardware", "custom", "", "ram-64g", "N/A", []string{"ram-64g"}},
		{"duplicate stock segments", "custom", "custom.ram-64g.ram-64g", "", "", []string{"ram-64g"}},
		{"no hardware invented", "custom", "custom", "", "", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stockHardwareOptions(tc.plan, tc.fqn, tc.memory, tc.storage); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCustomInventoryOnlyAcceptsExactModelSuffix(t *testing.T) {
	raw := []string{"ram-64g", "softraid-2x960nvme"}
	selected := []string{"ram-64g-custom-plan", "softraid-2x960nvme-2x6000sa-custom-plan"}
	got := CustomInventoryOptions("custom-plan", raw, selected)
	want := []string{"ram-64g-custom-plan", "softraid-2x960nvme"}
	if !reflect.DeepEqual(got, want) || raw[0] != "ram-64g" {
		t.Fatalf("ambiguous storage matched or source mutated: got=%v raw=%v", got, raw)
	}
}
