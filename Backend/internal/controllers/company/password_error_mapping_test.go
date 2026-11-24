package company

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

// パスワードの長さの検証エラーは理由つきの 400 にする (SOCAIAGENT-693)。
func TestMapCompanyAuthError_PasswordLength(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{name: "短すぎる", err: shared.ErrPasswordTooShort, wantMsg: "8文字以上"},
		{name: "長すぎる", err: shared.ErrPasswordTooLong, wantMsg: "72バイト"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var he *echo.HTTPError
			if !errors.As(mapCompanyAuthError(tt.err), &he) {
				t.Fatalf("HTTPError でない")
			}
			apiErr, _ := he.Message.(middleware.APIError)
			if he.Code != http.StatusBadRequest || !strings.Contains(apiErr.Msg, tt.wantMsg) {
				t.Fatalf("status = %d message = %q", he.Code, apiErr.Msg)
			}
		})
	}
}
