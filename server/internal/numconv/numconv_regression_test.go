package numconv

import (
	"encoding/json"
	"math"
	"testing"
)

func TestRegression_BUG_OVH_15_ToFloat64RejectsNaNAndInf(t *testing.T) {
	invalidInputs := []interface{}{
		"NaN",
		"nan",
		"+Inf",
		"-Inf",
		"Infinity",
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
		float32(math.NaN()),
		float32(math.Inf(1)),
		json.Number("NaN"),
		json.Number("+Inf"),
	}
	for _, in := range invalidInputs {
		if v, ok := ToFloat64(in); ok || v != 0 {
			t.Fatalf("expected ToFloat64(%v) to return (0, false), got (%v, %v)", in, v, ok)
		}
	}

	if v, ok := ToFloat64("19.99"); !ok || v != 19.99 {
		t.Fatalf("expected valid float 19.99, got (%v, %v)", v, ok)
	}
}
