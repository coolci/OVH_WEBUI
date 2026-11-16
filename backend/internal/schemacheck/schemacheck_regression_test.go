package schemacheck

import (
	"encoding/json"
	"go/parser"
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

func TestRegression_SchemacheckPathTemplateSafeBuilders(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{`ovh.BuildPath("dedicated", "server", svc, "ips")`, `/dedicated/server/{}/ips`},
		{`buildOVHPath("vps", svc, "reboot")`, `/vps/{}/reboot`},
		{`safePathFn("me", "order", orderID)`, `/me/order/{}`},
		{`buildOVHPathQuery(q, "dedicated", "server", svc, "tasks")`, `/dedicated/server/{}/tasks`},
		{`ovhPathOrAbort(c, "vps", svc, "snapshot")`, `/vps/{}/snapshot`},
		{`ovhPathQueryOrAbort(c, q, "me", "bill", billID)`, `/me/bill/{}`},
		{`ovh.WithQuery(buildOVHPath("me", "order"), q)`, `/me/order`},
		// Slice ellipsis and all-variable arguments must yield "{}" (never "/{}")
		{`buildOVHPath(baseSegs...)`, `{}`},
		{`buildOVHPathQuery(q, baseSegs...)`, `{}`},
		{`ovhPathOrAbort(c, baseSegs...)`, `{}`},
		{`ovhPathQueryOrAbort(c, q, baseSegs...)`, `{}`},
		{`buildOVHPath(segA, segB)`, `{}`},
		{`ovhPathOrAbort(c, segA, segB)`, `{}`},
		{`ovhPathQueryOrAbort(c, q, segA)`, `{}`},
		{`buildOVHPath()`, `{}`},
	}

	for _, tc := range cases {
		parsed, err := parser.ParseExpr(tc.expr)
		if err != nil {
			t.Fatalf("ParseExpr(%s): %v", tc.expr, err)
		}
		if got := pathTemplate(parsed); got != tc.want {
			t.Fatalf("pathTemplate(%s) = %q, want %q", tc.expr, got, tc.want)
		}
	}

	// Verify scanCalls handles multi-return safe helpers while ignoring baseSegs... / all-variable calls
	tmpDir := t.TempDir()
	sampleSrc := `package sample

func demo(c, q, cli any, svc string, baseSegs []string, segA string) {
	p1, ok := ovhPathOrAbort(c, "vps", svc, "ips")
	if !ok { return }
	cli.Get(p1, nil)

	p2, ok := ovhPathQueryOrAbort(c, q, "dedicated", "server", svc, "tasks")
	if !ok { return }
	cli.Get(p2, nil)

	p3, _ := buildOVHPath(baseSegs...)
	cli.Get(p3, nil)

	p4, _ := buildOVHPathQuery(q, baseSegs...)
	cli.Get(p4, nil)

	p5, _ := safePathFn(segA)
	cli.Get(p5, nil)
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "sample.go"), []byte(sampleSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	scanned := scanCalls(t, tmpDir)
	if _, ok := scanned["GET /vps/{}/ips"]; !ok {
		t.Fatalf("expected GET /vps/{}/ips in scanned calls, got %v", scanned)
	}
	if _, ok := scanned["GET /dedicated/server/{}/tasks"]; !ok {
		t.Fatalf("expected GET /dedicated/server/{}/tasks in scanned calls, got %v", scanned)
	}
	if _, bad := scanned["GET /{}"]; bad {
		t.Fatalf("must not emit bogus GET /{} from baseSegs... or all-variable calls: %v", scanned)
	}
	if len(scanned) != 2 {
		t.Fatalf("expected exactly 2 scanned endpoints, got %d: %v", len(scanned), scanned)
	}
}
