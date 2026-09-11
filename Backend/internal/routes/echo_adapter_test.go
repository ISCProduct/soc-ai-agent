package routes_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	orgID    uint
	err      error
	isAdmin  bool
	adminErr error
}

func (s stubOrgResolver) ResolveOrganizationID(uint) (uint, error) { return s.orgID, s.err }
func (s stubOrgResolver) IsUserAdmin(uint) (bool, error)           { return s.isAdmin, s.adminErr }

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

// TestEchoUserAuth_TenantHandling は EchoUserAuth のテナント一致/不一致まわりの
// 分岐(拒否/許可/管理者バイパス/管理者判定エラー)をテーブル駆動でまとめて検証する。
// 管理者バイパスのケースでは、リクエストコンテキストの組織IDがアクセス先の学園
// (tenantOrgID)に差し替わっていることも確認する(でないと管理者が自分の所属組織の
// データを見てしまう)。
func TestEchoUserAuth_TenantHandling(t *testing.T) {
	cases := []struct {
		name        string
		resolver    stubOrgResolver
		tenantOrgID uint
		wantStatus  int
		wantOrgID   uint // wantStatus==200のときだけ検証
	}{
		{
			name:        "テナント一致は許可",
			resolver:    stubOrgResolver{orgID: 1},
			tenantOrgID: 1,
			wantStatus:  http.StatusOK,
			wantOrgID:   1,
		},
		{
			name:        "非管理者のテナント不一致は403",
			resolver:    stubOrgResolver{orgID: 1},
			tenantOrgID: 2,
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "管理者はテナント不一致でも許可され、組織IDがアクセス先学園に差し替わる",
			resolver:    stubOrgResolver{orgID: 1, isAdmin: true},
			tenantOrgID: 2,
			wantStatus:  http.StatusOK,
			wantOrgID:   2,
		},
		{
			name:        "管理者判定に失敗した場合は500",
			resolver:    stubOrgResolver{orgID: 1, adminErr: errors.New("db error")},
			tenantOrgID: 2,
			wantStatus:  http.StatusInternalServerError,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			token, err := middleware.GenerateJWT(1, "user@example.com", testUserSecret)
			if err != nil {
				t.Fatalf("generate jwt: %v", err)
			}

			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				// EchoTenantResolverが既に解決済みという想定でテナントIDをcontextへ載せておく
				return func(c echo.Context) error {
					ctx := context.WithValue(c.Request().Context(), middleware.TenantOrganizationIDContextKey, tt.tenantOrgID)
					c.SetRequest(c.Request().WithContext(ctx))
					return next(c)
				}
			})
			e.GET("/me", func(c echo.Context) error {
				orgID, _ := middleware.OrganizationIDFromContext(c.Request().Context())
				return c.String(http.StatusOK, strconv.FormatUint(uint64(orgID), 10))
			}, routes.EchoUserAuth(testUserSecret, nil, tt.resolver))

			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			req.Header.Set("X-User-Token", token)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				got := rec.Body.String()
				want := strconv.FormatUint(uint64(tt.wantOrgID), 10)
				if got != want {
					t.Fatalf("organization id in context = %q, want %q", got, want)
				}
			}
		})
	}
}
