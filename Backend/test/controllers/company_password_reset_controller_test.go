package controllers_test

// #1196 企業ポータルのパスワードリセットのHTTP層テスト
//
// 実行: cd Backend && go test ./test/controllers/... -run CompanyPasswordReset -v

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/repositories"
	companyauth "Backend/internal/services/companyauth"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCompanyAuthControllerWithMock(t *testing.T) (*controllers.CompanyAuthController, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	svc := companyauth.NewCompanyUserService(
		db,
		repositories.NewCompanyUserRepository(db),
		repositories.NewCompanyUserRefreshTokenRepository(db),
		nil,
		"test-company-secret",
	)
	return controllers.NewCompanyAuthController(svc), mock
}

func postJSON(t *testing.T, path, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// TestCompanyPasswordReset_DoesNotRevealAccountExistence は、
// 登録済みメールと未登録メールでレスポンスが完全に一致することを検証する。
// ここが崩れると、リセット要求だけでアカウントの存在を総当たりできる。
func TestCompanyPasswordReset_DoesNotRevealAccountExistence(t *testing.T) {
	// 未登録のメール
	ctrlUnknown, mockUnknown := newCompanyAuthControllerWithMock(t)
	mockUnknown.ExpectQuery("SELECT \\* FROM `company_users`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	ctxUnknown, recUnknown := postJSON(t, "/api/company-auth/forgot-password", `{"email":"nobody@example.com"}`)
	require.NoError(t, ctrlUnknown.ForgotPassword(ctxUnknown))

	// 登録済みのメール（UPDATE でトークンが保存される）
	ctrlKnown, mockKnown := newCompanyAuthControllerWithMock(t)
	mockKnown.ExpectQuery("SELECT \\* FROM `company_users`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password"}).
			AddRow(1, 10, "hr@example.com", "hashed"))
	mockKnown.ExpectBegin()
	mockKnown.ExpectExec("UPDATE `company_users`").WillReturnResult(sqlmock.NewResult(0, 1))
	mockKnown.ExpectCommit()
	ctxKnown, recKnown := postJSON(t, "/api/company-auth/forgot-password", `{"email":"hr@example.com"}`)
	require.NoError(t, ctrlKnown.ForgotPassword(ctxKnown))

	assert.Equal(t, http.StatusOK, recUnknown.Code)
	assert.Equal(t, http.StatusOK, recKnown.Code)
	assert.Equal(t, recUnknown.Body.String(), recKnown.Body.String(),
		"登録済み/未登録でレスポンスが異なるとアカウントの存在を推測できてしまう")
}

// DBエラーが起きても 200 を返す。エラーの有無から存在を推測させないため。
func TestCompanyPasswordReset_DBErrorStillReturnsOK(t *testing.T) {
	ctrl, mock := newCompanyAuthControllerWithMock(t)
	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WillReturnError(assertErr{})

	ctx, rec := postJSON(t, "/api/company-auth/forgot-password", `{"email":"hr@example.com"}`)
	require.NoError(t, ctrl.ForgotPassword(ctx))
	assert.Equal(t, http.StatusOK, rec.Code)
}

type assertErr struct{}

func (assertErr) Error() string { return "db down" }

func TestCompanyPasswordReset_InvalidBody(t *testing.T) {
	ctrl, _ := newCompanyAuthControllerWithMock(t)
	ctx, _ := postJSON(t, "/api/company-auth/forgot-password", `{`)
	assert.Error(t, ctrl.ForgotPassword(ctx))
}

func TestCompanyPasswordResetExecute_InvalidBody(t *testing.T) {
	ctrl, _ := newCompanyAuthControllerWithMock(t)
	ctx, _ := postJSON(t, "/api/company-auth/reset-password", `{`)
	assert.Error(t, ctrl.ResetPassword(ctx))
}

// 短いパスワードは 400 で弾く。
func TestCompanyPasswordResetExecute_ShortPassword(t *testing.T) {
	ctrl, _ := newCompanyAuthControllerWithMock(t)
	ctx, rec := postJSON(t, "/api/company-auth/reset-password", `{"token":"tok","password":"short"}`)
	err := ctrl.ResetPassword(ctx)
	require.Error(t, err)
	he, ok := err.(*echo.HTTPError)
	require.True(t, ok, "echo.HTTPError であること")
	assert.Equal(t, http.StatusBadRequest, he.Code)
	_ = rec
}
