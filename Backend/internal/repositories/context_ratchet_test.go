package repositories

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

// context を受け取らないリポジトリメソッドを増やさない（SOCAIAGENT-516）。
//
// 既存分は repository_context_allowlist.txt に残している。
// 新しくメソッドを足すときは context.Context を受け、クエリは WithContext に渡す。
// 既存メソッドへ context を足したら、許可リストからその行を消す。
func TestRepositoryMethodsTakeContext(t *testing.T) {
	got := methodsMissingContext(t)
	if os.Getenv("UPDATE_REPO_CONTEXT_ALLOWLIST") == "1" {
		body := strings.Join(got, "\n")
		if body != "" {
			body += "\n"
		}
		if err := os.WriteFile("repository_context_allowlist.txt", []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d methods", len(got))
		return
	}

	allow := readAllowlist(t)
	var added []string
	for _, name := range got {
		if _, ok := allow[name]; !ok {
			added = append(added, name)
		}
	}
	var stale []string
	seen := map[string]bool{}
	for _, name := range got {
		seen[name] = true
	}
	for name := range allow {
		if !seen[name] {
			stale = append(stale, name)
		}
	}
	slices.Sort(stale)
	if len(added) > 0 || len(stale) > 0 {
		t.Fatalf("context の無いメソッドが増えた: %v\n許可リストに残っているが、もう context を受けているか消えた: %v", added, stale)
	}
}

func readAllowlist(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("repository_context_allowlist.txt")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[line] = true
	}
	return out
}

func methodsMissingContext(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name == nil {
				continue
			}
			recv := receiverTypeName(fn)
			if recv == "" {
				continue
			}
			if hasContextParam(fn) {
				continue
			}
			missing = append(missing, recv+"."+fn.Name.Name)
		}
	}
	slices.Sort(missing)
	missing = slices.Compact(missing)
	return missing
}

func receiverTypeName(fn *ast.FuncDecl) string {
	if len(fn.Recv.List) == 0 {
		return ""
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	id, ok := t.(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}

func hasContextParam(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		sel, ok := field.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkg, ok := sel.X.(*ast.Ident)
		if ok && pkg.Name == "context" && sel.Sel.Name == "Context" {
			return true
		}
	}
	return false
}
