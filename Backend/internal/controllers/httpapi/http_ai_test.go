package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"Backend/internal/companyfetch"
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
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "AI未設定は503",
			err:        openai.ErrAIUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantMsg:    "現在AI機能を利用できません。しばらくしてから再度お試しください。",
		},
		{
			name:       "ラップされていても503",
			err:        fmt.Errorf("generate failed: %w", openai.ErrAIUnavailable),
			wantStatus: http.StatusServiceUnavailable,
			wantMsg:    "現在AI機能を利用できません。しばらくしてから再度お試しください。",
		},
		{
			name:       "検索予算超過は429で英語の内部文を出さない",
			err:        fmt.Errorf("search: %w", companyfetch.ErrSearchBudgetExceeded),
			wantStatus: http.StatusTooManyRequests,
			wantMsg:    "検索の回数が多すぎます。少し時間を置いてからお試しください。",
		},
		{name: "その他のエラーは従来どおり500", err: errors.New("db error"), wantStatus: http.StatusInternalServerError, wantMsg: InternalServerErrorMessage},
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
			apiErr, ok := he.Message.(middleware.APIError)
			if !ok {
				t.Fatalf("message = %T", he.Message)
			}
			if apiErr.Msg != tt.wantMsg {
				t.Fatalf("msg = %q", apiErr.Msg)
			}
			if strings.Contains(apiErr.Msg, "budget exceeded") || strings.Contains(apiErr.Msg, "db error") {
				t.Fatalf("内部エラーを本文に出した: %q", apiErr.Msg)
			}
		})
	}
}
