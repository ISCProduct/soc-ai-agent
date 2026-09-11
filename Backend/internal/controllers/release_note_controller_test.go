package controllers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/internal/controllers"
	"Backend/internal/middleware"
	"Backend/internal/repositories"
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

// newAuthenticatedReleaseNoteRequest はEchoUserAuthミドルウェアが設定するuserIDを
// コンテキストに載せたリクエストを組み立てる。
func newAuthenticatedReleaseNoteRequest(userID uint) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodGet, "/api/whats-new", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDContextKey, userID)
	req = req.WithContext(ctx)
	return req, httptest.NewRecorder()
}

func TestReleaseNoteController_List_ReturnsNotesNewestFirst(t *testing.T) {
	db, mock := newReleaseNoteControllerTestDB(t)
	svc := services.NewReleaseNoteService(db, nil)
	userRepo := repositories.NewUserRepository(db)
	ctrl := controllers.NewReleaseNoteController(svc, userRepo)
	e := echo.New()

	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role", "is_admin"}).AddRow(1, "student", false))

	rows := sqlmock.NewRows([]string{"id", "pr_number", "title", "summary", "audience", "merged_at", "created_at"}).
		AddRow(1, 100, "更新情報ページを追加", "更新情報を確認できるようになりました。", "all", time.Now(), time.Now())
	mock.ExpectQuery("SELECT \\* FROM `release_notes` WHERE audience IN \\(\\?,\\?\\) ORDER BY merged_at DESC LIMIT \\?").
		WithArgs("all", "student", 20).
		WillReturnRows(rows)

	req, rec := newAuthenticatedReleaseNoteRequest(1)
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

// #966: システム管理者(is_admin=true)にはstudent/teacherの絞り込みではなくadmin向けの絞り込みが渡ること。
func TestReleaseNoteController_List_UsesAdminAudienceForAdminUser(t *testing.T) {
	db, mock := newReleaseNoteControllerTestDB(t)
	svc := services.NewReleaseNoteService(db, nil)
	userRepo := repositories.NewUserRepository(db)
	ctrl := controllers.NewReleaseNoteController(svc, userRepo)
	e := echo.New()

	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? ORDER BY `users`.`id` LIMIT \\?").
		WithArgs(uint(2), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role", "is_admin"}).AddRow(2, "student", true))

	rows := sqlmock.NewRows([]string{"id", "pr_number", "title", "summary", "audience", "merged_at", "created_at"})
	mock.ExpectQuery("SELECT \\* FROM `release_notes` WHERE audience IN \\(\\?,\\?\\) ORDER BY merged_at DESC LIMIT \\?").
		WithArgs("all", "admin", 20).
		WillReturnRows(rows)

	req, rec := newAuthenticatedReleaseNoteRequest(2)
	ctx := e.NewContext(req, rec)

	if err := ctrl.List(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// #966: 未認証（userIDがコンテキストにない）場合は401を返すこと。
func TestReleaseNoteController_List_Unauthenticated(t *testing.T) {
	db, _ := newReleaseNoteControllerTestDB(t)
	svc := services.NewReleaseNoteService(db, nil)
	userRepo := repositories.NewUserRepository(db)
	ctrl := controllers.NewReleaseNoteController(svc, userRepo)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/whats-new", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := ctrl.List(ctx)
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 HTTPError, got %v", err)
	}
}

func TestReleaseNoteController_Ingest_InvalidPayload(t *testing.T) {
	db, _ := newReleaseNoteControllerTestDB(t)
	svc := services.NewReleaseNoteService(db, nil)
	userRepo := repositories.NewUserRepository(db)
	ctrl := controllers.NewReleaseNoteController(svc, userRepo)
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
	userRepo := repositories.NewUserRepository(db)
	ctrl := controllers.NewReleaseNoteController(svc, userRepo)
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
