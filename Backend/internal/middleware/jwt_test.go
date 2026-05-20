package middleware_test

// JWT生成・検証ユーティリティの単体テスト
// 実行: cd Backend && go test ./internal/middleware/... -run TestJWT -v

import (
	"testing"

	"Backend/internal/middleware"
)

// TestJWT_GenerateAndParse は生成したJWTを正しく検証できることを確認する
func TestJWT_GenerateAndParse(t *testing.T) {
	const (
		secret = "test-secret"
		userID = uint(10)
		email  = "user@example.com"
	)

	token, err := middleware.GenerateJWT(userID, email, secret)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateJWT が空文字列を返した")
	}

	gotID, gotEmail, err := middleware.ParseJWT(token, secret)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	if gotID != userID {
		t.Errorf("ユーザーID: got %d, want %d", gotID, userID)
	}
	if gotEmail != email {
		t.Errorf("メール: got %s, want %s", gotEmail, email)
	}
}

// TestJWT_ParseRejectsWrongSecret は別シークレットで署名されたJWTを拒否することを検証する
func TestJWT_ParseRejectsWrongSecret(t *testing.T) {
	token, _ := middleware.GenerateJWT(1, "user@example.com", "correct-secret")

	if _, _, err := middleware.ParseJWT(token, "wrong-secret"); err == nil {
		t.Error("別シークレットのJWTを受け入れてはならない")
	}
}

// TestJWT_ParseRejectsMalformed は不正形式のトークンを拒否することを検証する
func TestJWT_ParseRejectsMalformed(t *testing.T) {
	if _, _, err := middleware.ParseJWT("not.a.jwt", "secret"); err == nil {
		t.Error("不正形式のJWTを受け入れてはならない")
	}
}

// TestGenerateUserToken_NotEmpty は GenerateUserToken が空でないトークンを返すことを検証する
func TestGenerateUserToken_NotEmpty(t *testing.T) {
	tok := middleware.GenerateUserToken(1, "user@example.com", "secret")
	if tok == "" {
		t.Error("GenerateUserToken が空文字列を返した")
	}
}

// TestGenerateUserToken_DifferentFromAdminToken はユーザートークンと管理者トークンが異なることを検証する（トークン流用防止）
func TestGenerateUserToken_DifferentFromAdminToken(t *testing.T) {
	userToken := middleware.GenerateUserToken(1, "user@example.com", "secret")
	adminToken := middleware.GenerateAdminToken(1, "user@example.com", "secret")
	if userToken == adminToken {
		t.Error("ユーザートークンと管理者トークンが同一になってはならない")
	}
}

// TestVerifyUserToken_Valid は正しいトークンを受け入れることを検証する
func TestVerifyUserToken_Valid(t *testing.T) {
	const (
		secret = "secret"
		userID = uint(5)
		email  = "user@example.com"
	)
	token := middleware.GenerateUserToken(userID, email, secret)
	if !middleware.VerifyUserToken(token, userID, email, secret) {
		t.Error("正規トークンを拒否してはならない")
	}
}

// TestVerifyUserToken_WrongUserID は別ユーザーIDのトークンを拒否することを検証する
func TestVerifyUserToken_WrongUserID(t *testing.T) {
	token := middleware.GenerateUserToken(1, "user@example.com", "secret")
	if middleware.VerifyUserToken(token, 2, "user@example.com", "secret") {
		t.Error("別ユーザーIDのトークンを受け入れてはならない")
	}
}

// TestVerifyUserToken_WrongSecret は別シークレットのトークンを拒否することを検証する
func TestVerifyUserToken_WrongSecret(t *testing.T) {
	token := middleware.GenerateUserToken(1, "user@example.com", "correct-secret")
	if middleware.VerifyUserToken(token, 1, "user@example.com", "wrong-secret") {
		t.Error("別シークレットのトークンを受け入れてはならない")
	}
}
