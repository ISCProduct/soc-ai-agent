package repositories_test

// #1203 無認証の公開APIが審査前のゲスト投稿企業を返さないことを検証する。
//
// /company-entry は無認証で投稿でき（honeypot とレート制限のみ）、
// 作られる企業は draft で始まる。審査前にそのまま公開APIへ出ると、
// 任意の内容の企業を作って即座に学生へ露出させられる。
//
// 企業一覧だけ塞いでも、relations / market-info は Preload で
// models.Company を丸ごと返すため隣から読めてしまう。
// また /companies/web-search と /companies/validate、面接・履歴書の企業ブリーフは
// 管理画面用の CompanyRepository を経由するので別に塞ぐ必要がある。
// 公開境界の全経路を1つのテストで押さえる。
//
// 「company_entry_submissions という文字列が SQL に出る」だけの検査では、
// 結合キーの取り違え・条件の反転・端点の取りこぼしを検出できない
// （実際にそれらの変異がテストをすり抜けた）。ここでは生成SQL全体を突き合わせる。

import (
	"strings"
	"testing"

	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// capturingMatcher は発行されたSQLを記録し、常にマッチする。
type capturingMatcher struct{ got *[]string }

func (m capturingMatcher) Match(expectedSQL, actualSQL string) error {
	*m.got = append(*m.got, normalizeSQL(actualSQL))
	return nil
}

// normalizeSQL は改行・連続空白を1つの空白に潰し、golden 比較を安定させる。
func normalizeSQL(s string) string { return strings.Join(strings.Fields(s), " ") }

func newCapturingDB(t *testing.T) (*gorm.DB, *[]string) {
	t.Helper()
	got := &[]string{}
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(capturingMatcher{got: got}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	// 発行されるクエリ数は経路によって異なるので、多めに用意して空行を返す。
	for range 8 {
		mock.ExpectQuery("").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return db, got
}

// guard は #1203 のガード条件。cols は企業IDを持つ列。
//
// この文字列そのものが検査対象。
//   - JOIN のキーが c.id = s.company_id であること（取り違えるとガードが無効化する）
//   - 条件が NOT EXISTS であること（反転すると未審査企業「だけ」を返す）
//   - 却下済み(is_active=false)も除くこと
//   - relations では端点4本すべてを見ること（1本でも漏れると素通りする）
func guard(cols ...string) string {
	return normalizeSQL(`NOT EXISTS ( SELECT 1 FROM company_entry_submissions s
		JOIN companies c ON c.id = s.company_id
		WHERE (c.data_status <> 'published' OR c.is_active = false)
		AND s.company_id IN (` + strings.Join(cols, ", ") + `) )`)
}

func TestPublicCompanyEndpoints_ExcludeUnreviewedGuestEntries(t *testing.T) {
	relationGuard := guard("parent_id", "child_id", "from_id", "to_id")
	companyGuard := guard("companies.id")
	marketGuard := guard("company_id")

	tests := []struct {
		name string
		call func(db *gorm.DB)
		want []string // 発行される各クエリに含まれていなければならないガード
	}{
		{name: "企業一覧", want: []string{companyGuard, companyGuard}, call: func(db *gorm.DB) {
			_, _, _ = repositories.NewCompanyQueryRepository(db).GetCompaniesFiltered(10, 0, "", "", "")
		}},
		{name: "企業詳細", want: []string{companyGuard}, call: func(db *gorm.DB) {
			_, _ = repositories.NewCompanyQueryRepository(db).GetCompanyByID(1)
		}},
		{name: "企業ごとの相関関係", want: []string{relationGuard}, call: func(db *gorm.DB) {
			_, _ = repositories.NewCompanyQueryRepository(db).GetByCompanyID(1)
		}},
		{name: "全相関関係", want: []string{relationGuard}, call: func(db *gorm.DB) {
			_, _ = repositories.NewCompanyQueryRepository(db).GetAll()
		}},
		{name: "企業ごとの市場情報", want: []string{marketGuard}, call: func(db *gorm.DB) {
			_, _ = repositories.NewCompanyQueryRepository(db).GetMarketInfoByCompanyID(1)
		}},
		{name: "全市場情報", want: []string{marketGuard}, call: func(db *gorm.DB) {
			_, _ = repositories.NewCompanyQueryRepository(db).GetAllMarketInfo()
		}},
		{name: "企業の求人一覧", want: []string{marketGuard}, call: func(db *gorm.DB) {
			_, _ = repositories.NewCompanyQueryRepository(db).GetJobPositionsByCompany(1)
		}},
		// #1203 レビュー指摘: 以下3つは CompanyRepository を直接使っており素通りしていた。
		{name: "企業名サジェスト(web-search)", want: []string{companyGuard}, call: func(db *gorm.DB) {
			_, _ = repositories.NewCompanyPublicRepository(db).FindAllActiveNames("あ")
		}},
		{name: "企業実在確認(validate)", want: []string{companyGuard}, call: func(db *gorm.DB) {
			_, _ = repositories.NewCompanyPublicRepository(db).FindByName("あ")
		}},
		{name: "面接・履歴書の企業ブリーフ", want: []string{companyGuard}, call: func(db *gorm.DB) {
			_, _ = repositories.NewCompanyPublicRepository(db).FindByID(1)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name+"がゲスト投稿の未審査企業を除く", func(t *testing.T) {
			db, got := newCapturingDB(t)
			tt.call(db)

			require.Len(t, *got, len(tt.want), "発行クエリ数が想定と異なる: %v", *got)
			for i, want := range tt.want {
				require.Contains(t, (*got)[i], want,
					"審査前のゲスト投稿を除く条件がSQLに無い、または条件が書き換わっている")
			}
		})
	}
}

// ガード条件が data_status='published' を例外にしていること。
// 管理者が公開した企業は、ゲスト投稿由来でも表示されなければならない。
func TestGuestEntryGuard_AllowsPublished(t *testing.T) {
	db, got := newCapturingDB(t)
	_, _ = repositories.NewCompanyQueryRepository(db).GetCompanyByID(1)

	require.Len(t, *got, 1)
	require.Contains(t, (*got)[0], "c.data_status <> 'published'",
		"published を例外にする条件が無い。管理者が公開しても表示されなくなる")
}

// ガードの走査対象が company_entry_submissions 側であること（S-3）。
// companies を端点ごとにフルスキャンする形に戻ると、無認証の
// /api/companies/relations が企業数に比例して重くなる。
func TestGuestEntryGuard_ScansSubmissionsOnce(t *testing.T) {
	db, got := newCapturingDB(t)
	_, _ = repositories.NewCompanyQueryRepository(db).GetAll()

	require.Len(t, *got, 1)
	require.Equal(t, 1, strings.Count((*got)[0], "company_entry_submissions"),
		"端点ごとにサブクエリが分かれている。1本の NOT EXISTS にまとめること")
}
