// Package logpolicy は、ログに個人情報を出す書き方を CI で止める検査だけを持つ。
package logpolicy

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// sensitiveWords は、ログの引数の名前（変数・フィールド・関数）の**最後の単語**がこれなら、個人情報かもしれないとみなす語。小文字で比べる。
// 名前を単語に分けて最後で見るのは、PromptTokens（件数）や repoSummaryPromptVersion（版）のような誤検出を避けるため。
// reason は、検査の対象にしない。コード（理由の種別）と、学生の自由記述の両方に使われ、名前だけでは区別できないため。
// 学生の自由記述を持つ変数は answer / text などの名前で運ぶこと。
var sensitiveWords = map[string]bool{
	"email": true, "mail": true, "phone": true, "address": true, "utterance": true, "transcript": true,
	"prompt": true, "answer": true, "freetext": true, "to": true,
	"message": true, "text": true, "content": true, "body": true, "input": true,
}

// safeWrappers は、引数をこの名前の関数で包んでいれば、個人情報が残らないとみなす関数。
var safeWrappers = map[string]bool{
	"maskemail": true, "maskemails": true, "chars": true, "len": true, "cap": true, "statustext": true, "guestpromotionloglabel": true, "usagelabel": true,
}

// allowMarker は、検査の対象外にする行の目印。**コメントに**、理由つきで書く（例: // logpolicy:ok 長さだけを出している）。
// 理由が無い目印・文字列の中の目印は、許可として数えない。
var allowMarker = regexp.MustCompile(`logpolicy:ok\s+\S`)

var (
	stdLogMethods = func(n string) bool {
		return strings.HasPrefix(n, "Print") || strings.HasPrefix(n, "Fatal") || strings.HasPrefix(n, "Panic")
	}
	slogFuncs       = map[string]bool{"Info": true, "Warn": true, "Error": true, "Debug": true, "InfoContext": true, "WarnContext": true, "ErrorContext": true, "DebugContext": true, "Log": true, "LogAttrs": true}
	loggerMethodSet = func(n string) bool { return stdLogMethods(n) || slogFuncs[n] }
)

// fileInfo は、1ファイルの import の名前（別名を含む）。
type fileInfo struct {
	logNames, slogNames map[string]bool // log / log/slog の、このファイルでの名前
	importNames         map[string]bool // すべての import の名前（パッケージ名の識別子を、値と取り違えないため）
}

func importsOf(f *ast.File) fileInfo {
	fi := fileInfo{logNames: map[string]bool{}, slogNames: map[string]bool{}, importNames: map[string]bool{}}
	for _, im := range f.Imports {
		p := strings.Trim(im.Path.Value, `"`)
		name := path.Base(p)
		if im.Name != nil {
			name = im.Name.Name
		}
		fi.importNames[name] = true
		switch p {
		case "log":
			fi.logNames[name] = true
		case "log/slog":
			fi.slogNames[name] = true
		}
	}
	return fi
}

func lastName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

// isLogCall は、ログを出す呼び出しか。次を見る。
//   - log.Printf など（import の別名も）と slog.Info など
//   - log.Default().Printf / slog.Default().Info
//   - 名前が log / logger で終わる値のメソッド（s.logger.Info、p.log.Printf など）
func isLogCall(call *ast.CallExpr, fi fileInfo) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch x := sel.X.(type) {
	case *ast.Ident:
		if fi.logNames[x.Name] {
			return stdLogMethods(sel.Sel.Name)
		}
		if fi.slogNames[x.Name] {
			return slogFuncs[sel.Sel.Name]
		}
	case *ast.CallExpr:
		if inner, ok := x.Fun.(*ast.SelectorExpr); ok && inner.Sel.Name == "Default" {
			if pkg, ok := inner.X.(*ast.Ident); ok && (fi.logNames[pkg.Name] || fi.slogNames[pkg.Name]) {
				return loggerMethodSet(sel.Sel.Name)
			}
		}
	}
	n := strings.ToLower(lastName(sel.X))
	return loggerMethodSet(sel.Sel.Name) && (strings.HasSuffix(n, "log") || strings.HasSuffix(n, "logger"))
}

// sensitiveName は、引数の式の中にある、個人情報らしい名前を返す。安全な関数で包んだ部分と、パッケージ名は見ない。
func sensitiveName(e ast.Expr, fi fileInfo) string {
	found := ""
	ast.Inspect(e, func(n ast.Node) bool {
		if found != "" {
			return false
		}
		switch x := n.(type) {
		case *ast.CallExpr:
			if safeWrappers[strings.ToLower(lastName(x.Fun))] {
				return false
			}
		case *ast.Ident:
			if !fi.importNames[x.Name] {
				found = matchWord(x.Name)
			}
		case *ast.SelectorExpr:
			found = matchWord(x.Sel.Name)
		case *ast.BasicLit:
			return false
		}
		return found == ""
	})
	return found
}

// lastWord は、camelCase / snake_case の名前の最後の単語を小文字で返す（「trimmedAnswer」→「answer」）。
func lastWord(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	var words []string
	cur := []rune{}
	runes := []rune(name)
	for i, r := range runes {
		switch {
		case r == ' ':
			if len(cur) > 0 {
				words = append(words, string(cur))
				cur = cur[:0]
			}
		case unicode.IsUpper(r) && len(cur) > 0 && (unicode.IsLower(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))):
			words = append(words, string(cur))
			cur = []rune{r}
		default:
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		words = append(words, string(cur))
	}
	if len(words) == 0 {
		return ""
	}
	return strings.ToLower(words[len(words)-1])
}

func matchWord(name string) string {
	// IsFreeText / hasBody のような真偽値の名前は、値そのものを持たない
	if l := strings.ToLower(name); (strings.HasPrefix(l, "is") || strings.HasPrefix(l, "has")) && len(name) > 2 && unicode.IsUpper(rune(name[2])) || strings.HasPrefix(name, "has") && len(name) > 3 && unicode.IsUpper(rune(name[3])) {
		return ""
	}
	if sensitiveWords[lastWord(name)] {
		return name
	}
	return ""
}

type finding struct{ pos, name, line string }

func scan(t *testing.T, root string) []finding {
	t.Helper()
	var out []finding
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		lines := strings.Split(string(src), "\n")
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, src, parser.ParseComments)
		if err != nil {
			return err
		}
		// 許可の目印は、コメントの中だけを見る（文字列の中の「logpolicy:ok」は数えない）
		allowed := map[int]bool{}
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if allowMarker.MatchString(c.Text) {
					allowed[fset.Position(c.Pos()).Line] = true
				}
			}
		}
		fi := importsOf(f)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isLogCall(call, fi) {
				return true
			}
			start, end := fset.Position(call.Pos()).Line, fset.Position(call.End()).Line
			for l := start; l <= end; l++ {
				if allowed[l] {
					return true
				}
			}
			for _, a := range call.Args {
				if name := sensitiveName(a, fi); name != "" {
					out = append(out, finding{pos: fmt.Sprintf("%s:%d", p, start), name: name, line: strings.TrimSpace(lines[start-1])})
					break
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].pos < out[j].pos })
	return out
}

// ログの引数に、個人情報らしい名前の値を渡していないこと。
// 出してよい理由があるときだけ、その行に `// logpolicy:ok 理由` を書く。
func TestNoSensitiveValuesInLogs(t *testing.T) {
	// Backend の internal/ と cmd/ の全体（このファイルは Backend/internal/logpolicy にある）
	for _, root := range []string{"..", "../../cmd"} {
		for _, f := range scan(t, root) {
			t.Errorf("%s: ログに %q を出している: %s", f.pos, f.name, f.line)
		}
	}
}

// 検査そのものが働くこと。検査が何も見つけない状態で通り続けるのを防ぐ。
func TestScanner(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package x\n"+body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("bad.go", `import (
	"log"
	stdlog "log"
	"log/slog"
)

type svc struct{ logger *slog.Logger }

func f(userMessage string, req struct{ Email string }, s svc) {
	log.Printf("answer=%s", userMessage)                // 1: 単行
	log.Printf("multi",                                  // 2: 複数行
		req.Email)
	stdlog.Println(userMessage)                          // 3: import の別名
	log.Default().Printf("%s", userMessage)              // 4: log.Default()
	s.logger.Info("m", "a", userMessage)                 // 5: logger のメソッド
	log.Printf("logpolicy:ok %s", userMessage)           // 6: 文字列の中の目印は許可にならない
	log.Println(userMessage)                             // logpolicy:ok
	log.Println(userMessage)                             // logpolicy:ok 長さだけを出している想定
}
`)
	write("good.go", `import (
	"log"
	"net/http"
)

func g(promptTokens int, isFreeText bool, userMessage string) {
	log.Printf("tokens=%d free=%v chars=%d", promptTokens, isFreeText, chars(userMessage))
	log.Printf("status=%s", http.StatusText(404))
	log.Printf("n=%d", cap([]byte(userMessage)))
}
func chars(string) int { return 0 }
`)
	got := scan(t, dir)
	// 6 件。目印に理由が無い行（log.Println の1つ目）も違反になる
	if len(got) != 7 {
		t.Fatalf("違反は7件のはず（1〜6 と、理由の無い目印）。got=%d: %v", len(got), got)
	}
	for _, f := range got {
		if !strings.HasSuffix(strings.Split(f.pos, ":")[0], "bad.go") {
			t.Errorf("good.go の行を違反にした: %v", f)
		}
	}
}

func TestLastWord(t *testing.T) {
	for in, want := range map[string]string{
		"trimmedAnswer": "answer", "PromptTokens": "tokens", "repoSummaryPromptVersion": "version",
		"user_message": "message", "Email": "email", "HTMLBody": "body", "msg": "msg",
	} {
		if got := lastWord(in); got != want {
			t.Errorf("lastWord(%q)=%q, want %q", in, got, want)
		}
	}
}
