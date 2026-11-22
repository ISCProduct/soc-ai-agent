package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers/admin"
	"Backend/internal/repositories"
	"Backend/internal/services/training"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SOCAIAGENT-694: training export の limit/offset の検証と、続きの offset をヘッダーで返すこと。
func TestAdminTrainingController_Export_Paging(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		rows       int
		wantStatus int
		wantNext   string
	}{
		{"offset が負なら 400", "?offset=-1", 0, http.StatusBadRequest, ""},
		{"offset が数値でなければ 400", "?offset=x", 0, http.StatusBadRequest, ""},
		{"limit が負なら 400", "?limit=-1", 0, http.StatusBadRequest, ""},
		{"続きが無ければヘッダーなし", "?limit=2", 1, http.StatusOK, ""},
		{"limit 件取れたら次の offset をヘッダーで返す", "?limit=1&offset=3", 1, http.StatusOK, "4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantStatus == http.StatusOK {
				rows := sqlmock.NewRows([]string{"session_id", "status"})
				utts := sqlmock.NewRows([]string{"session_id", "text"})
				for i := range tt.rows {
					rows.AddRow(i+1, "offered")
					utts.AddRow(i+1, "合成の発話")
				}
				mock.ExpectQuery("SELECT s.id AS session_id").WillReturnRows(rows)
				if tt.rows > 0 {
					mock.ExpectQuery("SELECT session_id, text").WillReturnRows(utts)
				}
			}

			ctrl := admin.NewAdminTrainingController(training.NewService(repositories.NewTrainingExportRepository(db)))
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/admin/training/export"+tt.query, nil), rec)
			status := http.StatusOK
			if err := ctrl.Export(c); err != nil {
				he, ok := err.(*echo.HTTPError)
				if !ok {
					t.Fatalf("予期しないエラー: %v", err)
				}
				status = he.Code
			} else {
				status = rec.Code
			}
			if status != tt.wantStatus {
				t.Fatalf("status=%d want %d", status, tt.wantStatus)
			}
			if got := rec.Header().Get("X-Next-Offset"); got != tt.wantNext {
				t.Errorf("X-Next-Offset=%q want %q", got, tt.wantNext)
			}
		})
	}
}
