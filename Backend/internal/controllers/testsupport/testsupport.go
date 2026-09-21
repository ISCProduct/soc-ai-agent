// Package testsupport はコントローラーのテストで共有するヘルパーを提供する。
//
// 以前は test/controllers/test_helpers_test.go にあったが、テストを各
// コントローラーのパッケージへ移すにあたり、_test.go のままでは他パッケージ
// から参照できないため通常のパッケージへ昇格させた。
package testsupport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers/mocks"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
)

// Echo はテスト用の echo インスタンス。HTTPErrorHandler を直接使う
// テストがあるため公開している。
var Echo = echo.New()

// WithUserID はリクエストコンテキストにuserIDを設定するヘルパー
func WithUserID(r *http.Request, userID uint) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey, userID)
	return r.WithContext(ctx)
}

func WithAdminUserID(r *http.Request, adminUserID uint) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.AdminUserIDContextKey, adminUserID)
	return r.WithContext(ctx)
}

// NewUnrestrictedSchoolService は担当校未割当(=無制限admin、CanAdminAccessSchoolが常にtrue)の
// SchoolServiceを返すテスト用ヘルパー(#980/#981/#982/#984)
func NewUnrestrictedSchoolService(adminUserID uint) *school.SchoolService {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", adminUserID).Return([]models.School{}, nil)
	return school.NewSchoolService(repo)
}

// NewRestrictedSchoolService は担当校を持つadmin(=先生)のSchoolServiceを返す。
// allowedSchoolID 以外の学校のリソースに対して CanAdminAccessSchool が false を返す(#1157)
func NewRestrictedSchoolService(adminUserID, allowedSchoolID uint) *school.SchoolService {
	repo := &mocks.SchoolRepositoryMock{}
	repo.On("ListSchoolsForAdmin", adminUserID).
		Return([]models.School{{ID: allowedSchoolID, Name: "担当校"}}, nil)
	return school.NewSchoolService(repo)
}

// NewCtx はリクエストとレコーダーからecho.Contextを生成する
func NewCtx(req *http.Request, rec *httptest.ResponseRecorder) echo.Context {
	return Echo.NewContext(req, rec)
}

// AssertStatus はハンドラーを呼び出してHTTPステータスを検証するヘルパー
func AssertStatus(t *testing.T, handler func(echo.Context) error, c echo.Context, expected int) {
	t.Helper()
	err := handler(c)
	if err != nil {
		Echo.HTTPErrorHandler(err, c)
	}
	rec := c.Response().Writer.(*httptest.ResponseRecorder)
	if rec.Code != expected {
		t.Errorf("expected status %d, got %d", expected, rec.Code)
	}
}

// WithSchoolFilter は admin の担当校絞り込みをリクエストコンテキストに載せる。
func WithSchoolFilter(r *http.Request, schoolID *uint) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.AdminSchoolFilterContextKey, schoolID))
}
