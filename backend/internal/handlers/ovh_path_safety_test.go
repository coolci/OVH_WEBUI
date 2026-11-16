package handlers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestOVHPathBuilders_RejectInjectionAndEscapeSegments(t *testing.T) {
	t.Run("valid segments and escaping", func(t *testing.T) {
		p, err := buildOVHPath("ip", "192.0.2.9/32", "mitigation", "192.0.2.9")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "/ip/192.0.2.9%2F32/mitigation/192.0.2.9"
		if p != want {
			t.Fatalf("got %q, want %q", p, want)
		}

		p2, err := buildOVHPath("dedicated", "server", "ns1.ip-1-2-3.eu", "task", "42")
		if err != nil || p2 != "/dedicated/server/ns1.ip-1-2-3.eu/task/42" {
			t.Fatalf("got (%q, %v)", p2, err)
		}

		// Query/fragment characters inside a segment are percent-encoded so they cannot truncate OVH path
		p3, err := buildOVHPath("support", "tickets", "id?injected=1#frag")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.ContainsAny(p3, "?#") || p3 != "/support/tickets/id%3Finjected=1%23frag" {
			t.Fatalf("segment query/fragment was not safely escaped: %q", p3)
		}
	})

	t.Run("reject empty dot dotdot and control chars", func(t *testing.T) {
		badCases := [][]string{
			{},
			{"dedicated", "server", ""},
			{"dedicated", "server", "   "},
			{"dedicated", "server", "."},
			{"dedicated", "server", ".."},
			{"dedicated", "server", "%2e"},
			{"dedicated", "server", "%2e%2e"},
			{"dedicated", "server", "%2E%2E"},
			{"dedicated", "server", "ns1\r\nX-Injected: 1"},
			{"dedicated", "server", "ns1\x00evil"},
		}
		for _, segs := range badCases {
			if got, err := buildOVHPath(segs...); err == nil {
				t.Errorf("expected error for segments %v, got %q", segs, got)
			}
		}
	})

	t.Run("query encoding via url.Values", func(t *testing.T) {
		q := url.Values{}
		q.Set("ipBlock", "1.2.3.4/32?evil=1#frag")
		got, err := buildOVHPathQuery(q, "dedicated", "server", "ns1.ip-1-2-3.eu", "ips")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "/dedicated/server/ns1.ip-1-2-3.eu/ips?ipBlock=1.2.3.4%2F32%3Fevil%3D1%23frag"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestHandlersAST_NoRawOVHPathOrQueryConcatenation(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read handlers dir: %v", err)
	}

	ovhPrefixes := []string{
		"/dedicated/",
		"/vps/",
		"/ip/",
		"/me/",
		"/services/",
		"/support/",
		"/order/",
	}
	rawQueryFragments := []string{
		"?block=",
		"?ipBlock=",
		"?ip=",
		"?type=",
		"?bootType=",
		"?date.from=",
		"?hardware=",
		"?default=",
		"&type=",
		"&period=",
	}

	ovhNoCtxMethods := map[string]bool{
		"Get": true, "Post": true, "Put": true, "Delete": true,
	}
	ovhCtxMethods := map[string]bool{
		"GetWithContext": true, "PostWithContext": true, "PutWithContext": true, "DeleteWithContext": true,
		"GetUnAuthWithContext": true, "PostUnAuthWithContext": true,
	}

	fset := token.NewFileSet()
	scanned := 0

	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		fileAST, err := parser.ParseFile(fset, filepath.Clean(name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		ast.Inspect(fileAST, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				sel, ok := node.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				method := sel.Sel.Name

				// 1. Check fmt.Sprintf("/dedicated/...", ...) or raw query string formatting
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fmt" && method == "Sprintf" && len(node.Args) > 0 {
					if lit, ok := stringLiteral(node.Args[0]); ok {
						for _, pfx := range ovhPrefixes {
							if strings.HasPrefix(lit, pfx) {
								pos := fset.Position(node.Pos())
								t.Errorf("%s:%d: forbidden fmt.Sprintf building OVH path %q (use buildOVHPath/ovhPathOrAbort)", pos.Filename, pos.Line, lit)
							}
						}
						for _, qf := range rawQueryFragments {
							if strings.Contains(lit, qf) {
								pos := fset.Position(node.Pos())
								t.Errorf("%s:%d: forbidden fmt.Sprintf building raw query %q (use buildOVHPathQuery/url.Values)", pos.Filename, pos.Line, lit)
							}
						}
					}
				}

				// 2. Check OVH SDK client calls (client.Get/Post/Put/Delete and *WithContext)
				var pathArg ast.Expr
				if ident, ok := sel.X.(*ast.Ident); ok && (ident.Name == "client" || strings.HasSuffix(ident.Name, "Client")) {
					if ovhNoCtxMethods[method] && len(node.Args) >= 1 {
						pathArg = node.Args[0]
					} else if ovhCtxMethods[method] && len(node.Args) >= 2 {
						pathArg = node.Args[1]
					}
				}
				if pathArg != nil {
					checkOVHClientPathArg(t, fset, method, pathArg)
				}

			case *ast.BinaryExpr:
				if node.Op != token.ADD {
					return true
				}
				// Skip ovh_security.go internal path/query joiner
				if name == "ovh_security.go" {
					return true
				}
				for _, side := range []ast.Expr{node.X, node.Y} {
					lit, ok := stringLiteral(side)
					if !ok {
						continue
					}
					for _, pfx := range ovhPrefixes {
						if strings.HasPrefix(lit, pfx) {
							pos := fset.Position(node.Pos())
							t.Errorf("%s:%d: forbidden '+' string concatenation on OVH path literal %q (use buildOVHPath/ovhPathOrAbort)", pos.Filename, pos.Line, lit)
						}
					}
					for _, qf := range rawQueryFragments {
						if strings.Contains(lit, qf) {
							pos := fset.Position(node.Pos())
							t.Errorf("%s:%d: forbidden '+' string concatenation with raw query fragment %q (use buildOVHPathQuery/url.Values)", pos.Filename, pos.Line, lit)
						}
					}
				}
			}
			return true
		})
	}

	if scanned == 0 {
		t.Fatal("expected to scan handler source files, got 0")
	}
}

func checkOVHClientPathArg(t *testing.T, fset *token.FileSet, method string, expr ast.Expr) {
	t.Helper()
	pos := fset.Position(expr.Pos())
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			t.Errorf("%s:%d: OVH client.%s must not use '+' string concatenation in path argument", pos.Filename, pos.Line, method)
		}
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fmt" && sel.Sel.Name == "Sprintf" {
				t.Errorf("%s:%d: OVH client.%s must not use fmt.Sprintf in path argument", pos.Filename, pos.Line, method)
			}
		}
	case *ast.BasicLit:
		if lit, ok := stringLiteral(e); ok {
			if strings.ContainsAny(lit, "?#") || strings.Contains(lit, "..") {
				t.Errorf("%s:%d: OVH client.%s literal path %q contains raw query/fragment/traversal characters (use buildOVHPathQuery)", pos.Filename, pos.Line, method, lit)
			}
		}
	}
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	unquoted, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return unquoted, true
}
