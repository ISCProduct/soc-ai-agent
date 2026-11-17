package repositories

import (
	"os"
	"strings"
	"testing"
)

// Repository は Service より下の層。Service を import すると層が逆向きになり、
// Service のテストが Repository 経由で DB 依存を引き込む。共有したい値は
// domain/repository など下層に置くこと。
func TestRepositoriesDoNotImportServices(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		src, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		// 検索文字列を分けて書く。そのまま書くとこのファイル自身に一致してしまう。
		if strings.Contains(string(src), `"Backend/internal/`+`services`) {
			t.Errorf("%s が internal/services を import している（層の逆流）", e.Name())
		}
	}
}
