package routes_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/repositories"
	"Backend/internal/routes"
	"Backend/internal/services/organization"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testUserSecret = "test-secret"

// stubOrgResolver は EchoUserAuth の OrganizationIDResolver を固定値で返す。
type stubOrgResolver struct {
	orgID   uint
	err     error
	isAdmin bool
}

func (s stubOrgResolver) ResolveOrganizationID(uint) (uint, error) { return s.orgID, s.err }
func (s stubOrgResolver) IsUserAdmin(uint) (bool, error)           { return s.isAdmin, nil }

func newRoutesTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestEchoTenantResolver_UnknownSlugRejected(t *testing.T) {
	db, mock := newRoutesTestDB(t)
	orgs := organization.NewOrganizationService(repositories.NewOrganizationRepository(db))

	mock.ExpectQuery("SELECT \\* FROM `organizations` WHERE slug = \\?").
		WithArgs("no-such-univ", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "status"}))

	e := echo.New()
	e.Use(routes.EchoTenantResolver(orgs))
	e.GET("/ping", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Tenant-Slug", "no-such-univ")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestEchoTenantResolver_NoHeaderPassesThrough(t *testing.T) {
	db, _ := newRoutesTestDB(t)
	orgs := organization.NewOrganizationService(repositories.NewOrganizationRepository(db))

	e := echo.New()
	e.Use(routes.EchoTenantResolver(orgs))
	e.GET("/ping", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestEchoUserAuth_TenantMismatchRejected(t *testing.T) {
	token, err := middleware.GenerateJWT(1, "user@example.com", testUserSecret)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		// EchoTenantResolverが既に解決済みという想定でテナントIDをcontextへ載せておく
		return func(c echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), middleware.TenantOrganizationIDContextKey, uint(2))
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	e.GET("/me", func(c echo.Context) error { return c.String(http.StatusOK, "ok") },
		routes.EchoUserAuth(testUserSecret, nil, stubOrgResolver{orgID: 1}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("X-User-Token", token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestEchoUserAuth_TenantMatchAllowed(t *testing.T) {
	token, err := middleware.GenerateJWT(1, "user@example.com", testUserSecret)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), middleware.TenantOrganizationIDContextKey, uint(1))
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	e.GET("/me", func(c echo.Context) error { return c.String(http.StatusOK, "ok") },
		routes.EchoUserAuth(testUserSecret, nil, stubOrgResolver{orgID: 1}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("X-User-Token", token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// プラットフォーム管理者(is_admin)は自身の所属組織と異なる学園サブドメインへの
// アクセスでもtenant mismatchで弾かれない。
func TestEchoUserAuth_TenantMismatchAllowedForAdmin(t *testing.T) {
	token, err := middleware.GenerateJWT(1, "admin@example.com", testUserSecret)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), middleware.TenantOrganizationIDContextKey, uint(2))
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	e.GET("/me", func(c echo.Context) error { return c.String(http.StatusOK, "ok") },
		routes.EchoUserAuth(testUserSecret, nil, stubOrgResolver{orgID: 1, isAdmin: true}))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("X-User-Token", token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
