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
	// 子テーブル削除 11 + 両端デモ関係削除 1 + 混在FK解除 4 + 企業削除 1 = 17
	for i := 0; i < 17; i++ {
		mock.ExpectExec("(?i)(DELETE FROM|UPDATE)").WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit()

	err := models.CleanupDemoCompanies(db)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanupDemoCompanies_RelationsRequireBothDemoEndpoints(t *testing.T) {
	db, mock := newCleanupTestDB(t)

	mock.ExpectBegin()
	for i := 0; i < 11; i++ {
		mock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 0))
	}
	// 片端デモの OR 連鎖ではなく、parent+child / from+to の両端デモ条件
	mock.ExpectExec(`DELETE FROM company_relations WHERE \(parent_id IS NOT NULL AND child_id IS NOT NULL AND parent_id IN .+ AND child_id IN .+\) OR \(from_id IS NOT NULL AND to_id IS NOT NULL AND from_id IN .+ AND to_id IN .+\)`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`UPDATE company_relations SET parent_id = NULL WHERE parent_id IN`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`UPDATE company_relations SET child_id = NULL WHERE child_id IN`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`UPDATE company_relations SET from_id = NULL WHERE from_id IN`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`UPDATE company_relations SET to_id = NULL WHERE to_id IN`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM companies").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := models.CleanupDemoCompanies(db)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
