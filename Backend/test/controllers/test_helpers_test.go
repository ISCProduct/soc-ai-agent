package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/labstack/echo/v4"
)

var testEcho = echo.New()

// withUserID はリクエストコンテキストにuserIDを設定するヘルパー
func withUserID(r *http.Request, userID uint) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey, userID)
	return r.WithContext(ctx)
}

func withAdminUserID(r *http.Request, adminUserID uint) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.AdminUserIDContextKey, adminUserID)
	return r.WithContext(ctx)
}

// newUnrestrictedSchoolService は担当校未割当(=無制限admin、CanAdminAccessSchoolが常にtrue)の
// SchoolServiceを返すテスト用ヘルパー(#980/#981/#982/#984)
func newUnrestrictedSchoolService(adminUserID uint) *services.SchoolService {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", adminUserID).Return([]models.School{}, nil)
	return services.NewSchoolService(repo)
}

// newCtx はリクエストとレコーダーからecho.Contextを生成する
func newCtx(req *http.Request, rec *httptest.ResponseRecorder) echo.Context {
	return testEcho.NewContext(req, rec)
}

// assertStatus はハンドラーを呼び出してHTTPステータスを検証するヘルパー
func assertStatus(t *testing.T, handler func(echo.Context) error, c echo.Context, expected int) {
	t.Helper()
	err := handler(c)
	if err != nil {
		testEcho.HTTPErrorHandler(err, c)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	if rec.Code != expected {
		t.Errorf("expected status %d, got %d", expected, rec.Code)
	}
}
