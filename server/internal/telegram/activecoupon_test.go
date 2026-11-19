package telegram

import (
	"testing"
)

func TestActiveCoupon(t *testing.T) {
	_ = ClearActiveCoupon(nil)
	if got := ActiveCoupon(nil); got != "" {
		t.Fatalf("expected empty coupon, got %q", got)
	}

	if err := SetActiveCoupon(nil, "SAVE10"); err != nil {
		t.Fatalf("SetActiveCoupon failed: %v", err)
	}
	if got := ActiveCoupon(nil); got != "SAVE10" {
		t.Fatalf("expected SAVE10, got %q", got)
	}

	if err := ClearActiveCoupon(nil); err != nil {
		t.Fatalf("ClearActiveCoupon failed: %v", err)
	}
	if got := ActiveCoupon(nil); got != "" {
		t.Fatalf("expected empty coupon after clear, got %q", got)
	}
}
