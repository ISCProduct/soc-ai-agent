package middleware_test

// UserAuthFunc HTTPミドルウェアのセキュリティテスト
// 実行: cd Backend && go test ./internal/middleware/... -run TestUserAuth -v

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
)

// TestUserAuth_SecretNotConfigured は userSecret が空のとき 503 を返すことを検証する（フェイルクローズ）
func TestUserAuth_SecretNotConfigured(t *testing.T) {
	h := middleware.UserAuthFunc("", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Token", "sometoken")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("シークレット未設定: got %d, want 503", rr.Code)
	}
}

// TestUserAuth_MissingToken はトークンヘッダーが欠けているとき 401 を返すことを検証する
func TestUserAuth_MissingToken(t *testing.T) {
	h := middleware.UserAuthFunc("test-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("トークン欠落: got %d, want 401", rr.Code)
	}
}

// TestUserAuth_InvalidToken は無効なJWTトークンのとき 401 を返すことを検証する
func TestUserAuth_InvalidToken(t *testing.T) {
	h := middleware.UserAuthFunc("test-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Token", "not-a-valid-jwt")
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("不正トークン: got %d, want 401", rr.Code)
	}
}

// TestUserAuth_WrongSecret は異なるシークレットで署名されたJWTを拒否することを検証する
func TestUserAuth_WrongSecret(t *testing.T) {
	token := middleware.GenerateUserToken(1, "user@example.com", "correct-secret")
	h := middleware.UserAuthFunc("wrong-secret", okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Token", token)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("別シークレット: got %d, want 401", rr.Code)
	}
}

// TestUserAuth_ValidToken は正しいJWTで次のハンドラが呼ばれることを検証する
func TestUserAuth_ValidToken(t *testing.T) {
	const (
		secret = "correct-secret"
		userID = uint(1)
		email  = "user@example.com"
	)
	validToken := middleware.GenerateUserToken(userID, email, secret)
	h := middleware.UserAuthFunc(secret, okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Token", validToken)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("正規トークン: got %d, want 200", rr.Code)
	}
}

// TestUserAuth_ContextContainsUserID はJWT検証後コンテキストにユーザーIDが保存されることを検証する
func TestUserAuth_ContextContainsUserID(t *testing.T) {
	const (
		secret = "test-secret"
		userID = uint(42)
		email  = "user@example.com"
	)
	token := middleware.GenerateUserToken(userID, email, secret)

	var capturedID uint
	capturingHandler := func(w http.ResponseWriter, r *http.Request) {
		id, _ := r.Context().Value(middleware.UserIDContextKey).(uint)
		capturedID = id
		w.WriteHeader(http.StatusOK)
	}

	h := middleware.UserAuthFunc(secret, capturingHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Token", token)
	rr := httptest.NewRecorder()

	h(rr, req)

	if capturedID != userID {
		t.Errorf("コンテキストのユーザーID: got %d, want %d", capturedID, userID)
	}
}
