package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"Backend/internal/middleware"
	"Backend/internal/openai"

	"github.com/labstack/echo/v4"
)

// TestEchoInternalError_AIUnavailable は AI 未設定・ローカル障害が
// 503 + 明示メッセージになることを検証する（#1293）。
//
// 500「内部エラーが発生しました」だと、API 利用者が設定の問題なのか
// バグなのか切り分けられない。
func TestEchoInternalError_AIUnavailable(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{name: "AI未設定は503", err: openai.ErrAIUnavailable, wantStatus: http.StatusServiceUnavailable},
		{
			name:       "ラップされていても503",
			err:        fmt.Errorf("generate failed: %w", openai.ErrAIUnavailable),
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:        "LLM deadline exceeded は504で再試行可能な文面",
			err:         fmt.Errorf("generate failed: %w", context.DeadlineExceeded),
			wantStatus:  http.StatusGatewayTimeout,
			wantCode:    ErrCodeGatewayTimeout,
			wantMessage: "処理に時間がかかっています。時間をおいて再度お試しください。",
		},
		{name: "その他のエラーは従来どおり500", err: errors.New("db error"), wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InternalError(tt.err)
			var he *echo.HTTPError
			if !errors.As(err, &he) {
				t.Fatalf("echo.HTTPError ではない: %T", err)
			}
			if he.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", he.Code, tt.wantStatus)
			}
			if tt.wantCode != "" {
				resp, ok := he.Message.(middleware.APIError)
				if !ok {
					t.Fatalf("message type = %T, want API error", he.Message)
				}
				if got := resp.Code; got != tt.wantCode {
					t.Errorf("error code = %q, want %q", got, tt.wantCode)
				}
				if got := resp.Msg; got != tt.wantMessage {
					t.Errorf("error message = %q, want %q", got, tt.wantMessage)
				}
			}
		})
	}
}
