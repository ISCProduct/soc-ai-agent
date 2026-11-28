package repositories

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newBusinessTagRepoTest(t *testing.T) (*CompanyBusinessTagRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm作成失敗: %v", err)
	}
	return NewCompanyBusinessTagRepository(db), mock
}

// 一覧への追加は重複を無視し、紐付けは名前で読み直した ID で作る。
// INSERT 後に GORM が埋める ID は重複行があるとずれるので使ってはいけない。
func TestCompanyBusinessTagRepository_ReplaceForCompany(t *testing.T) {
	repo, mock := newBusinessTagRepoTest(t)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `company_business_tags` WHERE company_id = \\?").
		WithArgs(uint(5)).WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("INSERT INTO `business_tags` .* ON DUPLICATE KEY UPDATE").
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectQuery("SELECT \\* FROM `business_tags` WHERE name IN \\(\\?,\\?\\)").
		WithArgs("クラウド", "業務システム").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(7, "クラウド").AddRow(100, "業務システム"))
	mock.ExpectExec("INSERT INTO `company_business_tags` .*VALUES \\(\\?,\\?,\\?\\),\\(\\?,\\?,\\?\\)").
		WithArgs(uint(5), uint(7), sqlmock.AnyArg(), uint(5), uint(100), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	if err := repo.ReplaceForCompany(context.Background(), 5, []string{"クラウド", "業務システム"}); err != nil {
		t.Fatalf("ReplaceForCompany: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 空配列なら紐付けを消すだけで、一覧には触らない。
func TestCompanyBusinessTagRepository_ReplaceForCompany_Empty(t *testing.T) {
	repo, mock := newBusinessTagRepoTest(t)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `company_business_tags` WHERE company_id = \\?").
		WithArgs(uint(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.ReplaceForCompany(context.Background(), 5, []string{}); err != nil {
		t.Fatalf("ReplaceForCompany: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
