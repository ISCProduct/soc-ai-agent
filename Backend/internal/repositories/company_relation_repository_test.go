package repositories_test

// CompanyRelationRepository の資本/ビジネス Upsert テスト（Issue #572）
// 実行: cd Backend && go test ./test/repositories/ -run TestCompanyRelation -v

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/models"
	"Backend/internal/repositories"
)

func newRelationRepoTestDB(t *testing.T) (*repositories.CompanyRelationRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	dialector := mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	})
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return repositories.NewCompanyRelationRepository(db), mock
}

func TestUpsertCapitalRelation_CreatesWithParentChild(t *testing.T) {
	repo, mock := newRelationRepoTestDB(t)

	mock.ExpectQuery("SELECT .* FROM `company_relations`").
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `company_relations`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	ratio := 100.0
	if err := repo.UpsertCapitalRelation(1, 2, "capital_subsidiary", &ratio, "完全子会社"); err != nil {
		t.Fatalf("UpsertCapitalRelation failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未充足の SQL 期待: %v", err)
	}
}

func TestUpsertBusinessRelation_CreatesWithFromTo(t *testing.T) {
	repo, mock := newRelationRepoTestDB(t)

	mock.ExpectQuery("SELECT .* FROM `company_relations`").
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `company_relations`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := repo.UpsertBusinessRelation(1, 2, "business_partner", "業務提携"); err != nil {
		t.Fatalf("UpsertBusinessRelation failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未充足の SQL 期待: %v", err)
	}
}

func TestIsCapitalRelationType(t *testing.T) {
	if !models.IsCapitalRelationType("capital_subsidiary") {
		t.Fatal("expected capital_subsidiary to be capital")
	}
	if models.IsCapitalRelationType("business_partner") {
		t.Fatal("expected business_partner not to be capital")
	}
}
