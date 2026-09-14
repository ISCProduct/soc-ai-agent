package repositories_test

import (
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/repositories"
)

// newAppStatusRepo は発行SQLを記録する sqlmock 付きのリポジトリを返す。
func newAppStatusRepo(t *testing.T) (*repositories.UserApplicationStatusRepository, sqlmock.Sqlmock, *[]string) {
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
	return repositories.NewUserApplicationStatusRepository(db), mock, captured
}

// TestFindAll_ScopedBySchool は担当校の絞り込みがクエリ条件に入ることを検証する（#1157）。
//
// コントローラのテストは *uint がサービス層へ渡ることしか見ないため、
// JOIN や列修飾の誤りはここでしか検出できない。
func TestFindAll_ScopedBySchool(t *testing.T) {
	repo, mock, captured := newAppStatusRepo(t)
	schoolID := uint(5)

	mock.ExpectQuery("SELECT .* FROM `user_application_statuses` JOIN users ON users.id = user_application_statuses.user_id WHERE users.school_id = \\?").
		WithArgs(schoolID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "company_id", "status"}))

	if _, err := repo.FindAll(0, 0, "", &schoolID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	sql := (*captured)[0]
	// created_at は users 側にも存在するため、修飾を外すと MySQL が
	// "Column 'created_at' in order clause is ambiguous" で落ちる
	if !strings.Contains(sql, "ORDER BY user_application_statuses.created_at") {
		t.Errorf("ORDER BY が修飾されていない: %s", sql)
	}
}

// TestFindAll_NoSchoolFilterKeepsQueryUnchanged は絞り込みなし(プラットフォーム管理者・
// 企業オーナー経路)では JOIN が付かないことを検証する。
func TestFindAll_NoSchoolFilterKeepsQueryUnchanged(t *testing.T) {
	repo, mock, captured := newAppStatusRepo(t)

	mock.ExpectQuery("SELECT \\* FROM `user_application_statuses` WHERE user_id = \\?").
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "company_id", "status"}))

	if _, err := repo.FindAll(7, 0, "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains((*captured)[0], "JOIN users") {
		t.Errorf("絞り込みなしなのに JOIN が付いている: %s", (*captured)[0])
	}
}

// TestFindOwnerSchoolID は応募の所有ユーザーの学校IDを users から引くことを検証する（#1157）。
func TestFindOwnerSchoolID(t *testing.T) {
	repo, mock, _ := newAppStatusRepo(t)

	mock.ExpectQuery("SELECT users.school_id AS school_id FROM `user_application_statuses` JOIN users ON users.id = user_application_statuses.user_id WHERE user_application_statuses.id = \\?").
		WithArgs(uint(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"school_id"}).AddRow(9))

	got, err := repo.FindOwnerSchoolID(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || *got != 9 {
		t.Fatalf("school_id = %v, want 9", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestFindOwnerSchoolID_NullSchool は学校未所属の生徒では nil が返ることを検証する。
// nil は CanAdminAccessSchool 側で「制限adminには見せない」と解釈される（fail-closed）。
func TestFindOwnerSchoolID_NullSchool(t *testing.T) {
	repo, mock, _ := newAppStatusRepo(t)

	mock.ExpectQuery("SELECT users.school_id AS school_id FROM `user_application_statuses`").
		WithArgs(uint(43), 1).
		WillReturnRows(sqlmock.NewRows([]string{"school_id"}).AddRow(nil))

	got, err := repo.FindOwnerSchoolID(43)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("school_id = %v, want nil", *got)
	}
}
