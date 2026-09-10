package middleware_test

// リクエストロガーミドルウェアのテスト（Issue #403）
// 実行: cd Backend && go test ./test/middleware/... -run TestRequestLogger -v

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/middleware"

	"github.com/labstack/echo/v4"
)

// serveWithLogger はロガーミドルウェアを通してハンドラを実行し、出力されたログを返す。
func serveWithLogger(t *testing.T, method, path string, handler echo.HandlerFunc) string {
	t.Helper()

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// エラーはミドルウェアが素通しする。実際のレスポンス生成は
	// HTTPErrorHandler の役目なのでここでは検証しない。
	_ = middleware.EchoRequestLogger(handler)(c)

	return buf.String()
}

func TestRequestLogger(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		handler echo.HandlerFunc
		want    []string
	}{
		{
			name:    "正常系はmethod/path/statusを記録する",
			method:  http.MethodGet,
			path:    "/api/health",
			handler: func(c echo.Context) error { return c.NoContent(http.StatusOK) },
			want:    []string{"GET", "/api/health", "status=200", "INFO"},
		},
		{
			name:    "4xxはWARNレベルで記録する",
			method:  http.MethodGet,
			path:    "/not-found",
			handler: func(c echo.Context) error { return c.NoContent(http.StatusNotFound) },
			want:    []string{"status=404", "WARN"},
		},
		{
			name:    "5xxはERRORレベルで記録する",
			method:  http.MethodGet,
			path:    "/crash",
			handler: func(c echo.Context) error { return c.NoContent(http.StatusInternalServerError) },
			want:    []string{"status=500", "ERROR"},
		},
		{
			// レスポンスを書くのはミドルウェアより後に走る HTTPErrorHandler なので、
			// ResponseWriter から status を取ると 200 のまま記録されてしまう。
			// 署名検証の 401 がログ上は成功に見え、調査で誤診した実績がある。
			name:    "HTTPErrorを返した場合はそのコードを記録する",
			method:  http.MethodPost,
			path:    "/api/discord/interactions",
			handler: func(c echo.Context) error { return echo.NewHTTPError(http.StatusUnauthorized, "invalid signature") },
			want:    []string{"status=401", "WARN"},
		},
		{
			name:    "HTTPError以外のエラーは500として記録する",
			method:  http.MethodGet,
			path:    "/boom",
			handler: func(c echo.Context) error { return errors.New("何かが壊れた") },
			want:    []string{"status=500", "ERROR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logged := serveWithLogger(t, tt.method, tt.path, tt.handler)
			for _, want := range tt.want {
				if !strings.Contains(logged, want) {
					t.Errorf("ログに %q が含まれていない: %s", want, logged)
				}
			}
		})
	}
}

// TestRequestLogger_IncludesRequestID はX-Request-IDがある場合にログに含まれることを検証する
func TestRequestLogger_IncludesRequestID(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	const testID = "trace-xyz-999"

	e := echo.New()
	// RequestIDMiddleware は http.Handler 版のままなので、本番と同じく WrapMiddleware で繋ぐ
	h := echo.WrapMiddleware(middleware.RequestIDMiddleware)(
		middleware.EchoRequestLogger(func(c echo.Context) error {
			return c.NoContent(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
	req.Header.Set(middleware.RequestIDHeader, testID)
	rec := httptest.NewRecorder()

	if err := h(e.NewContext(req, rec)); err != nil {
		t.Fatalf("ハンドラがエラーを返した: %v", err)
	}

	if !strings.Contains(buf.String(), testID) {
		t.Errorf("ログにrequest_id %q が含まれていない: %s", testID, buf.String())
	}
}

// TestRequestLogger_PropagatesError はミドルウェアがエラーを握りつぶさないことを検証する。
// 握りつぶすと HTTPErrorHandler が走らず、エラーが 200 で返る。
func TestRequestLogger_PropagatesError(t *testing.T) {
	slog.SetDefault(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	e := echo.New()
	want := echo.NewHTTPError(http.StatusForbidden, "forbidden")
	err := middleware.EchoRequestLogger(func(c echo.Context) error { return want })(
		e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder()),
	)

	if !errors.Is(err, want) {
		t.Errorf("ミドルウェアはエラーをそのまま返すべき: got %v, want %v", err, want)
	}
}
