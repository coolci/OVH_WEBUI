package main

import "testing"

func TestLedgerRequiresActualMethodAndExplicitTest(t *testing.T) {
	registeredRoutes = map[string]bool{routeKey("GET", "/api/things/:id"): true}
	contract := `paths:
  /things/{id}:
    get:
      responses: {}
`
	row := Row{Method: "GET", CurrentPath: "/api/things/:thing", Target: "/things/:id", Evidence: "unrelated-test-file"}
	if deriveStatus(&row, contract) != "wip" {
		t.Fatal("unrelated evidence claimed completion")
	}
	row.Evidence = "go-test:./backend/internal/example#TestSpecific"
	testResults[row.Evidence] = true
	if deriveStatus(&row, contract) != "done" {
		t.Fatal("verified matching route not done")
	}
	row.Method = "POST"
	if deriveStatus(&row, contract) != "todo" {
		t.Fatal("wrong method accepted")
	}
	registeredRoutes[routeKey("POST", row.CurrentPath)] = true
	if deriveStatus(&row, contract) != "wip" {
		t.Fatal("wrong contract method accepted")
	}
	row.Method = "GET"
	testResults[row.Evidence] = false
	if deriveStatus(&row, contract) != "wip" {
		t.Fatal("failed test accepted")
	}
	if verifiedTest("go-test:../outside#TestOops") {
		t.Fatal("unsafe test path accepted")
	}
}
func TestRouteKeysPreserveWildcardSemantics(t *testing.T) {
	if routeKey("GET", "/api/:id") == routeKey("GET", "/api/*id") {
		t.Fatal("wildcard collapsed")
	}
}
