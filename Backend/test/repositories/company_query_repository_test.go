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

// publicCompanyGuards は無認証の企業APIに必ず乗る絞り込み。
// data_status が抜けると審査前のゲスト投稿企業が学生に見えるため、
// どのフィルタ条件でもこの2つが SQL に含まれることを強制する（#1074）。
const publicCompanyGuards = "is_active = .*data_status = "

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
				mock.ExpectQuery("SELECT count.*"+publicCompanyGuards).
					WithArgs(true, "published").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				mock.ExpectQuery(publicCompanyGuards+".*ORDER BY updated_at DESC, id ASC LIMIT").
					WithArgs(true, "published", 10).
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
				mock.ExpectQuery("SELECT count.*"+publicCompanyGuards+".*name LIKE").
					WithArgs(true, "published", "%テック%").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(publicCompanyGuards+".*name LIKE.*ORDER BY name ASC LIMIT").
					WithArgs(true, "published", "%テック%", 10).
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

// TestPublicCompanyQueries_ExcludeDraft は、無認証の企業APIが審査前(draft)の企業を
// 返さないことを検証する。
//
// company-entry からゲストが投稿した企業は data_status='draft' で作られる
// (company_entry_service.go:173)。この絞り込みが抜けると、審査前の企業情報が
// 誰にでも見え、学生の企業検索や企業詳細画面にも出てしまう（#1074）。
func TestPublicCompanyQueries_ExcludeDraft(t *testing.T) {
	t.Run("一覧は published のみを引く", func(t *testing.T) {
		repo, mock := newCompanyQueryRepoTestDB(t)
		mock.ExpectQuery("SELECT count.*"+publicCompanyGuards).
			WithArgs(true, "published").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(publicCompanyGuards).
			WithArgs(true, "published", 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

		if _, _, err := repo.GetCompaniesFiltered(10, 0, "", "", ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("draft を除外する条件が SQL に含まれていない: %v", err)
		}
	})

	t.Run("企業詳細も published のみを引く", func(t *testing.T) {
		repo, mock := newCompanyQueryRepoTestDB(t)
		mock.ExpectQuery(publicCompanyGuards).
			WithArgs(uint(1), true, "published", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "公開企業"))

		if _, err := repo.GetCompanyByID(1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("draft を除外する条件が SQL に含まれていない: %v", err)
		}
	})
}
