package training_test

// 学習データAPIの認証の切り分けを、ソース上で固定する。
//
// stats は件数だけを返すのでサービス間認証(X-Admin-Secret)で足りるが、
// export は候補者の発話と選考結果を返すため管理者本人の認証が要る。
// 片方を緩めたときに、もう片方まで一緒に緩むのを防ぐ。

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestTrainingAPI_認証の切り分け(t *testing.T) {
	src, err := os.ReadFile("../../../cmd/server/main.go")
	if err != nil {
		t.Skipf("main.go を読めない: %v", err)
	}
	text := string(src)

	statsLine := findLine(text, "/admin/training/stats")
	exportLine := findLine(text, "/training/export")

	if statsLine == "" {
		t.Fatal("stats のルート登録が見つからない")
	}
	if exportLine == "" {
		t.Fatal("export のルート登録が見つからない")
	}

	// stats: 件数のみ。サービス間認証で叩ける。
	if !strings.Contains(statsLine, "EchoStaticSecretAuth") {
		t.Errorf("stats はサービス間認証であるべき: %s", strings.TrimSpace(statsLine))
	}

	// export: 個人情報を含む。管理者本人の認証(adminEntry グループ)から外さない。
	if !strings.Contains(exportLine, "adminEntry") {
		t.Errorf("export は管理者認証の配下に置くこと: %s", strings.TrimSpace(exportLine))
	}
	if strings.Contains(exportLine, "EchoStaticSecretAuth") {
		t.Errorf("export を共有シークレット認証にしてはいけない: %s", strings.TrimSpace(exportLine))
	}
}

func findLine(text, needle string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, needle) && !strings.HasPrefix(strings.TrimSpace(line), "//") {
			if regexp.MustCompile(`\.(GET|POST)\(`).MatchString(line) {
				return line
			}
		}
	}
	return ""
}
