package routes

// #1196 EchoCompanyAuth のテスト。
//
// このミドルウェアは、無効化された企業ユーザーが /api/company-portal/*
// （学生検索・タグ・学生分析）へ到達するのを止める唯一の防壁。
// ここが落ちるとポータル全体の認可が抜ける。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	companycontrollers "Backend/internal/controllers/company"
	"Backend/internal/middleware"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testCompanySecret = "test-company-secret"

func newCompanyUserRepoMock(t *testing.T) (*repositories.CompanyUserRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return repositories.NewCompanyUserRepository(db), mock
}

// runCompanyAuth はミドルウェアを1回通し、到達したかどうかとステータスを返す。
func runCompanyAuth(t *testing.T, repo *repositories.CompanyUserRepository, token string) (reached bool, status int) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/company-portal/students", nil)
	if token != "" {
		req.Header.Set("X-Company-User-Token", token)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := EchoCompanyAuth(testCompanySecret, repo)(func(c echo.Context) error {
		reached = true
		return c.NoContent(http.StatusOK)
	})
	if err := handler(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return reached, rec.Code
}

func validCompanyToken(t *testing.T) string {
	t.Helper()
	tok, err := middleware.GenerateJWT(1, "hr@example.com", testCompanySecret)
	require.NoError(t, err)
	return tok
}

// 無効化されたアカウントは、JWTが有効でもポータルへ到達できない。
func TestEchoCompanyAuth_RejectsDisabledUser(t *testing.T) {
	repo, mock := newCompanyUserRepoMock(t)
	disabled := time.Now().Add(-time.Hour)
	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password", "disabled_at"}).
			AddRow(1, 10, "hr@example.com", "hashed", disabled))

	reached, status := runCompanyAuth(t, repo, validCompanyToken(t))
	assert.False(t, reached, "無効化されたアカウントがポータルへ到達している")
	assert.Equal(t, http.StatusForbidden, status)
}

// 有効なアカウントは通る（上のテストが常にfalseになる作りでないことの担保）。
func TestEchoCompanyAuth_AllowsActiveUser(t *testing.T) {
	repo, mock := newCompanyUserRepoMock(t)
	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password"}).
			AddRow(1, 10, "hr@example.com", "hashed"))

	reached, status := runCompanyAuth(t, repo, validCompanyToken(t))
	assert.True(t, reached, "有効なアカウントが弾かれている")
	assert.Equal(t, http.StatusOK, status)
}

// 招待未受諾（パスワード未設定）は通さない。
func TestEchoCompanyAuth_RejectsInvitePending(t *testing.T) {
	repo, mock := newCompanyUserRepoMock(t)
	mock.ExpectQuery("SELECT \\* FROM `company_users`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password"}).
			AddRow(1, 10, "hr@example.com", ""))

	reached, status := runCompanyAuth(t, repo, validCompanyToken(t))
	assert.False(t, reached)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestEchoCompanyAuth_RejectsMissingToken(t *testing.T) {
	repo, _ := newCompanyUserRepoMock(t)
	reached, status := runCompanyAuth(t, repo, "")
	assert.False(t, reached)
	assert.Equal(t, http.StatusUnauthorized, status)
}

// 別シークレットで署名されたトークンは通さない（学生用トークンの流用を防ぐ）。
func TestEchoCompanyAuth_RejectsTokenSignedWithOtherSecret(t *testing.T) {
	repo, _ := newCompanyUserRepoMock(t)
	tok, err := middleware.GenerateJWT(1, "hr@example.com", "student-secret")
	require.NoError(t, err)

	reached, status := runCompanyAuth(t, repo, tok)
	assert.False(t, reached, "学生用シークレットで署名したトークンが通っている")
	assert.Equal(t, http.StatusUnauthorized, status)
}

// シークレット未設定なら fail-close（503）。誤って全開放しない。
func TestEchoCompanyAuth_FailsClosedWithoutSecret(t *testing.T) {
	repo, _ := newCompanyUserRepoMock(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/company-portal/students", nil)
	req.Header.Set("X-Company-User-Token", "whatever")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	reached := false
	handler := EchoCompanyAuth("", repo)(func(c echo.Context) error {
		reached = true
		return c.NoContent(http.StatusOK)
	})
	if err := handler(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	assert.False(t, reached)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func newCompanyRepoMock(t *testing.T) (*repositories.CompanyRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return repositories.NewCompanyRepository(db), mock
}

func runVerifiedCompany(t *testing.T, repo *repositories.CompanyRepository, companyID uint) (reached bool, status int) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/company-portal/students", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.CompanyIDContextKey, companyID))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := EchoRequireVerifiedCompany(repo)(func(c echo.Context) error {
		reached = true
		return c.NoContent(http.StatusOK)
	})
	if err := handler(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return reached, rec.Code
}

func TestEchoRequireVerifiedCompany_RejectsUnverified(t *testing.T) {
	repo, mock := newCompanyRepoMock(t)
	mock.ExpectQuery("SELECT \\* FROM `companies`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_verified"}).AddRow(10, false))

	reached, status := runVerifiedCompany(t, repo, 10)
	assert.False(t, reached, "未審査の企業が学生APIへ到達している")
	assert.Equal(t, http.StatusForbidden, status)
}

func TestEchoRequireVerifiedCompany_AllowsVerified(t *testing.T) {
	repo, mock := newCompanyRepoMock(t)
	mock.ExpectQuery("SELECT \\* FROM `companies`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_verified"}).AddRow(10, true))

	reached, status := runVerifiedCompany(t, repo, 10)
	assert.True(t, reached, "審査済みの企業が弾かれている")
	assert.Equal(t, http.StatusOK, status)
}

func TestEchoRequireVerifiedCompany_RejectsMissingCompany(t *testing.T) {
	repo, mock := newCompanyRepoMock(t)
	mock.ExpectQuery("SELECT \\* FROM `companies`").WillReturnError(gorm.ErrRecordNotFound)

	reached, status := runVerifiedCompany(t, repo, 10)
	assert.False(t, reached)
	assert.Equal(t, http.StatusForbidden, status)
}

// ルート登録そのものに審査チェックが付いていることを確認する。
// ミドルウェア単体が正しくても、学生・スカウトの経路から外れていれば意味がない。
func TestUnverifiedCompanyCannotReachStudentOrScoutAPIs(t *testing.T) {
	users, userMock := newCompanyUserRepoMock(t)
	companies, companyMock := newCompanyRepoMock(t)
	e := echo.New()
	api := e.Group("/api")
	SetupCompanyAuthRoutes(
		api,
		&companycontrollers.CompanyAuthController{},
		&companycontrollers.CompanyPortalController{},
		&companycontrollers.CompanyStudentController{},
		nil, nil, nil, nil,
		&companycontrollers.CompanyPortalScoutController{},
		testCompanySecret,
		users,
		companies,
	)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/company-portal/students"},
		{http.MethodPost, "/api/company-portal/students/semantic-search"},
		{http.MethodGet, "/api/company-portal/students/5"},
		{http.MethodPost, "/api/company-portal/scouts"},
		{http.MethodGet, "/api/company-portal/scout-templates"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			userMock.ExpectQuery("SELECT \\* FROM `company_users`").
				WillReturnRows(sqlmock.NewRows([]string{"id", "company_id", "email", "password"}).
					AddRow(1, 10, "hr@example.com", "hashed"))
			companyMock.ExpectQuery("SELECT \\* FROM `companies`").
				WillReturnRows(sqlmock.NewRows([]string{"id", "is_verified"}).AddRow(10, false))

			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Company-User-Token", validCompanyToken(t))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusForbidden, rec.Code, tc.path)
			assert.Contains(t, rec.Body.String(), "審査が完了するまで")
		})
	}
}
