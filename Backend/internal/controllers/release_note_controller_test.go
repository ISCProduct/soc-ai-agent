package controllers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/internal/controllers"
	"Backend/internal/services"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newReleaseNoteControllerTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestReleaseNoteController_List_ReturnsNotesNewestFirst(t *testing.T) {
	db, mock := newReleaseNoteControllerTestDB(t)
	svc := services.NewReleaseNoteService(db, nil)
	ctrl := controllers.NewReleaseNoteController(svc)
	e := echo.New()

	rows := sqlmock.NewRows([]string{"id", "pr_number", "title", "summary", "merged_at", "created_at"}).
		AddRow(1, 100, "更新情報ページを追加", "更新情報を確認できるようになりました。", time.Now(), time.Now())
	mock.ExpectQuery("SELECT \\* FROM `release_notes` ORDER BY merged_at DESC LIMIT \\?").
		WithArgs(20).
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/api/whats-new", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := ctrl.List(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(body) != 1 || body[0]["title"] != "更新情報ページを追加" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestReleaseNoteController_Ingest_InvalidPayload(t *testing.T) {
	db, _ := newReleaseNoteControllerTestDB(t)
	svc := services.NewReleaseNoteService(db, nil)
	ctrl := controllers.NewReleaseNoteController(svc)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/whats-new/ingest", bytes.NewReader([]byte("not-json")))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := ctrl.Ingest(ctx)
	if err == nil {
		t.Fatal("expected error for invalid payload")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 HTTPError, got %v", err)
	}
}

func TestReleaseNoteController_Ingest_NilLLMClientReturnsInternalError(t *testing.T) {
	db, _ := newReleaseNoteControllerTestDB(t)
	svc := services.NewReleaseNoteService(db, nil)
	ctrl := controllers.NewReleaseNoteController(svc)
	e := echo.New()

	payload := map[string]any{
		"sources": []map[string]any{
			{"pr_number": 1, "title": "t", "body": "b", "merged_at": time.Now()},
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/whats-new/ingest", bytes.NewReader(b))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := ctrl.Ingest(ctx)
	if err == nil {
		t.Fatal("expected error when llm client is nil")
	}
}
