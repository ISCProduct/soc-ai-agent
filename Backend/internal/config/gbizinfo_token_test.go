package config_test

// gBizINFO のアクセストークンを解決する経路のテスト。
// 実行: cd Backend && go test ./test/config/ -v
//
// 本番・staging のシークレットに入っているのは GBIZINFO_API_KEY だけで、
// GBIZINFO_API_TOKEN は設定されていない。にもかかわらず main.go が
// os.Getenv("GBIZINFO_API_TOKEN") を直接読んでいたため、企業グラフの
// gBizINFO 取得だけが空文字のトークンで 401 になっていた(#1358)。
//
// 見たいのは「名前の解決が config に一本化されていること」。
// 値そのものより、解決経路が1箇所であることが再発防止になる。

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Backend/internal/config"
)

// LoadConfig の必須値チェックに引っかからないよう最低限を揃える。
func setDBEnv(t *testing.T) {
	t.Helper()
	// OAUTH_STATE_SECRET が無いと LoadConfig が起動を中止する。
	for k, v := range map[string]string{
		"DB_HOST": "localhost", "DB_PORT": "3306",
		"DB_USER": "u", "DB_PASSWORD": "p", "DB_NAME": "d",
		"OAUTH_STATE_SECRET": "test-oauth-state-secret",
	} {
		t.Setenv(k, v)
	}
}

func TestGBizInfoToken_名前の解決(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		token string
		want  string
	}{
		// 本番・staging が実際に使っている形。これが壊れると 401 に戻る。
		{"API_KEYだけ設定されている（本番・stagingの形）", "key-value", "", "key-value"},
		// 古い名前しか無い環境（ローカルの .env 等）も拾う。
		{"API_TOKENだけ設定されている", "", "token-value", "token-value"},
		// 両方あるときは KEY を優先する。混在時に挙動がぶれないこと。
		{"両方あるならAPI_KEYを優先する", "key-value", "token-value", "key-value"},
		{"どちらも無ければ空", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDBEnv(t)
			t.Setenv("GBIZINFO_API_KEY", tt.key)
			t.Setenv("GBIZINFO_API_TOKEN", tt.token)

			cfg, err := config.LoadConfig()
			if err != nil {
				t.Fatalf("設定を読めない: %v", err)
			}
			if cfg.GBizInfoToken != tt.want {
				t.Errorf("トークンが違う: got %q, want %q", cfg.GBizInfoToken, tt.want)
			}
		})
	}
}

// 環境変数名の解決を config の外でやり直さないことを固定する。
//
// 直読みが1箇所でも復活すると、そこだけフォールバックが効かず、
// 「gBizINFO 本体は動くのに特定の経路だけ 401」という気づきにくい形になる。
// 実際にそうなっていた(#1358)。
func TestGBizInfo環境変数をconfig以外で直読みしない(t *testing.T) {
	root := "../.." // Backend/

	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if name := info.Name(); name == "vendor" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// config パッケージが解決の責任を持つ。ここだけは直読みしてよい。
		if strings.Contains(filepath.ToSlash(path), "/internal/config/") {
			return nil
		}

		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // 解析できないファイルは対象外
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Getenv" {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "os" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if strings.Contains(lit.Value, "GBIZINFO_") {
				offenders = append(offenders, fmt.Sprintf("%s:%d %s",
					filepath.ToSlash(path), fset.Position(lit.Pos()).Line, lit.Value))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("走査に失敗: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("config の外で GBIZINFO_* を直読みしている。config.GBizInfoToken を使うこと:\n%s",
			strings.Join(offenders, "\n"))
	}
}
