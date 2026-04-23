package middleware

// OAuth CSRF 対策 - state パラメータの Cookie 保存と検証（Issue #324）
// state をサーバー側の HttpOnly Cookie に保存し、コールバック時に HMAC 署名付きで検証する。

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	oauthStateCookieName = "oauth_state"
	oauthStateTTL        = 10 * time.Minute // OAuth フローは10分以内に完了すべき
)

// GenerateOAuthState はランダムな state 値を生成し、HMAC 署名付き Cookie にセットして state 文字列を返す。
// state はコールバック時に VerifyOAuthState で検証する。
func GenerateOAuthState(w http.ResponseWriter) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.URLEncoding.EncodeToString(b)
	signed := signState(state)

	secure := os.Getenv("APP_ENV") == "production"
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    signed,
		Path:     "/",
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	return state, nil
}

// VerifyOAuthState はリクエストの state パラメータと Cookie を比較し、一致しない場合は false を返す。
// 検証後は Cookie を削除する（使い捨て）。
func VerifyOAuthState(w http.ResponseWriter, r *http.Request) bool {
	stateParam := r.URL.Query().Get("state")

	cookie, err := r.Cookie(oauthStateCookieName)
	// 検証後は Cookie を削除（使い捨て）
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production",
		SameSite: http.SameSiteLaxMode,
	})

	if err != nil || stateParam == "" {
		return false
	}

	// Cookie の値は "state.signature" 形式
	expected := signState(stateParam)
	return hmac.Equal([]byte(cookie.Value), []byte(expected))
}

// signState は state 値を HMAC-SHA256 で署名し "state.signature" 形式の文字列を返す
func signState(state string) string {
	secret := os.Getenv("ADMIN_SECRET")
	if secret == "" {
		secret = "dev-oauth-state-secret" // 開発環境フォールバック（本番では必ず設定）
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(state))
	sig := hex.EncodeToString(mac.Sum(nil))
	return strings.Join([]string{state, sig}, ".")
}
