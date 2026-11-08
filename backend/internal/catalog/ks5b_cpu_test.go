package catalog

import (
	"encoding/json"
	"testing"
)

// TestExtractCPUFromProduct 验证从 catalog products 顶层字典中提取 CPU 型号（覆盖 KS-5-B 等缺少 | CPU 后缀的机型）
func TestExtractCPUFromProduct(t *testing.T) {
	catalogJSON := `{
		"plans": [
			{
				"planCode": "26sk50b-v1",
				"invoiceName": "KS-5-B",
				"product": "26sk50b"
			},
			{
				"planCode": "24sk602",
				"invoiceName": "KS-6 | AMD Epyc 7351P",
				"product": "24sk60"
			},
			{
				"planCode": "26sk50a-v1",
				"invoiceName": "KS-5-A | Intel Xeon E-2274G",
				"product": "26sk50a"
			},
			{
				"planCode": "test-desc-only",
				"invoiceName": "TEST-DESC",
				"product": "test-desc"
			}
		],
		"products": [
			{
				"name": "26sk50b",
				"description": "Intel Xeon E5-1650v4",
				"blobs": {
					"technical": {
						"server": {
							"cpu": {
								"brand": "Intel",
								"model": "Xeon E5-1650v4",
								"cores": 6,
								"threads": 12,
								"frequency": 3.6,
								"boost": 4
							}
						}
					}
				}
			},
			{
				"name": "24sk60",
				"description": "AMD Epyc 7351p",
				"blobs": {
					"technical": {
						"server": {
							"cpu": {
								"brand": "AMD",
								"model": "Epyc 7351p",
								"cores": 16
							}
						}
					}
				}
			},
			{
				"name": "26sk50a",
				"description": "Intel Xeon E-2274G",
				"blobs": {
					"technical": {
						"server": {
							"cpu": {
								"brand": "Intel",
								"model": "Intel Xeon E-2274G",
								"cores": 4
							}
						}
					}
				}
			},
			{
				"name": "test-desc",
				"description": "AMD Ryzen 5 3600",
				"blobs": {}
			}
		]
	}`

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(catalogJSON), &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	productsRaw, _ := raw["products"].([]interface{})
	productByName := make(map[string]map[string]interface{}, len(productsRaw))
	for _, pr := range productsRaw {
		if pm, ok := pr.(map[string]interface{}); ok {
			if name, ok := pm["name"].(string); ok && name != "" {
				productByName[name] = pm
			}
		}
	}

	testCases := []struct {
		prodKey  string
		expected string
		desc     string
	}{
		{"26sk50b", "Intel Xeon E5-1650v4", "KS-5-B (brand + model拼接)"},
		{"24sk60", "AMD Epyc 7351p", "KS-6 (AMD + Epyc拼接)"},
		{"26sk50a", "Intel Xeon E-2274G", "KS-5-A (model已含brand防重复拼接)"},
		{"test-desc", "AMD Ryzen 5 3600", "Fallback至description"},
	}

	for _, tc := range testCases {
		prod := productByName[tc.prodKey]
		actual := productCPU(prod)
		if actual != tc.expected {
			t.Errorf("[%s] 期望 %q, 得到 %q", tc.desc, tc.expected, actual)
		} else {
			t.Logf("PASS: [%s] -> %q", tc.desc, actual)
		}
	}
}
