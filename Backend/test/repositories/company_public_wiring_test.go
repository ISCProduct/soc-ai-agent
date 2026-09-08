package repositories_test

// #1203 レビュー指摘(B-1): 公開境界のガードはリポジトリ実装ではなく「どれを注入するか」で決まる。
// CompanyRepository は管理画面用でフィルタを持たないため、学生・無認証向けの
// サービスに渡した瞬間に審査前のゲスト投稿が素通りする（実際にそうなっていた）。
// 配線が戻ったことをコンパイルでは検出できないので、main.go の注入先を直接押さえる。

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMainWiresPublicCompanyRepoToStudentFacingServices(t *testing.T) {
	src, err := os.ReadFile("../../cmd/server/main.go")
	require.NoError(t, err)

	// 注入先 → 何を守るか
	sinks := map[string]string{
		`company\.NewCompanyValidationService\(([A-Za-z]+),`: "/companies/web-search と /companies/validate",
		`interviewService\.SetCompanyRepo\(([A-Za-z]+)\)`:    "面接の企業ブリーフ（企業説明が面接プロンプトに入る）",
		`resumeService\.SetCompanyRepo\(([A-Za-z]+)\)`:       "履歴書の企業サジェスト",
	}

	for pattern, protects := range sinks {
		m := regexp.MustCompile(pattern).FindSubmatch(src)
		require.NotNil(t, m, "注入箇所が見つからない: %s", pattern)
		require.Equal(t, "companyPublicRepo", string(m[1]),
			"%s に可視性ガードの無いリポジトリが注入されている（#1203）", protects)
	}
}
