package handlers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHandlersDoNotAccessDBDirectly enforces the layering rule that HTTP handlers
// must delegate persistence operations through app.State (or app/service interfaces)
// instead of touching state.DB directly.
func TestHandlersDoNotAccessDBDirectly(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read handlers dir: %v", err)
	}

	fset := token.NewFileSet()
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "DB" {
				return true
			}
			pos := fset.Position(sel.Pos())
			violations = append(violations, pos.String())
			return true
		})
	}
	if len(violations) > 0 {
		t.Fatalf("handlers must not access .DB directly (delegate via app.State): %s", strings.Join(violations, ", "))
	}
}
