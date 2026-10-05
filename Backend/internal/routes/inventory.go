package routes

import (
	"cmp"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
)

// Route 登録済みエンドポイント1件。
type Route struct {
	Method string
	Path   string
}

// mainGoRoutes cmd/server/main.go で直接登録しているルート。
//
// 本来は Setup*Routes 側に寄せたいが、これらは main.go のローカル変数として
// 組み立てたコントローラに依存しており、移すと引数だけが増える関数が1つ増える。
// Inventory() は routes パッケージ内の登録しか見られないため、ここに明示して持つ。
// 二重管理になるので、main.go 側の直接登録が増えていないことを
// TestMainGoDirectRoutesAreListed で固定している。
var mainGoRoutes = []Route{
	// /api の外。監視用。
	{Method: "GET", Path: "/health"},
	{Method: "GET", Path: "/healthz"},
	{Method: "GET", Path: "/metrics"},

	{Method: "POST", Path: "/api/company-entry"},
	{Method: "GET", Path: "/api/whats-new"},

	{Method: "POST", Path: "/api/admin/company-entry-submissions/:id/resend-email"},
	{Method: "POST", Path: "/api/admin/companies/:id/company-users"},
	{Method: "GET", Path: "/api/admin/companies/:id/company-users"},
	{Method: "PATCH", Path: "/api/admin/companies/:id/company-users/:userID"},

	{Method: "GET", Path: "/api/admin/training/stats"},
	{Method: "GET", Path: "/api/admin/training/export"},
	{Method: "POST", Path: "/api/admin/whats-new/ingest"},
}

// Inventory 実装に登録されている全エンドポイントを返す。
//
// コントローラは nil で渡している。経路登録はハンドラを値として受け取るだけで
// 中身を呼ばないため、DB もサービスも要らない。認証ミドルウェアも
// 生成時には引数を参照せず、リクエスト時にだけ参照する（echo_adapter.go）。
//
// これで DB 無しに実データが取れるので、CI で openapi.yaml との差分を検出できる。
// 新しいエンドポイントを追加して spec に書き忘れたら、テストが落ちる。
func Inventory() []Route {
	e := echo.New()
	api := e.Group("/api")

	SetupAuthRoutes(api, nil, nil, "", nil, nil)
	SetupChatRoutes(api, nil, nil, "", nil, nil)
	SetupCompanyRoutes(api, nil)
	SetupAdminRoutes(api,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "")
	SetupResumeRoutes(api, nil, "", nil, nil)
	SetupInterviewRoutes(api, nil, nil, "", nil, nil)
	SetupGitHubRoutes(api, nil, "", nil, nil)
	SetupESRoutes(api, nil, nil)
	SetupScheduleRoutes(api, nil, "", nil, nil)
	SetupGoogleCalendarRoutes(api, nil, "", nil, nil)
	SetupApplicationRoutes(api, nil, nil, "", nil, nil)
	SetupCompanyAuthRoutes(api, nil, nil, nil, nil, nil, nil, nil, "", nil)
	SetupUserRoutes(api, nil, nil, nil, nil, "", nil, nil)
	SetupCollectiveInsightRoutes(api, nil, "", nil, nil)

	out := make([]Route, 0, len(e.Routes())+len(mainGoRoutes))
	for _, r := range e.Routes() {
		// echo が Group ごとに作る疑似ルート（echo_route_not_found / echo_method_not_allowed）
		// は実エンドポイントではないので外す。
		if strings.HasPrefix(r.Method, "echo_") {
			continue
		}
		out = append(out, Route{Method: r.Method, Path: r.Path})
	}
	out = append(out, mainGoRoutes...)

	slices.SortFunc(out, func(a, b Route) int {
		if c := cmp.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return cmp.Compare(a.Method, b.Method)
	})
	return slices.Compact(out)
}
