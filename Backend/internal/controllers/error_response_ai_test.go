package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

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
	}{
		{name: "AI未設定は503", err: openai.ErrAIUnavailable, wantStatus: http.StatusServiceUnavailable},
		{
			name:       "ラップされていても503",
			err:        fmt.Errorf("generate failed: %w", openai.ErrAIUnavailable),
			wantStatus: http.StatusServiceUnavailable,
		},
		{name: "その他のエラーは従来どおり500", err: errors.New("db error"), wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := echoInternalError(tt.err)
			var he *echo.HTTPError
			if !errors.As(err, &he) {
				t.Fatalf("echo.HTTPError ではない: %T", err)
			}
			if he.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", he.Code, tt.wantStatus)
			}
		})
	}
}
