package repositories

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"Backend/internal/models"
	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SOCAIAGENT-612: 学生向けの企業検索。

func newCompanySearchMock(t *testing.T) (*CompanySearchRepository, sqlmock.Sqlmock) {
	t.Helper()
	matcher := sqlmock.QueryMatcherFunc(func(want, got string) error {
		for _, part := range strings.Split(want, "|") {
			if !strings.Contains(got, part) {
				return errors.New("SQL に " + part + " が無い: " + got)
			}
		}
		return nil
	})
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent), SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	return NewCompanySearchRepository(db), mock
}

// 学生に見せる企業だけ（有効・未削除・審査前のゲスト投稿でない）
const visibleCompanySQL = "companies.is_active = ? AND companies.deleted_at IS NULL|c.is_guest_entry = true"

func TestCompanySearchRepository_Search(t *testing.T) {
	repo, mock := newCompanySearchMock(t)
	filters := "companies.name LIKE ?|companies.industry = ?|companies.location LIKE ?|jp.data_status = 'published'|c.data_status = 'published'|(jc.id = ? OR jc.parent_id = ?)"
	// 名前の % と _ は文字として扱う。都道府県は所在地に含まれるか
	args := []driver.Value{true, "%100\\%\\_A社%", "IT", "%東京都%", uint(3), uint(3)}
	mock.ExpectQuery("SELECT count(*)|" + visibleCompanySQL + "|" + filters).WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(41))
	// 並びは企業名順（推薦や課金で上位に固定しない）
	// 出どころの列も返す（AI 補完の企業に「AI推定」を出すため）
	mock.ExpectQuery(visibleCompanySQL + "|" + filters + "|AS source_type|AS last_fetch_confidence|AS last_model_used|ORDER BY companies.name ASC, companies.id ASC|LIMIT ?|OFFSET ?").
		WithArgs(append(args, 20, 40)...).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "industry", "location", "description", "logo_url"}).
			AddRow(1, "A社", "IT", "東京都渋谷区", "", ""))
	rows, total, err := repo.Search(CompanySearchFilter{Name: "100%_A社", Industry: "IT", Prefecture: "東京都", JobCategoryID: 3, Limit: 20, Offset: 40})
	if err != nil || total != 41 || len(rows) != 1 || rows[0].Name != "A社" {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompanySearchRepository_Search_条件なし(t *testing.T) {
	repo, mock := newCompanySearchMock(t)
	mock.ExpectQuery("SELECT count(*)|" + visibleCompanySQL).WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(visibleCompanySQL).WithArgs(true, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, total, err := repo.Search(CompanySearchFilter{Limit: 20}); err != nil || total != 0 {
		t.Fatalf("total=%d err=%v", total, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompanySearchRepository_SearchOptions(t *testing.T) {
	repo, mock := newCompanySearchMock(t)
	mock.ExpectQuery(visibleCompanySQL + "|DISTINCT|companies.industry <> ''").
		WillReturnRows(sqlmock.NewRows([]string{"industry"}).AddRow("IT"))
	// 職種は、公開中の企業の公開中の求人がある職種。シードの小分類は parent_id が無いので、階層では絞らない
	mock.ExpectQuery("jc.id = top.id OR jc.parent_id = top.id|jp.data_status = 'published'|companies.is_active = true|companies.data_status = 'published'|c.is_guest_entry = true").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "エンジニア"))
	industries, cats, err := repo.SearchOptions()
	if err != nil || len(industries) != 1 || len(cats) != 1 || cats[0].Name != "エンジニア" {
		t.Fatalf("industries=%v cats=%v err=%v", industries, cats, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompanySearchRepository_FavoriteStates(t *testing.T) {
	repo, mock := newCompanySearchMock(t)
	// 最新の診断（面接スナップショット・適性検査を除く）の推薦だけを見る
	mock.ExpectQuery("FROM `user_weight_scores`|session_id NOT LIKE ?|source <> ?").
		WithArgs(7, "interview-%", models.ScoreSourceAssessment, 1).
		WillReturnRows(sqlmock.NewRows([]string{"session_id"}).AddRow("s1"))
	mock.ExpectQuery("FROM `user_company_matches`|user_id = ? AND session_id = ? AND company_id IN").
		WithArgs(7, "s1", 1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"company_id", "match_id", "is_favorited"}).AddRow(2, 50, true))
	got, err := repo.FavoriteStates(7, []uint{1, 2})
	if err != nil || len(got) != 1 || got[0].MatchID != 50 || !got[0].IsFavorited {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	// 診断がまだ無ければ空
	repo, mock = newCompanySearchMock(t)
	mock.ExpectQuery("FROM `user_weight_scores`").WillReturnRows(sqlmock.NewRows([]string{"session_id"}))
	if got, err := repo.FavoriteStates(7, []uint{1}); err != nil || len(got) != 0 || got == nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
	// 企業が空なら問い合わせない
	if got, err := repo.FavoriteStates(7, nil); err != nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestIsPrefecture(t *testing.T) {
	if len(Prefectures) != 47 || !IsPrefecture("京都府") || IsPrefecture("京都") || IsPrefecture("%") {
		t.Fatal("都道府県の判定")
	}
}
