package safego_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 素の goroutine を増やさないための検査（#1446）。
//
// goroutine 内の panic は誰も回復しないとプロセス全体を落とす。
// Echo の Recover が守るのはリクエストを処理している goroutine だけなので、
// `go` で切り離した処理が1つ落ちると Backend が丸ごと死に、全ユーザーが 502 になる。
//
// 過去に2度取りこぼしている。#1405 が12箇所を置換したが cmd/server/main.go が
// 対象から漏れ（#1446）、その main.go を直したあとも9箇所が残っていた。
// 人手の走査では漏れるので、増えたら落ちるようにする。
//
// 結果を返す goroutine は対象外。panic を握り潰すと受け側がチャネル待ちで
// 止まるため、その場で defer recover してエラーを送り返すのが正しい。
// 除外するときは、なぜ safego を使わないのかをコードのコメントに書くこと。
var allowed = map[string]string{
	// safego 自身の実装。
	"internal/safego/safego.go": "safego の実装そのもの",
	// 結果を done チャネルへ返すため、自前で recover してエラーを送り返している。
	"internal/services/company/website_extract.go": "結果を返す goroutine（自前 recover 済み）",
}

func TestNoBareGoroutine(t *testing.T) {
	// このテストは Backend/internal/safego にあるので、2つ上がモジュールルート。
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	bare := regexp.MustCompile(`(?m)^[\t ]*go[\t ]+[a-zA-Z_(]`)

	var found []string
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if _, ok := allowed[rel]; ok {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(src), "\n") {
				if bare.MatchString(line) {
					found = append(found, rel+":"+itoa(i+1)+" "+strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(found) > 0 {
		t.Errorf(
			"素の goroutine が %d 箇所あります。safego.Go / safego.Every を使ってください。\n"+
				"結果を返す goroutine なら、その場で defer recover してエラーを送り返したうえで\n"+
				"このテストの allowed に理由つきで足してください。\n  %s",
			len(found), strings.Join(found, "\n  "),
		)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
