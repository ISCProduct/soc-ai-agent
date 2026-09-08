package repositories_test

// #1203 無認証の公開APIが審査前のゲスト投稿企業を返さないことを検証する。
//
// /company-entry は無認証で投稿でき（honeypot とレート制限のみ）、
// 作られる企業は draft で始まる。審査前にそのまま公開APIへ出ると、
// 任意の内容の企業を作って即座に学生へ露出させられる。
//
// 企業一覧だけ塞いでも、relations / market-info は Preload で
// models.Company を丸ごと返すため隣から読めてしまう。
// 公開境界の全経路を1つのテストで押さえる。

import (
	"testing"

	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newPublicQueryRepo(t *testing.T) (*repositories.CompanyQueryRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return repositories.NewCompanyQueryRepository(db), mock
}

// 審査前のゲスト投稿を除く条件。company_entry_submissions を参照していること。
const guestEntryGuard = "company_entry_submissions"

func TestPublicCompanyEndpoints_ExcludeUnreviewedGuestEntries(t *testing.T) {
	tests := []struct {
		name    string
		queries int // このメソッドが発行するクエリ数
		call    func(r *repositories.CompanyQueryRepository)
	}{
		{name: "企業一覧", queries: 2, call: func(r *repositories.CompanyQueryRepository) {
			_, _, _ = r.GetCompaniesFiltered(10, 0, "", "", "")
		}},
		{name: "企業詳細", queries: 1, call: func(r *repositories.CompanyQueryRepository) {
			_, _ = r.GetCompanyByID(1)
		}},
		{name: "企業ごとの相関関係", queries: 1, call: func(r *repositories.CompanyQueryRepository) {
			_, _ = r.GetByCompanyID(1)
		}},
		{name: "全相関関係", queries: 1, call: func(r *repositories.CompanyQueryRepository) {
			_, _ = r.GetAll()
		}},
		{name: "企業ごとの市場情報", queries: 1, call: func(r *repositories.CompanyQueryRepository) {
			_, _ = r.GetMarketInfoByCompanyID(1)
		}},
		{name: "全市場情報", queries: 1, call: func(r *repositories.CompanyQueryRepository) {
			_, _ = r.GetAllMarketInfo()
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name+"がゲスト投稿の未審査企業を除く", func(t *testing.T) {
			repo, mock := newPublicQueryRepo(t)
			// 発行される主要クエリすべてにガード条件が含まれること。
			// 含まれなければ期待にマッチせずエラーになる。
			for i := 0; i < tt.queries; i++ {
				mock.ExpectQuery(guestEntryGuard).
					WillReturnRows(sqlmock.NewRows([]string{"id"}))
			}
			tt.call(repo)
			require.NoError(t, mock.ExpectationsWereMet(),
				"審査前のゲスト投稿を除く条件がSQLに含まれていない")
		})
	}
}

// ガード条件が data_status='published' を例外にしていること。
// 管理者が公開した企業は、ゲスト投稿由来でも表示されなければならない。
func TestGuestEntryGuard_AllowsPublished(t *testing.T) {
	repo, mock := newPublicQueryRepo(t)
	mock.ExpectQuery("data_status.*published").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "公開済み企業"))

	_, _ = repo.GetCompanyByID(1)
	require.NoError(t, mock.ExpectationsWereMet(),
		"published を例外にする条件が無い。管理者が公開しても表示されなくなる")
}
