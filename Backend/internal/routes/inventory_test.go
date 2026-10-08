package routes

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// -update を付けて走らせると api/routes.txt を実装に合わせて書き直す。
//
//	go test ./internal/routes/ -run TestRouteCatalog -update
var updateCatalog = flag.Bool("update", false, "api/routes.txt を実装に合わせて更新する")

const (
	catalogPath = "../../api/routes.txt"
	openapiPath = "../../api/openapi.yaml"
	mainGoPath  = "../../cmd/server/main.go"
)

func formatCatalog(rs []Route) string {
	var b strings.Builder
	b.WriteString("# 実装に登録されている全エンドポイント。\n")
	b.WriteString("# 手で編集しない。次のコマンドで生成する:\n")
	b.WriteString("#   cd Backend && go test ./internal/routes/ -run TestRouteCatalog -update\n")
	fmt.Fprintf(&b, "# 合計 %d 本\n", len(rs))
	for _, r := range rs {
		fmt.Fprintf(&b, "%s %s\n", r.Method, r.Path)
	}
	return b.String()
}

// TestRouteCatalog api/routes.txt が実装と一致していることを確認する。
// エンドポイントを追加・削除・改名したらここで落ちる。
func TestRouteCatalog(t *testing.T) {
	want := formatCatalog(Inventory())

	if *updateCatalog {
		if err := os.WriteFile(catalogPath, []byte(want), 0o644); err != nil {
			t.Fatalf("書き込みに失敗: %v", err)
		}
		t.Logf("%s を更新した", catalogPath)
		return
	}

	got, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("%s が読めない: %v", catalogPath, err)
	}
	if string(got) == want {
		return
	}

	gotSet := parseCatalog(string(got))
	wantSet := parseCatalog(want)
	for _, r := range wantSet {
		if !slices.Contains(gotSet, r) {
			t.Errorf("カタログに無い（実装にはある）: %s", r)
		}
	}
	for _, r := range gotSet {
		if !slices.Contains(wantSet, r) {
			t.Errorf("カタログにあるが実装に無い: %s", r)
		}
	}
	t.Log("`go test ./internal/routes/ -run TestRouteCatalog -update` で更新できる")
}

func parseCatalog(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// TestOpenAPIPathsExistInImplementation openapi.yaml と実装が双方向で
// 一致していることを確認する。
//
//   - spec にあるが実装に無い  → 落ちる（嘘のドキュメントを防ぐ）
//   - 実装にあるが spec に無い  → 落ちる（書き忘れを防ぐ）
//
// 全エンドポイントを書き切った時点で双方向にした。片方向だと、新しい
// エンドポイントを足して spec に書き忘れても気づけない。
func TestOpenAPIPathsExistInImplementation(t *testing.T) {
	raw, err := os.ReadFile(openapiPath)
	if err != nil {
		t.Fatalf("%s が読めない: %v", openapiPath, err)
	}

	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("openapi.yaml を解釈できない: %v", err)
	}

	impl := map[string]bool{}
	for _, r := range Inventory() {
		impl[r.Method+" "+r.Path] = true
	}

	specced := 0
	inSpec := map[string]bool{}
	for specPath, ops := range doc.Paths {
		// OpenAPI は {id}、echo は :id なので寄せる。
		implPath := regexp.MustCompile(`\{([^}]+)\}`).ReplaceAllString(specPath, ":$1")
		for method := range ops {
			switch strings.ToLower(method) {
			case "get", "post", "put", "patch", "delete":
			default:
				continue // parameters, summary など
			}
			key := strings.ToUpper(method) + " " + implPath
			if !impl[key] {
				t.Errorf("openapi.yaml にあるが実装に無い: %s %s", strings.ToUpper(method), specPath)
				continue
			}
			inSpec[key] = true
			specced++
		}
	}

	// 実装にあるが spec に無いものを洗い出す。
	for key := range impl {
		if !inSpec[key] {
			t.Errorf("実装にあるが openapi.yaml に無い: %s", key)
		}
	}

	total := len(impl)
	t.Logf("spec 網羅率: %d / %d 本 (%.1f%%)", specced, total, float64(specced)*100/float64(total))
}

// TestMainGoDirectRoutesAreListed main.go で直接登録しているルートが
// inventory.go の mainGoRoutes に漏れなく書かれていることを確認する。
//
// Inventory() は routes パッケージ内の登録しか見られないため、main.go 側に
// 1 本足して mainGoRoutes に書き忘れると、spec からも静かに漏れる。
// 本数だけでも固定しておけば、増えたときにここで気づける。
func TestMainGoDirectRoutesAreListed(t *testing.T) {
	raw, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("%s が読めない: %v", mainGoPath, err)
	}
	// 変数名・メソッド・パスを取る。変数名はグループの prefix 解決に使う。
	re := regexp.MustCompile(`(?m)^\s*([a-zA-Z][a-zA-Z0-9]*)\.(GET|POST|PUT|PATCH|DELETE|Any)\("([^"]*)"`)
	matches := re.FindAllStringSubmatch(string(raw), -1)

	// main.go のグループ変数と prefix。`api := e.Group("/api")` 等を直接読むのは
	// 過剰なので、使われている変数だけを対応表に持つ。増えたら下の default で落ちる。
	prefix := map[string]string{
		"e":          "",     // ルート直下（/health, /metrics）
		"api":        "/api", // api := e.Group("/api")
		"adminEntry": "/api/admin",
	}

	var found []Route
	for _, m := range matches {
		varName, method, path := m[1], m[2], m[3]
		p, ok := prefix[varName]
		if !ok {
			t.Errorf("main.go に未知のグループ変数 %q がある（%s %q）。"+
				"prefix の対応表に追加すること", varName, method, path)
			continue
		}
		found = append(found, Route{Method: method, Path: p + path})
	}

	// 件数だけでなく (メソッド, パス) の組で照合する。件数比較だと、改名や
	// GET→POST の変更で本数が変わらない場合に mainGoRoutes と routes.txt が
	// 古い経路を持ち続けてしまう。
	want := map[Route]bool{}
	for _, r := range mainGoRoutes {
		want[r] = true
	}
	got := map[Route]bool{}
	for _, r := range found {
		got[r] = true
	}
	for r := range got {
		if !want[r] {
			t.Errorf("main.go にあるが mainGoRoutes に無い: %s %s", r.Method, r.Path)
		}
	}
	for r := range want {
		if !got[r] {
			t.Errorf("mainGoRoutes にあるが main.go に無い: %s %s", r.Method, r.Path)
		}
	}
}

// TestAllRouteRegistrationsAreCovered internal/routes/*.go の登録行数と
// Inventory() の件数が一致することを確認する。
//
// company_auth_routes.go はコントローラを `!= nil` で囲んでおり、Inventory() が
// nil を渡していた間は そのブロックの 15 本が静かに登録されず、カタログからも
// openapi.yaml からも漏れていた。件数が合わないと気づけない類の漏れなので、
// ソース上の登録行数と突き合わせて固定する。
//
// `Any` は 1 行で 11 メソッドに展開されるため、使われていたらこの比較は成立しない。
// 現在は使っていないので、使われたらその旨で落とす。
//
// なお mainGoRoutes の書き漏れはこの比較では検出できない(両辺に同じ値が出るため)。
// そちらは TestMainGoDirectRoutesAreListed が main.go の行数と突き合わせている。
func TestAllRouteRegistrationsAreCovered(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ディレクトリが読めない: %v", err)
	}
	reg := regexp.MustCompile(`(?m)^\s*[a-zA-Z][a-zA-Z0-9]*\.(GET|POST|PUT|PATCH|DELETE)\(`)
	anyReg := regexp.MustCompile(`(?m)^\s*[a-zA-Z][a-zA-Z0-9]*\.Any\(`)
	lines, anyLines := 0, 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("%s が読めない: %v", name, err)
		}
		lines += len(reg.FindAllString(string(raw), -1))
		anyLines += len(anyReg.FindAllString(string(raw), -1))
	}
	if anyLines > 0 {
		t.Fatalf("Any 登録が %d 件ある。1 行で 11 メソッドに展開されるため件数比較が成立しない。"+
			"メソッドを絞るか、このテストを展開数込みに直すこと", anyLines)
	}

	want := lines + len(mainGoRoutes)
	got := len(Inventory())
	if got != want {
		t.Errorf("登録行 %d + mainGoRoutes %d = %d だが Inventory() は %d 本。"+
			"`!= nil` で囲まれたブロックに nil を渡していて登録が飛んでいないか確認する",
			lines, len(mainGoRoutes), want, got)
	}
}
