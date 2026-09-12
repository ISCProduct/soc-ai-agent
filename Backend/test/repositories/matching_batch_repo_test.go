package repositories_test

import (
	"errors"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/domain/entity"
	"Backend/internal/repositories"
)

func TestGetWeightProfilesByCompanyIDs(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewCompanyRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `company_weight_profiles` WHERE company_id IN \\(\\?,\\?\\) AND job_position_id IS NULL").
		WithArgs(1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "technical_orientation"}).
			AddRow(10, 1, 80).
			AddRow(11, 2, 40))

	got, err := repo.GetWeightProfilesByCompanyIDs([]uint{1, 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[1] == nil || got[1].TechnicalOrientation != 80 {
		t.Fatalf("unexpected profiles: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetWeightProfilesByCompanyIDs_Empty(t *testing.T) {
	repo := repositories.NewCompanyRepository(nil)
	got, err := repo.GetWeightProfilesByCompanyIDs(nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty ids should return empty map, got %v err %v", got, err)
	}
}

// newMatchRepoMock は SQL を記録しつつ正規表現マッチする sqlmock を返す。
// 「UPDATE が件数分のクエリにならないこと」(#1166 の受け入れ条件) を実SQLで検証するため。
func newMatchRepoMock(t *testing.T) (*repositories.UserCompanyMatchRepository, sqlmock.Sqlmock, *[]string) {
	t.Helper()
	captured := &[]string{}
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		*captured = append(*captured, actualSQL)
		return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
	})
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return repositories.NewUserCompanyMatchRepository(db), mock, captured
}

// TestCreateOrUpdateBatch_SingleUpsert は作成と更新が1回の upsert にまとまることを検証する。
func TestCreateOrUpdateBatch_SingleUpsert(t *testing.T) {
	repo, mock, captured := newMatchRepoMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_company_matches` .* ON DUPLICATE KEY UPDATE").
		WillReturnResult(sqlmock.NewResult(100, 3))
	mock.ExpectCommit()

	saved, err := repo.CreateOrUpdateBatch([]*entity.UserCompanyMatch{
		{UserID: 1, SessionID: "s1", CompanyID: 1, MatchScore: 80},
		{UserID: 1, SessionID: "s1", CompanyID: 2, MatchScore: 90},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved != 2 {
		t.Fatalf("saved=%d want 2", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	if len(*captured) != 1 {
		t.Fatalf("発行クエリ数=%d want 1: %v", len(*captured), *captured)
	}
	sql := (*captured)[0]
	if strings.Contains(sql, "UPDATE `user_company_matches` SET") {
		t.Errorf("件数分の UPDATE が残っている: %s", sql)
	}
	// ユーザー操作の結果は再計算で消さない
	for _, col := range []string{"is_viewed", "is_favorited", "is_applied"} {
		if strings.Contains(sql[strings.Index(sql, "ON DUPLICATE KEY UPDATE"):], col) {
			t.Errorf("%s が衝突時の更新対象に含まれている: %s", col, sql)
		}
	}
}

// TestCreateOrUpdateBatch_PropagatesError は保存失敗が呼び出し元へ伝わることを検証する（#1166）。
func TestCreateOrUpdateBatch_PropagatesError(t *testing.T) {
	repo, mock, _ := newMatchRepoMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_company_matches`").
		WillReturnError(errors.New("deadlock"))
	mock.ExpectRollback()

	saved, err := repo.CreateOrUpdateBatch([]*entity.UserCompanyMatch{
		{UserID: 1, SessionID: "s1", CompanyID: 1, MatchScore: 80},
	})
	if err == nil {
		t.Fatal("保存失敗がエラーとして返っていない")
	}
	if saved != 0 {
		t.Fatalf("saved=%d want 0", saved)
	}
}

// TestCreateOrUpdateBatch_SkipsOtherUserSession は先頭要素と user/session が違う行を無視する既存挙動を固定する。
func TestCreateOrUpdateBatch_SkipsOtherUserSession(t *testing.T) {
	repo, mock, captured := newMatchRepoMock(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_company_matches`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	saved, err := repo.CreateOrUpdateBatch([]*entity.UserCompanyMatch{
		{UserID: 1, SessionID: "s1", CompanyID: 1, MatchScore: 80},
		{UserID: 2, SessionID: "s1", CompanyID: 1, MatchScore: 80},
		nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved != 1 {
		t.Fatalf("saved=%d want 1", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if len(*captured) != 1 {
		t.Fatalf("発行クエリ数=%d want 1: %v", len(*captured), *captured)
	}
}

// TestCreateOrUpdateBatch_Empty は空入力でクエリを発行しないことを検証する。
func TestCreateOrUpdateBatch_Empty(t *testing.T) {
	repo := repositories.NewUserCompanyMatchRepository(nil)
	saved, err := repo.CreateOrUpdateBatch(nil)
	if err != nil || saved != 0 {
		t.Fatalf("saved=%d err=%v want 0/nil", saved, err)
	}
}
