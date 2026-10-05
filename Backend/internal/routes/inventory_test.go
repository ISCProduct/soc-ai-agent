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
	re := regexp.MustCompile(`(?m)^\s*[a-zA-Z][a-zA-Z0-9]*\.(GET|POST|PUT|PATCH|DELETE|Any)\(`)
	found := len(re.FindAllString(string(raw), -1))

	if found != len(mainGoRoutes) {
		t.Errorf("main.go の直接登録が %d 本、mainGoRoutes は %d 件。"+
			"main.go に追加したら inventory.go の mainGoRoutes にも追記する",
			found, len(mainGoRoutes))
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
