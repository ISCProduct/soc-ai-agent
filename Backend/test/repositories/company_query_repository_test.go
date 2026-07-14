package repositories_test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Backend/internal/repositories"
)

func newCompanyQueryRepoTestDB(t *testing.T) (*repositories.CompanyQueryRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock作成失敗: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open失敗: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return repositories.NewCompanyQueryRepository(db), mock
}

func TestGetCompaniesFiltered_Order(t *testing.T) {
	tests := []struct {
		name       string
		searchName string
		limit      int
		offset     int
		setupMock  func(sqlmock.Sqlmock)
		wantTotal  int64
		wantCount  int
	}{
		{
			name:       "デフォルトは updated_at DESC, id ASC",
			searchName: "",
			limit:      10,
			offset:     0,
			wantTotal:  2,
			wantCount:  2,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM `companies` WHERE is_active = \\?").
					WithArgs(true).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				mock.ExpectQuery("SELECT \\* FROM `companies` WHERE is_active = \\? ORDER BY updated_at DESC, id ASC LIMIT \\?").
					WithArgs(true, 10).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "is_active"}).
						AddRow(2, "B社", true).
						AddRow(1, "A社", true))
			},
		},
		{
			name:       "名前検索時は name ASC",
			searchName: "テック",
			limit:      10,
			offset:     0,
			wantTotal:  1,
			wantCount:  1,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM `companies` WHERE is_active = \\? AND name LIKE \\?").
					WithArgs(true, "%テック%").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery("SELECT \\* FROM `companies` WHERE is_active = \\? AND name LIKE \\? ORDER BY name ASC LIMIT \\?").
					WithArgs(true, "%テック%", 10).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "is_active"}).
						AddRow(1, "テック株式会社", true))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newCompanyQueryRepoTestDB(t)
			tt.setupMock(mock)

			companies, total, err := repo.GetCompaniesFiltered(tt.limit, tt.offset, "", tt.searchName, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if total != tt.wantTotal {
				t.Errorf("expected total %d, got %d", tt.wantTotal, total)
			}
			if len(companies) != tt.wantCount {
				t.Errorf("expected %d companies, got %d", tt.wantCount, len(companies))
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("expectations not met: %v", err)
			}
		})
	}
}
