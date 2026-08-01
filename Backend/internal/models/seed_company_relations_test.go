package models

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestSeedCompanyRelations_SkipsWhenFactRelationsExist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	gdb, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `company_relations` SET `deleted_at`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `company_relations`").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(2))

	if err := seedCompanyRelations(gdb); err != nil {
		t.Fatalf("seedCompanyRelations: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEnsureCompanyByCorporateNumber_UsesExistingRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	gdb, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}

	mock.ExpectQuery("SELECT .* FROM `companies`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "corporate_number"}).
			AddRow(7180301018923, "トヨタテクニカルディベロップメント株式会社", "7180301018923"))

	id, err := ensureCompanyByCorporateNumber(gdb, "7180301018923", "トヨタテクニカルディベロップメント株式会社")
	if err != nil {
		t.Fatalf("ensureCompanyByCorporateNumber: %v", err)
	}
	if id != 7180301018923 {
		t.Fatalf("expected id 7180301018923, got %d", id)
	}
}
