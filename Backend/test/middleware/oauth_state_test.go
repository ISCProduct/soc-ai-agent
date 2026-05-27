package middleware_test

// OAuth state 検証のテスト（Issue #324）
// 実行: cd Backend && go test ./test/middleware/... -run TestOAuthState -v

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/middleware"
)

const oauthStateCookieName = "oauth_state"

// TestGenerateOAuthState_SetsCookie は GenerateOAuthState が HttpOnly Cookie をセットすることを検証する（#324修正の担保）
func TestGenerateOAuthState_SetsCookie(t *testing.T) {
	w := httptest.NewRecorder()
	state, err := middleware.GenerateOAuthState(w)
	if err != nil {
		t.Fatalf("GenerateOAuthState returned error: %v", err)
	}
	if state == "" {
		t.Fatal("state が空")
	}

	resp := w.Result()
	cookies := resp.Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == oauthStateCookieName {
			stateCookie = c
			break
		}
	}
	if stateCookie == nil {
		t.Fatal("oauth_state Cookie がセットされていない")
	}
	if !stateCookie.HttpOnly {
		t.Error("Cookie は HttpOnly であるべき")
	}
	if stateCookie.SameSite != http.SameSiteLaxMode {
		t.Error("Cookie は SameSite=Lax であるべき")
	}
}

// TestVerifyOAuthState_ValidState は正しい state で検証が通ることを検証する（#324修正の担保）
func TestVerifyOAuthState_ValidState(t *testing.T) {
	loginW := httptest.NewRecorder()
	state, err := middleware.GenerateOAuthState(loginW)
	if err != nil {
		t.Fatalf("GenerateOAuthState error: %v", err)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/callback?state="+state, nil)
	for _, c := range loginW.Result().Cookies() {
		callbackReq.AddCookie(c)
	}
	callbackW := httptest.NewRecorder()

	if !middleware.VerifyOAuthState(callbackW, callbackReq) {
		t.Error("正しい state は検証を通るべき")
	}
}

// TestVerifyOAuthState_InvalidState は改ざんされた state で検証が失敗することを検証する（#324修正の担保）
func TestVerifyOAuthState_InvalidState(t *testing.T) {
	loginW := httptest.NewRecorder()
	if _, err := middleware.GenerateOAuthState(loginW); err != nil {
		t.Fatalf("GenerateOAuthState error: %v", err)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/callback?state=tampered-state-value", nil)
	for _, c := range loginW.Result().Cookies() {
		callbackReq.AddCookie(c)
	}
	callbackW := httptest.NewRecorder()

	if middleware.VerifyOAuthState(callbackW, callbackReq) {
		t.Error("改ざんされた state は検証を失敗させるべき")
	}
}

// TestVerifyOAuthState_MissingCookie は Cookie なしで検証が失敗することを検証する
func TestVerifyOAuthState_MissingCookie(t *testing.T) {
	callbackReq := httptest.NewRequest(http.MethodGet, "/callback?state=somestate", nil)
	callbackW := httptest.NewRecorder()

	if middleware.VerifyOAuthState(callbackW, callbackReq) {
		t.Error("Cookie がない場合は検証を失敗させるべき")
	}
}

// TestVerifyOAuthState_MissingStateParam は state パラメータなしで検証が失敗することを検証する
func TestVerifyOAuthState_MissingStateParam(t *testing.T) {
	loginW := httptest.NewRecorder()
	if _, err := middleware.GenerateOAuthState(loginW); err != nil {
		t.Fatalf("GenerateOAuthState error: %v", err)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/callback", nil)
	for _, c := range loginW.Result().Cookies() {
		callbackReq.AddCookie(c)
	}
	callbackW := httptest.NewRecorder()

	if middleware.VerifyOAuthState(callbackW, callbackReq) {
		t.Error("state パラメータがない場合は検証を失敗させるべき")
	}
}

// TestVerifyOAuthState_CookieDeletedAfterVerification は検証後に Cookie が削除されることを検証する（使い捨てトークン）
func TestVerifyOAuthState_CookieDeletedAfterVerification(t *testing.T) {
	loginW := httptest.NewRecorder()
	state, _ := middleware.GenerateOAuthState(loginW)

	callbackReq := httptest.NewRequest(http.MethodGet, "/callback?state="+state, nil)
	for _, c := range loginW.Result().Cookies() {
		callbackReq.AddCookie(c)
	}
	callbackW := httptest.NewRecorder()
	middleware.VerifyOAuthState(callbackW, callbackReq)

	for _, c := range callbackW.Result().Cookies() {
		if c.Name == oauthStateCookieName && c.MaxAge > 0 {
			t.Error("検証後に oauth_state Cookie が削除されていない")
		}
	}
}
