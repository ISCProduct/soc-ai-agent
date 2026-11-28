package admin_test

// 診断妥当性レポートの参照API（GET /api/admin/diagnosis-quality）のテスト。
// 応答のステータス・形と、呼ぶ SQL を確かめる。

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	admincontrollers "Backend/internal/controllers/admin"
	"Backend/internal/controllers/testsupport"
	"Backend/internal/repositories"
	"Backend/internal/services/diagnosis"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	selectQualityBySession = regexp.QuoteMeta("SELECT * FROM `diagnosis_quality_reports` WHERE user_id = ? AND session_id = ?")
	selectQualityRecent    = regexp.QuoteMeta("SELECT * FROM `diagnosis_quality_reports` ORDER BY updated_at DESC LIMIT ?")
)

func newDiagnosisQualityController(t *testing.T) (*admincontrollers.AdminDiagnosisQualityController, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return admincontrollers.NewAdminDiagnosisQualityController(diagnosis.NewQualityService(repositories.NewDiagnosisQualityRepository(db), nil, nil, nil, nil)), mock
}

func diagnosisQualityRequest(query string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/api/admin/diagnosis-quality"+query, nil)
}

func decodeDiagnosisQuality(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("JSON でない: %q", rec.Body.String())
	}
	return body
}

func TestAdminDiagnosisQuality_GetBySession(t *testing.T) {
	t.Run("見つかればレポートを返す", func(t *testing.T) {
		c, mock := newDiagnosisQualityController(t)
		mock.ExpectQuery(selectQualityBySession).WithArgs(uint(7), "sess-1", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "session_id", "confidence", "summary"}).AddRow(1, 7, "sess-1", 80, "合成サマリー"))
		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, c.List, testsupport.NewCtx(diagnosisQualityRequest("?user_id=7&session_id=sess-1"), rec), http.StatusOK)
		body := decodeDiagnosisQuality(t, rec)
		assert.Equal(t, float64(80), body["Confidence"])
		assert.Equal(t, "合成サマリー", body["Summary"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("無ければ404", func(t *testing.T) {
		c, mock := newDiagnosisQualityController(t)
		mock.ExpectQuery(selectQualityBySession).WithArgs(uint(7), "sess-1", 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, c.GetBySession, testsupport.NewCtx(diagnosisQualityRequest("?user_id=7&session_id=sess-1"), rec), http.StatusNotFound)
		assert.Equal(t, "not found", decodeDiagnosisQuality(t, rec)["error"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DBエラーは500でエラー文言を返す", func(t *testing.T) {
		c, mock := newDiagnosisQualityController(t)
		mock.ExpectQuery(selectQualityBySession).WithArgs(uint(7), "sess-1", 1).WillReturnError(errors.New("db down"))
		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, c.GetBySession, testsupport.NewCtx(diagnosisQualityRequest("?user_id=7&session_id=sess-1"), rec), http.StatusInternalServerError)
		assert.Equal(t, "db down", decodeDiagnosisQuality(t, rec)["error"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("session_id が無ければ400", func(t *testing.T) {
		c, mock := newDiagnosisQualityController(t)
		testsupport.AssertStatus(t, c.GetBySession, testsupport.NewCtx(diagnosisQualityRequest("?user_id=7"), httptest.NewRecorder()), http.StatusBadRequest)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("user_id が無ければ400", func(t *testing.T) {
		c, mock := newDiagnosisQualityController(t)
		testsupport.AssertStatus(t, c.GetBySession, testsupport.NewCtx(diagnosisQualityRequest("?session_id=sess-1"), httptest.NewRecorder()), http.StatusBadRequest)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAdminDiagnosisQuality_List(t *testing.T) {
	t.Run("直近の一覧を items で返す（limit 未指定は50件）", func(t *testing.T) {
		c, mock := newDiagnosisQualityController(t)
		mock.ExpectQuery(selectQualityRecent).WithArgs(50).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "session_id"}).AddRow(1, 7, "sess-1").AddRow(2, 8, "sess-2"))
		rec := httptest.NewRecorder()
		testsupport.AssertStatus(t, c.List, testsupport.NewCtx(diagnosisQualityRequest(""), rec), http.StatusOK)
		items, _ := decodeDiagnosisQuality(t, rec)["items"].([]any)
		assert.Len(t, items, 2)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// 上限 100 で頭打ち（SOCAIAGENT-530）。SQL の LIMIT に渡る値で確かめる。
	limitCases := []struct {
		name      string
		query     string
		wantLimit int
	}{
		{name: "上限内はそのまま", query: "?limit=10", wantLimit: 10},
		{name: "上限ちょうど", query: "?limit=100", wantLimit: 100},
		{name: "上限超過は100へ頭打ち", query: "?limit=101", wantLimit: 100},
		{name: "極端な値も100へ頭打ち", query: "?limit=1000000", wantLimit: 100},
		{name: "0は既定の50", query: "?limit=0", wantLimit: 50},
		{name: "数値以外は既定の50", query: "?limit=abc", wantLimit: 50},
	}
	for _, tt := range limitCases {
		t.Run("limit: "+tt.name, func(t *testing.T) {
			c, mock := newDiagnosisQualityController(t)
			mock.ExpectQuery(selectQualityRecent).WithArgs(tt.wantLimit).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			testsupport.AssertStatus(t, c.List, testsupport.NewCtx(diagnosisQualityRequest(tt.query), httptest.NewRecorder()), http.StatusOK)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}

	t.Run("DBエラーは500", func(t *testing.T) {
		c, mock := newDiagnosisQualityController(t)
		mock.ExpectQuery(selectQualityRecent).WithArgs(50).WillReturnError(errors.New("db down"))
		testsupport.AssertStatus(t, c.List, testsupport.NewCtx(diagnosisQualityRequest(""), httptest.NewRecorder()), http.StatusInternalServerError)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
