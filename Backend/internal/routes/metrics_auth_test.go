package routes

// /metrics のアクセス制御テスト（Issue #1186）
// 実行: cd Backend && go test ./internal/routes/... -run TestEchoMetricsAuth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// TestEchoMetricsAuth は Bearer トークンが一致するときだけ通すことを検証する。
// backend の ALB はインターネット直結のため、素通しにするとエンドポイント一覧と
// レイテンシ分布が誰でも読める。
func TestEchoMetricsAuth(t *testing.T) {
	const token = "s3cret-token"

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"正しいBearerトークン", "Bearer " + token, http.StatusOK},
		{"ヘッダー無し", "", http.StatusUnauthorized},
		{"Bearerプレフィックス無し", token, http.StatusUnauthorized},
		{"別のトークン", "Bearer wrong-token", http.StatusUnauthorized},
		{"空のBearer", "Bearer ", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			h := EchoMetricsAuth(token)(func(c echo.Context) error {
				return c.String(http.StatusOK, "# metrics")
			})

			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()

			got := http.StatusOK
			if err := h(e.NewContext(req, rec)); err != nil {
				he, ok := err.(*echo.HTTPError)
				if !ok {
					t.Fatalf("予期しないエラー: %v", err)
				}
				got = he.Code
			} else {
				got = rec.Code
			}

			if got != tt.want {
				t.Errorf("ステータス=%d, 期待=%d", got, tt.want)
			}
		})
	}
}

// TestMetricsSkipper は計装対象から外すパスを固定する（#1186）。
// 特にルート未一致(c.Path()が空)を外さないと、スキャンのたびに url ラベルが増え続ける。
func TestMetricsSkipper(t *testing.T) {
	tests := []struct {
		name  string
		route string // Echo が解決したルート。未一致なら空文字
		want  bool
	}{
		{"通常のAPIルートは計装する", "/api/users/:id", false},
		{"ルート未一致(404)は除外", "", true},
		{"ヘルスチェックは除外", "/health", true},
		{"ヘルスチェック(k8s形式)は除外", "/healthz", true},
		{"metrics自身は除外", "/metrics", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/dummy", nil)
			c := e.NewContext(req, httptest.NewRecorder())
			c.SetPath(tt.route)

			if got := MetricsSkipper(c); got != tt.want {
				t.Errorf("MetricsSkipper(%q)=%v, 期待=%v", tt.route, got, tt.want)
			}
		})
	}
}
