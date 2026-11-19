package price

import "testing"

// 询价与下单必须同一口径:同周期内挑计价,不换周期(与 purchase.pickPricing 一致)。
func TestPickPricingNeverSubstitutesPeriod(t *testing.T) {
	yearlyOnly := []interface{}{map[string]interface{}{"duration": "P12M", "pricingMode": "upfront12", "pricingType": "rental"}}
	if d, m, ok := pickPricing(yearlyOnly, "P1M", ""); ok {
		t.Fatalf("monthly request substituted %s/%s", d, m)
	}
	both := append(yearlyOnly, map[string]interface{}{"duration": "P1M", "pricingMode": "default", "pricingType": "rental"})
	if d, m, ok := pickPricing(both, "P1M", ""); !ok || d != "P1M" || m != "default" {
		t.Fatalf("got %s/%s", d, m)
	}
}
