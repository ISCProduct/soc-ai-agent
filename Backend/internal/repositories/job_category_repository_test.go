package repositories

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newJobCategoryTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

func TestFindByIDs_EmptySkipsQuery(t *testing.T) {
	db, mock := newJobCategoryTestDB(t)
	repo := NewJobCategoryRepository(db)

	got, err := repo.FindByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindByIDs_OneQuery(t *testing.T) {
	db, mock := newJobCategoryTestDB(t)
	repo := NewJobCategoryRepository(db)
	mock.ExpectQuery("SELECT .* FROM `job_categories` WHERE id IN").
		WithArgs(uint(9), uint(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(uint(2), "営業").
			AddRow(uint(9), "研究"))

	got, err := repo.FindByIDs(context.Background(), []uint{9, 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 9 {
		t.Fatalf("got %#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindByIDs_ReturnsQueryError(t *testing.T) {
	db, mock := newJobCategoryTestDB(t)
	repo := NewJobCategoryRepository(db)
	mock.ExpectQuery("SELECT .* FROM `job_categories` WHERE id IN").
		WillReturnError(errors.New("db down"))

	if _, err := repo.FindByIDs(context.Background(), []uint{1}); err == nil {
		t.Fatal("expected error, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
