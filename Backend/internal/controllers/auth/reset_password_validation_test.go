package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"Backend/internal/services/auth"
	"Backend/internal/services/shared"
)

// 再設定の失敗は理由が分かる日本語で返し、内部のエラー文（bcrypt・DB）は返さない (SOCAIAGENT-693)。
func TestResetPassword_ErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{name: "短すぎる", err: shared.ErrPasswordTooShort, wantStatus: http.StatusBadRequest, wantMsg: "8文字以上"},
		{name: "長すぎる", err: shared.ErrPasswordTooLong, wantStatus: http.StatusBadRequest, wantMsg: "72バイト"},
		{name: "無効なリンク", err: auth.ErrPasswordResetTokenInvalid, wantStatus: http.StatusBadRequest, wantMsg: "リンク"},
		{name: "内部エラー", err: fmt.Errorf("failed to hash password: %w", errors.New("bcrypt: password length exceeds 72 bytes")), wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockAuthService{resetPasswordFn: func(_, _ string) error { return tt.err }}
			e, ctrl := newAuthTestServer(svc)
			rec := servePost(e, "/reset", `{"token":"tok","password":"whatever-pass"}`, ctrl.ResetPassword)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if body := rec.Body.String(); strings.Contains(body, "bcrypt") || strings.Contains(body, "failed to") {
				t.Errorf("内部のエラー文が出ている: %s", body)
			}
			if tt.wantMsg != "" && !strings.Contains(decodeAuthErrResp(t, rec).Error, tt.wantMsg) {
				t.Errorf("message に %q が無い: %s", tt.wantMsg, rec.Body.String())
			}
		})
	}
}

// 登録もパスワードの長さの理由を返す（本番でも "Registration failed" に丸めない）(SOCAIAGENT-693)。
func TestRegister_PasswordTooLong_ReturnsReason(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	svc := &mockAuthService{registerFn: func(auth.RegisterRequest, uint) (*auth.AuthResponse, error) { return nil, shared.ErrPasswordTooLong }}
	e, ctrl := newAuthTestServer(svc)
	body := `{"email":"new@example.com","password":"` + strings.Repeat("a", 73) + `","name":"合成 太郎"}`
	rec := servePost(e, "/register", body, ctrl.Register)
	if rec.Code != http.StatusBadRequest || !strings.Contains(decodeAuthErrResp(t, rec).Error, "72バイト") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}
