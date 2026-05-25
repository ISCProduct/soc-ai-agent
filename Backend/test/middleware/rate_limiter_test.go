package middleware_test

// レート制限ミドルウェアのテスト（Issue #325）
// 実行: cd Backend && go test ./test/middleware/... -run TestRateLimit -v

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/internal/middleware"
)

// TestRateLimiter_AllowsUnderLimit はウィンドウ内の上限以下のリクエストが全て通ることを検証する
func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(time.Minute, 5)
	for i := range 5 {
		if !rl.Allow("key1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

// TestRateLimiter_BlocksOverLimit はウィンドウ内の上限を超えたリクエストがブロックされることを検証する（#325修正の担保）
func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(time.Minute, 3)
	for range 3 {
		rl.Allow("key2")
	}
	if rl.Allow("key2") {
		t.Error("4回目のリクエストはブロックされるべき")
	}
}

// TestRateLimiter_DifferentKeysAreIndependent は異なるキーが独立してカウントされることを検証する
func TestRateLimiter_DifferentKeysAreIndependent(t *testing.T) {
	rl := middleware.NewRateLimiter(time.Minute, 2)
	rl.Allow("user-a")
	rl.Allow("user-a")
	if rl.Allow("user-a") {
		t.Error("user-a の3回目はブロックされるべき")
	}
	if !rl.Allow("user-b") {
		t.Error("user-b の1回目は通るべき")
	}
}

// TestLoginRateLimit_Middleware はミドルウェアが 429 を返すことを検証する
func TestLoginRateLimit_Middleware(t *testing.T) {
	testLimiter := middleware.NewRateLimiter(time.Minute, 2)
	wrapped := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip := middleware.GetClientIP(r)
			if !testLimiter.Allow(ip) {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next(w, r)
		}
	}

	handler := wrapped(okHandler)
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		w := httptest.NewRecorder()
		handler(w, req)
		if i <= 2 {
			if w.Code != http.StatusOK {
				t.Errorf("request %d: want 200, got %d", i, w.Code)
			}
		} else {
			if w.Code != http.StatusTooManyRequests {
				t.Errorf("request %d: want 429, got %d", i, w.Code)
			}
		}
	}
}

// TestGetClientIP_XForwardedFor は X-Forwarded-For ヘッダーからIPを取得することを検証する
func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.RemoteAddr = "127.0.0.1:8080"
	ip := middleware.GetClientIP(req)
	if ip != "1.2.3.4" {
		t.Errorf("got %q, want %q", ip, "1.2.3.4")
	}
}

// TestGetClientIP_RemoteAddr は X-Forwarded-For がない場合に RemoteAddr を使用することを検証する
func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:9999"
	ip := middleware.GetClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("got %q, want %q", ip, "192.168.1.1")
	}
}
