package auth

import (
	"errors"
	"net/http"
	"testing"
)

// POST /api/auth/resend-verification は、結果にかかわらず同じ応答を返す。
// 応答の違いから、アカウントの有無・認証済みか否か・内部の失敗を推測させない。

func TestResendVerification_ResponseIsIdenticalRegardlessOfOutcome(t *testing.T) {
	outcomes := []struct {
		name string
		err  error
	}{
		{"送った(または何もしなかった)", nil},
		{"サーバー側の失敗(DB 保存失敗など)", errors.New("failed to save verification token: db down")},
	}

	var wantStatus int
	var wantBody string
	for i, tt := range outcomes {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			svc := &mockAuthService{resendVerificationFn: func(email string) error {
				got = email
				return tt.err
			}}
			e, ctrl := newAuthTestServer(svc)

			rec := servePost(e, "/api/auth/resend-verification", `{"email":"user@example.com"}`, ctrl.ResendVerification)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
			}
			if got != "user@example.com" {
				t.Fatalf("サービスに渡したメールアドレス = %q", got)
			}
			if i == 0 {
				wantStatus, wantBody = rec.Code, rec.Body.String()
				return
			}
			if rec.Code != wantStatus || rec.Body.String() != wantBody {
				t.Fatalf("結果によって応答が変わってはいけない: 1件目=%d %s / 今回=%d %s", wantStatus, wantBody, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestResendVerification_ResponseBodyDoesNotEchoEmail(t *testing.T) {
	svc := &mockAuthService{resendVerificationFn: func(string) error { return nil }}
	e, ctrl := newAuthTestServer(svc)

	rec := servePost(e, "/api/auth/resend-verification", `{"email":"secret-person@example.com"}`, ctrl.ResendVerification)

	if body := rec.Body.String(); len(body) == 0 || containsAny(body, "secret-person", "@example.com") {
		t.Fatalf("応答にメールアドレスを含めない: %s", body)
	}
}

func TestResendVerification_InvalidBodyIsBadRequest(t *testing.T) {
	called := false
	svc := &mockAuthService{resendVerificationFn: func(string) error { called = true; return nil }}
	e, ctrl := newAuthTestServer(svc)

	rec := servePost(e, "/api/auth/resend-verification", `{not json`, ctrl.ResendVerification)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if called {
		t.Fatal("壊れたボディではサービスを呼ばない")
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
