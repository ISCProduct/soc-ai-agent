package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/routes"

	"github.com/labstack/echo/v4"
)

// サービス間シークレット認証(X-Admin-Secret)の境界を固定する。
// ここで使う値は SERVICE_SECRET であり、ADMIN_SECRET(管理者トークンのHMAC鍵)とは別物(#1158)。
// 未設定でADMIN_SECRETへフォールバックするような実装に戻ると、分離した意味が無くなる。
func TestEchoStaticSecretAuth(t *testing.T) {
	tests := []struct {
		name          string
		serviceSecret string
		header        string
		wantStatus    int
	}{
		{"未設定ならfail-close", "", "any-value", http.StatusServiceUnavailable},
		{"未設定かつヘッダ無しでもfail-close", "", "", http.StatusServiceUnavailable},
		{"ヘッダ無しは拒否", "service-value", "", http.StatusForbidden},
		{"値が違えば拒否", "service-value", "other-value", http.StatusForbidden},
		{"一致すれば通す", "service-value", "service-value", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/admin/training/stats", nil)
			if tt.header != "" {
				req.Header.Set("X-Admin-Secret", tt.header)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := routes.EchoStaticSecretAuth(tt.serviceSecret)(func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			})
			if err := handler(c); err != nil {
				e.HTTPErrorHandler(err, c)
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
