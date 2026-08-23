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
	"strconv"
	"strings"
	"time"
)

const (
	oauthStateCookieName  = "oauth_state"
	oauthStateTTL         = 10 * time.Minute // OAuth フローは10分以内に完了すべき
	oauthTenantCookieName = "oauth_tenant_slug"
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

// SetOAuthTenantSlug はOAuthフロー開始時に学園サブドメインslugを使い捨てCookieへ保存する。
// state Cookieと同様、ブラウザのOAuthプロバイダ往復をまたいでテナント情報を引き継ぐために使う。
func SetOAuthTenantSlug(w http.ResponseWriter, slug string) {
	if slug == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthTenantCookieName,
		Value:    slug,
		Path:     "/",
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production",
		SameSite: http.SameSiteLaxMode,
	})
}

// ConsumeOAuthTenantSlug はOAuthコールバック時にテナントslug Cookieを読み取り、削除する（使い捨て）。
// Cookieが無ければ空文字を返す（テナント制約なしのOAuthフロー）。
func ConsumeOAuthTenantSlug(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie(oauthTenantCookieName)
	http.SetCookie(w, &http.Cookie{
		Name:     oauthTenantCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production",
		SameSite: http.SameSiteLaxMode,
	})
	if err != nil {
		return ""
	}
	return cookie.Value
}

// signState は state 値を HMAC-SHA256 で署名し "state.signature" 形式の文字列を返す
func signState(state string) string {
	return state + "." + hmacHex(state)
}

func hmacHex(data string) string {
	secret := os.Getenv("OAUTH_STATE_SECRET")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateSignedState はCookieを使わず、任意のペイロード（例: ユーザーID）を
// HMAC署名付きstate文字列に埋め込んで返す。Next.jsプロキシ経由などCookieが
// コールバック先（別オリジンのBackend）まで届かないOAuthフロー向け。
func GenerateSignedState(payload string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(b)
	body := strings.Join([]string{payload, nonce, strconv.FormatInt(time.Now().Unix(), 10)}, "|")
	return body + "." + hmacHex(body), nil
}

// VerifySignedState はGenerateSignedStateで生成されたstate文字列を検証し、
// 有効なら埋め込まれたペイロードを返す。署名不一致・期限切れ・不正な形式は失敗として扱う。
func VerifySignedState(state string) (payload string, ok bool) {
	idx := strings.LastIndex(state, ".")
	if idx < 0 {
		return "", false
	}
	body, sig := state[:idx], state[idx+1:]
	if !hmac.Equal([]byte(sig), []byte(hmacHex(body))) {
		return "", false
	}

	parts := strings.SplitN(body, "|", 3)
	if len(parts) != 3 {
		return "", false
	}
	ts, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", false
	}
	if time.Since(time.Unix(ts, 0)) > oauthStateTTL {
		return "", false
	}
	return parts[0], true
}
