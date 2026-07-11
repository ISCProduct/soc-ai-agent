package models_test

import (
	"testing"

	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newCleanupTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	return db, mock
}

func TestCleanupDemoCompanies_IdempotentWhenNoDemoData(t *testing.T) {
	db, mock := newCleanupTestDB(t)

	mock.ExpectBegin()
	for i := 0; i < 13; i++ {
		mock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit()

	err := models.CleanupDemoCompanies(db)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
