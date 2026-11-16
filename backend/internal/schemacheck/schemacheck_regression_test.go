package schemacheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegression_BUG_OVH_17_SchemacheckNormalizesUSOrderDetailAndDedupesSupport(t *testing.T) {
	// 1. Verify "support" appears exactly once in namespaces
	supportCount := 0
	for _, ns := range namespaces {
		if ns == "support" {
			supportCount++
		}
	}
	if supportCount != 1 {
		t.Fatalf("expected 'support' once in namespaces, got %d", supportCount)
	}

	// 2. Verify testdata/ovh-endpoints.json includes CA, EU, and US under GET /me/order/{orderId}/details/{orderDetailId}
	raw, err := os.ReadFile(filepath.Join("testdata", "ovh-endpoints.json"))
	if err != nil {
		t.Fatal(err)
	}
	var base baseline
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatal(err)
	}
	detailEp := base["GET /me/order/{orderId}/details/{orderDetailId}"]
	if _, ok := detailEp["CA"]; !ok {
		t.Fatal("missing CA in GET /me/order/{orderId}/details/{orderDetailId}")
	}
	if _, ok := detailEp["EU"]; !ok {
		t.Fatal("missing EU in GET /me/order/{orderId}/details/{orderDetailId}")
	}
	if _, ok := detailEp["US"]; !ok {
		t.Fatal("missing US in GET /me/order/{orderId}/details/{orderDetailId}")
	}

	// 3. Verify collectWithFetcher normalizes US {detailId} into {orderDetailId}
	origUsed := usedEndpoints
	origRemoved := removedEndpoints
	defer func() {
		usedEndpoints = origUsed
		removedEndpoints = origRemoved
	}()
	usedEndpoints = []string{"GET /me/order/{orderId}/details/{orderDetailId}"}
	removedEndpoints = nil

	got := collectWithFetcher(t, func(region, ns string) (map[string]interface{}, error) {
		if ns != "me" {
			return map[string]interface{}{"apis": []interface{}{}}, nil
		}
		paramName := "orderDetailId"
		path := "/me/order/{orderId}/details/{orderDetailId}"
		if region == "US" {
			paramName = "detailId"
			path = "/me/order/{orderId}/details/{detailId}"
		}
		return map[string]interface{}{
			"apis": []interface{}{
				map[string]interface{}{
					"path": path,
					"operations": []interface{}{
						map[string]interface{}{
							"httpMethod":   "GET",
							"responseType": "billing.OrderDetail",
							"apiStatus":    map[string]interface{}{"value": "PRODUCTION"},
							"parameters": []interface{}{
								map[string]interface{}{"paramType": "path", "name": "orderId", "dataType": "long", "required": true},
								map[string]interface{}{"paramType": "path", "name": paramName, "dataType": "long", "required": true},
							},
						},
					},
				},
			},
		}, nil
	})
	regionsGot := got["GET /me/order/{orderId}/details/{orderDetailId}"]
	for _, r := range []string{"CA", "EU", "US"} {
		if _, ok := regionsGot[r]; !ok {
			t.Fatalf("collectWithFetcher lost region %s: %+v", r, regionsGot)
		}
	}
}
