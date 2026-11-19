package catalogwatch

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestExpandExclusiveOptionalAndStableKeys(t *testing.T) {
	raw := `{"plans":[{"planCode":"p","addonFamilies":[{"name":"memory","addons":["ram32","ram64"],"exclusive":true,"mandatory":true},{"name":"network","addons":["net1","net2"],"exclusive":true,"mandatory":false}],"configurations":[{"name":"dedicated_datacenter","values":["gra","rbx"]}]}]}`
	var c Catalog
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	expanded, err := Expand(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded) != 6 {
		t.Fatalf("got %d groups", len(expanded))
	}
	for _, v := range expanded {
		if len(v.Datacenters) != 2 {
			t.Fatal("DCs must remain alternatives")
		}
		if len(v.Options) > 2 {
			t.Fatal("exclusive family combined")
		}
	}
	c.Plans[0].Families[0].Addons = []string{"ram64", "ram32"}
	again, _ := Expand(c)
	if !reflect.DeepEqual(expanded, again) {
		t.Fatal("ordering changed signatures")
	}
	if key("a", Candidate{PlanCode: "p", Options: []string{"ram32"}, Datacenters: []string{"gra"}}) != key("a", Candidate{PlanCode: "p", Options: []string{"ram32"}, Datacenters: []string{"rbx"}}) {
		t.Fatal("DC must not change one-order group")
	}
}
func TestExpandRejectsInvalidSnapshot(t *testing.T) {
	for _, raw := range []string{`{"plans":[]}`, `{"plans":[{"planCode":"p"}]}`, `{"plans":[{"planCode":"a,b"}]}`} {
		var c Catalog
		_ = json.Unmarshal([]byte(raw), &c)
		if _, err := Expand(c); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestOfficialIECatalogShapeExcludesOptionalLicensePowersets(t *testing.T) {
	raw, err := os.ReadFile("testdata/eco-ie-ordering-shape.json")
	if err != nil {
		t.Fatal(err)
	}
	var c Catalog
	if err = json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	candidates, err := Expand(c)
	if err != nil {
		t.Fatal(err)
	}
	plans := map[string]bool{}
	for _, v := range candidates {
		plans[v.PlanCode] = true
		for _, o := range v.Options {
			if strings.Contains(o, "license") {
				t.Fatalf("software licensing generated machine order: %s", o)
			}
		}
	}
	if len(plans) != len(c.Plans) {
		t.Fatalf("lost plans: %d/%d", len(plans), len(c.Plans))
	}
	t.Logf("Parsed official IE shape: %d plans, %d hardware/network configurations", len(plans), len(candidates))
}
