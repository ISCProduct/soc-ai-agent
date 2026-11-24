package auth

import (
	"net/http"
	"testing"

	"Backend/internal/middleware"
)

// keyLimiter は許可するかどうかを固定し、渡されたキーを記録する制限器。
type keyLimiter struct {
	allow bool
	keys  []string
}

func (l *keyLimiter) Allow(key string) bool { l.keys = append(l.keys, key); return l.allow }

// 登録申請は、メールアドレスの形式でない値を 400 で断り、宛先ごとの上限を超えると 429 で止める。
// どちらの場合も、確認メールの送信(サービス呼び出し)に進まない。
func TestRequestRegistration_EmailFormatAndPerEmailLimit(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		allow      bool
		wantStatus int
		wantCalled bool
		wantKey    string
	}{
		{name: "正しい形式で上限内なら送る", body: `{"email":"New.User@example.com"}`, allow: true, wantStatus: http.StatusOK, wantCalled: true, wantKey: "new.user@example.com"},
		{name: "宛先の上限を超えたら429", body: `{"email":"user@example.com"}`, allow: false, wantStatus: http.StatusTooManyRequests, wantKey: "user@example.com"},
		{name: "@が無い値は400", body: `{"email":"not-an-email"}`, allow: true, wantStatus: http.StatusBadRequest},
		{name: "表示名つきは400", body: `{"email":"Name <user@example.com>"}`, allow: true, wantStatus: http.StatusBadRequest},
		{name: "複数指定は400", body: `{"email":"a@example.com,b@example.com"}`, allow: true, wantStatus: http.StatusBadRequest},
		{name: "空白のみは400", body: `{"email":"   "}`, allow: true, wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := middleware.RegistrationRequestEmailRateLimiter
			limiter := &keyLimiter{allow: tt.allow}
			middleware.RegistrationRequestEmailRateLimiter = limiter
			t.Cleanup(func() { middleware.RegistrationRequestEmailRateLimiter = orig })

			called := false
			svc := &mockAuthService{requestRegistrationFn: func(string) error { called = true; return nil }}
			e, ctrl := newAuthTestServer(svc)
			rec := servePost(e, "/registration", tt.body, ctrl.RequestRegistration)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if called != tt.wantCalled {
				t.Errorf("サービス呼び出し = %v, want %v", called, tt.wantCalled)
			}
			if tt.wantKey == "" {
				if len(limiter.keys) != 0 {
					t.Errorf("形式が不正なら上限を消費しない: %v", limiter.keys)
				}
			} else if len(limiter.keys) != 1 || limiter.keys[0] != tt.wantKey {
				t.Errorf("宛先(小文字)をキーに制限する: %v", limiter.keys)
			}
		})
	}
}
