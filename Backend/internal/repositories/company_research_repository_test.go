package repositories

import (
	"context"
	"testing"
	"time"

	"database/sql/driver"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/models"
)

func newResearchRepo(t *testing.T) (*CompanyResearchRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return NewCompanyResearchRepository(db), mock
}

func docCols() []string {
	return []string{"id", "company_id", "source_type", "source_ref", "title", "url", "as_of", "fetched_at", "content_hash", "body", "created_at", "updated_at"}
}

// 本文のハッシュが同じなら書き込まず changed=false。変わったら更新、無ければ作成する。
func TestCompanyResearchRepository_UpsertDocument(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		existing    []any // nil なら行なし
		wantChanged bool
		expectWrite func(m sqlmock.Sqlmock)
	}{
		{
			name:        "新規なら作成する",
			wantChanged: true,
			expectWrite: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO `company_documents`").WillReturnResult(sqlmock.NewResult(5, 1))
			},
		},
		{
			name:        "本文が同じなら書かない",
			existing:    []any{3, 1, "edinet", "S100AAAA:business", "事業の内容", "", nil, now, "hash-1", "本文", now, now},
			wantChanged: false,
			expectWrite: func(m sqlmock.Sqlmock) {},
		},
		{
			name:        "本文が変わったら更新する",
			existing:    []any{3, 1, "edinet", "S100AAAA:business", "事業の内容", "", nil, now, "hash-0", "古い本文", now, now},
			wantChanged: true,
			expectWrite: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE `company_documents` SET").WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newResearchRepo(t)
			mock.ExpectBegin()
			rows := sqlmock.NewRows(docCols())
			if tt.existing != nil {
				rows.AddRow(toDriverValues(tt.existing)...)
			}
			mock.ExpectQuery("SELECT \\* FROM `company_documents` WHERE .* FOR UPDATE").
				WithArgs(1, "edinet", "S100AAAA:business", 1).
				WillReturnRows(rows)
			tt.expectWrite(mock)
			mock.ExpectCommit()

			doc := &models.CompanyDocument{CompanyID: 1, SourceType: "edinet", SourceRef: "S100AAAA:business",
				Title: "事業の内容", FetchedAt: now, ContentHash: "hash-1", Body: "本文"}
			changed, err := repo.UpsertDocument(context.Background(), doc)
			require.NoError(t, err)
			require.Equal(t, tt.wantChanged, changed)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 数字は (company_id, field, source_type) の重複で上書きする。空なら何もしない。
func TestCompanyResearchRepository_UpsertFacts(t *testing.T) {
	repo, mock := newResearchRepo(t)
	require.NoError(t, repo.UpsertFacts(context.Background(), nil))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `company_facts` .* ON DUPLICATE KEY UPDATE `value`=VALUES\\(`value`\\)").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectCommit()
	err := repo.UpsertFacts(context.Background(), []models.CompanyFact{
		{CompanyID: 1, Field: "average_age", Value: "41.2", Unit: "歳", SourceType: "edinet", FetchedAt: time.Now()},
		{CompanyID: 1, Field: "employees_single", Value: "320", Unit: "人", SourceType: "edinet", FetchedAt: time.Now()},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func toDriverValues(vs []any) []driver.Value {
	out := make([]driver.Value, len(vs))
	for i, v := range vs {
		out[i] = v
	}
	return out
}
