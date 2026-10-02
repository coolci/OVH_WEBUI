package types_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/ovh-webui/server/internal/types"
)

// TestOrderLimitsInSync verifies PRD F04.7 / D-28:
// The limits defined in backend/internal/types/orderlimits.go must strictly match
// src/lib/order-limits.ts.
func TestOrderLimitsInSync(t *testing.T) {
	// Look for src/lib/order-limits.ts from repo root
	// In test run, cwd is backend/internal/types
	candidates := []string{
		filepath.Join("..", "..", "..", "src", "lib", "order-limits.ts"),
		filepath.Join("..", "..", "src", "lib", "order-limits.ts"),
		filepath.Join("src", "lib", "order-limits.ts"),
	}

	var content []byte
	var foundPath string
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			content = data
			foundPath = p
			break
		}
	}

	if foundPath == "" {
		t.Fatalf("could not locate src/lib/order-limits.ts; searched: %v", candidates)
	}

	text := string(content)

	parseConst := func(name string) int {
		re := regexp.MustCompile(name + `\s*=\s*(\d+)`)
		m := re.FindStringSubmatch(text)
		if len(m) < 2 {
			t.Fatalf("failed to find %s in %s", name, foundPath)
		}
		val, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("failed to parse %s value %q: %v", name, m[1], err)
		}
		return val
	}

	tsMaxQuantity := parseConst("MAX_ORDER_QUANTITY")
	tsMaxFanout := parseConst("MAX_ORDER_FANOUT")
	tsMaxQueue := parseConst("MAX_QUEUE_SIZE")

	if types.MaxOrderQuantity != tsMaxQuantity {
		t.Errorf("MaxOrderQuantity mismatch: Go=%d, TS=%d", types.MaxOrderQuantity, tsMaxQuantity)
	}
	if types.MaxOrderFanout != tsMaxFanout {
		t.Errorf("MaxOrderFanout mismatch: Go=%d, TS=%d", types.MaxOrderFanout, tsMaxFanout)
	}
	if types.MaxQueueSize != tsMaxQueue {
		t.Errorf("MaxQueueSize mismatch: Go=%d, TS=%d", types.MaxQueueSize, tsMaxQueue)
	}
}

func TestClampOrderQuantity(t *testing.T) {
	// PRD acceptance requirement: 9999 -> clamped to 20
	if got := types.ClampOrderQuantity(9999); got != 20 {
		t.Errorf("ClampOrderQuantity(9999) = %d; want 20", got)
	}
	if got := types.ClampOrderQuantity(0); got != 1 {
		t.Errorf("ClampOrderQuantity(0) = %d; want 1", got)
	}
	if got := types.ClampOrderQuantity(-10); got != 1 {
		t.Errorf("ClampOrderQuantity(-10) = %d; want 1", got)
	}
	if got := types.ClampOrderQuantity(5); got != 5 {
		t.Errorf("ClampOrderQuantity(5) = %d; want 5", got)
	}
	if got := types.ClampOrderQuantity(20); got != 20 {
		t.Errorf("ClampOrderQuantity(20) = %d; want 20", got)
	}
	if got := types.ClampOrderQuantity(21); got != 20 {
		t.Errorf("ClampOrderQuantity(21) = %d; want 20", got)
	}
}
