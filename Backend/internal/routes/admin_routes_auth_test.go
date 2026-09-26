package routes_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/routes"

	"github.com/labstack/echo/v4"
)

// TestSetupAdminRoutes_AllRoutesRequireAuth は /api/admin 配下に認証なしのルートが
// 無いことを固定する(#1411)。
//
// 以前は /admin/company-graph だけがミドルウェア無しのグループで登録されており、
// 「/admin に生やせば守られる」という前提が崩れていた。実害のあるハンドラは
// 含まれていなかったが、次にそのグループへ足した人が無認証で公開してしまう。
//
// コントローラーは nil のまま登録する。ルート登録時にメソッド値を取るだけで
// デリファレンスは起きないため問題なく、逆に認証をすり抜けたルートがあれば
// ハンドラに到達して panic するので検出できる。
func TestSetupAdminRoutes_AllRoutesRequireAuth(t *testing.T) {
	e := echo.New()
	api := e.Group("/api")
	routes.SetupAdminRoutes(
		api,
		// コントローラー22個 + userRepo + schoolService
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil,
		"test-admin-secret",
	)

	var adminRoutes []*echo.Route
	for _, r := range e.Routes() {
		if strings.HasPrefix(r.Path, "/api/admin") {
			adminRoutes = append(adminRoutes, r)
		}
	}
	if len(adminRoutes) == 0 {
		t.Fatal("/api/admin 配下のルートが 1 本も登録されていない。登録関数のシグネチャ変更を疑うこと")
	}

	for _, r := range adminRoutes {
		t.Run(r.Method+" "+r.Path, func(t *testing.T) {
			// ハンドラへ到達した場合 nil コントローラーで panic する。
			// 認証が効いていれば 401 で止まるので、ここには来ない。
			defer func() {
				if v := recover(); v != nil {
					t.Fatalf("認証を通過してハンドラに到達した（%s %s は無認証で公開されている）", r.Method, r.Path)
				}
			}()

			req := httptest.NewRequest(r.Method, r.Path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("認証ヘッダー無しの応答 = %d, want %d（無認証で公開されている可能性）", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}
