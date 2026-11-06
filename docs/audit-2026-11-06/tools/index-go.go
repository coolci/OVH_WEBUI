// Audit-only AST inventory. Run from the repository root.
package main

import (
 "encoding/json"
 "go/ast"
 "go/parser"
 "go/token"
 "os"
 "os/exec"
 "strings"
)

type Entry struct { File string `json:"file"`; Name string `json:"name"`; Line int `json:"line"`; End int `json:"end"`; Kind string `json:"kind"` }
func main() {
 raw, err := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "*.go").Output(); if err != nil { panic(err) }
 entries := []Entry{}
 for _, file := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
  if strings.HasPrefix(file,"docs/audit-2026-11-06/") {continue}; fs := token.NewFileSet(); tree, err := parser.ParseFile(fs, file, nil, parser.ParseComments); if err != nil { panic(err) }
  for _, decl := range tree.Decls { if fn, ok := decl.(*ast.FuncDecl); ok {
   name := fn.Name.Name; if fn.Recv != nil { name = "(" + exprName(fn.Recv.List[0].Type) + ")." + name }
   kind := "function"; if strings.HasSuffix(file,"_test.go") {kind="test/helper"}
   entries = append(entries, Entry{file,name,fs.Position(fn.Pos()).Line,fs.Position(fn.End()).Line,kind})
  } }
 }
 enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); if err := enc.Encode(entries); err != nil {panic(err)}
}
func exprName(e ast.Expr) string { switch n:=e.(type) {case *ast.Ident:return n.Name; case *ast.StarExpr:return "*"+exprName(n.X); case *ast.IndexExpr:return exprName(n.X); default:return "receiver"} }
