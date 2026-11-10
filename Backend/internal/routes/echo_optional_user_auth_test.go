package routes_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/routes"
	"Backend/internal/services/auth"

	"github.com/labstack/echo/v4"
)

// stubAccessGuard は退会済みかどうかだけを返す。
type stubAccessGuard struct{ err error }

func (s stubAccessGuard) EnsureActiveUser(uint) error { return s.err }

// optionalAuthEcho は EchoOptionalUserAuth を1本だけ通す echo を組む。
// ハンドラは「実行者が載ったか」をレスポンスに出す。
func optionalAuthEcho(secret string, access auth.UserAccessGuard, orgs ...routes.OrganizationIDResolver) *echo.Echo {
	e := echo.New()
	e.Use(routes.EchoOptionalUserAuth(secret, access, orgs...))
	e.GET("/ping", func(c echo.Context) error {
		userID, ok := middleware.UserIDFromContext(c.Request().Context())
		if !ok {
			return c.String(http.StatusOK, "anonymous")
		}
		orgID, _ := middleware.OrganizationIDFromContext(c.Request().Context())
		return c.String(http.StatusOK, "user="+itoa(userID)+" org="+itoa(orgID))
	})
	return e
}

func itoa(v uint) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}

// 任意認証は「401 を返さない」ことが仕様なので、退行が静かに通る。
// 素通りさせる条件と、実行者を載せる条件の両方をここで固定する（#1531）。
func TestEchoOptionalUserAuth(t *testing.T) {
	validToken := middleware.GenerateUserToken(42, "u@example.com", testUserSecret)
	otherKeyToken := middleware.GenerateUserToken(42, "u@example.com", "another-secret")

	tests := []struct {
		name     string
		secret   string
		token    string
		access   auth.UserAccessGuard
		resolver []routes.OrganizationIDResolver
		wantBody string
	}{
		{
			name: "トークン無しは素通り", secret: testUserSecret, token: "",
			resolver: []routes.OrganizationIDResolver{stubOrgResolver{orgID: 7}},
			wantBody: "anonymous",
		},
		{
			// 壊れたトークンで 401 にすると、期限切れだけで添削そのものが使えなくなる。
			name: "壊れたトークンは素通り", secret: testUserSecret, token: "not-a-jwt",
			resolver: []routes.OrganizationIDResolver{stubOrgResolver{orgID: 7}},
			wantBody: "anonymous",
		},
		{
			// 別の鍵で署名されたトークンを実行者として載せると、他人のIDで履歴が溜まる。
			name: "別鍵で署名されたトークンは載せない", secret: testUserSecret, token: otherKeyToken,
			resolver: []routes.OrganizationIDResolver{stubOrgResolver{orgID: 7}},
			wantBody: "anonymous",
		},
		{
			name: "secret 未設定は素通り", secret: "", token: validToken,
			resolver: []routes.OrganizationIDResolver{stubOrgResolver{orgID: 7}},
			wantBody: "anonymous",
		},
		{
			name: "退会済みは載せない", secret: testUserSecret, token: validToken,
			access:   stubAccessGuard{err: auth.ErrAccountWithdrawn},
			resolver: []routes.OrganizationIDResolver{stubOrgResolver{orgID: 7}},
			wantBody: "anonymous",
		},
		{
			// 組織が確定しない実行者を載せると、呼び出し側が所属不明のまま保存先を決める。
			name: "組織解決エラーは載せない", secret: testUserSecret, token: validToken,
			resolver: []routes.OrganizationIDResolver{stubOrgResolver{err: errors.New("db down")}},
			wantBody: "anonymous",
		},
		{
			name: "組織が0なら載せない", secret: testUserSecret, token: validToken,
			resolver: []routes.OrganizationIDResolver{stubOrgResolver{orgID: 0}},
			wantBody: "anonymous",
		},
		{
			name: "有効なトークンは実行者と組織を載せる", secret: testUserSecret, token: validToken,
			resolver: []routes.OrganizationIDResolver{stubOrgResolver{orgID: 7}},
			wantBody: "user=42 org=7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := optionalAuthEcho(tt.secret, tt.access, tt.resolver...)
			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			if tt.token != "" {
				req.Header.Set("X-User-Token", tt.token)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			// 任意認証はどの場合も 401 を返さない。
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200（任意認証は拒否しない）", rec.Code)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Fatalf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}
