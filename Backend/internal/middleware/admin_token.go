package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"Backend/domain/entity"
)

// defaultAdminTokenTTL は管理者トークンの既定の有効期間。
//
// 一般ユーザーのJWT(1時間, #616)より大幅に長い。管理者トークンには**リフレッシュ経路が無い**ため。
// 再発行されるのはログイン時と GET /api/auth/user の応答(frontend/lib/auth.ts, #857)だけで、
// 管理画面(app/admin/**)はどちらも呼ばないため、短いTTLにすると操作の途中で 403 になり
// 再ログインしか復帰手段が無くなる。
//
// 無期限(修正前)より確実に良いが、漏洩時の有効期間を数時間に縮めるには
// 管理者トークンのリフレッシュ経路を用意する必要がある(別Issue)。
// それまでの間、運用で短縮したい場合は ADMIN_TOKEN_TTL_HOURS で上書きできる。
const defaultAdminTokenTTL = 7 * 24 * time.Hour

// adminTokenClockSkew は発行時刻が未来にずれている場合に許容する幅。
const adminTokenClockSkew = 5 * time.Minute

var (
	// ErrAdminTokenInvalid は署名不一致・形式不正（期限を持たない旧形式を含む）を表す。
	ErrAdminTokenInvalid = errors.New("admin token invalid")
	// ErrAdminTokenExpired は署名は正しいが有効期限を過ぎていることを表す。
	ErrAdminTokenExpired = errors.New("admin token expired")
)

// adminTokenTTL は有効期間を返す。ADMIN_TOKEN_TTL_HOURS が正の整数なら優先する。
func adminTokenTTL() time.Duration {
	if v := os.Getenv("ADMIN_TOKEN_TTL_HOURS"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h > 0 {
			return time.Duration(h) * time.Hour
		}
	}
	return defaultAdminTokenTTL
}

// adminTokenSignature は発行時刻を含めた payload の HMAC-SHA256 を返す。
func adminTokenSignature(userID uint, email, secret string, issuedAtUnix int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d:%s:%d", userID, email, issuedAtUnix)
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateAdminToken は発行時刻付きの管理者トークンを生成する（#1155）。
// 形式は "<発行時刻(unix秒)>.<HMAC-SHA256>"。
// 発行時刻を payload に含めるため、同じ管理者でも発行ごとに異なる値になる。
func GenerateAdminToken(userID uint, email, secret string) string {
	return generateAdminTokenAt(userID, email, secret, time.Now())
}

func generateAdminTokenAt(userID uint, email, secret string, now time.Time) string {
	issuedAtUnix := now.Unix()
	return fmt.Sprintf("%d.%s", issuedAtUnix, adminTokenSignature(userID, email, secret, issuedAtUnix))
}

// ParseAdminToken は署名と有効期限を検証し、発行時刻を返す。
func ParseAdminToken(token string, userID uint, email, secret string) (time.Time, error) {
	return parseAdminTokenAt(token, userID, email, secret, time.Now())
}

func parseAdminTokenAt(token string, userID uint, email, secret string, now time.Time) (time.Time, error) {
	if secret == "" || token == "" {
		return time.Time{}, ErrAdminTokenInvalid
	}
	rawIssuedAt, sig, ok := strings.Cut(token, ".")
	if !ok {
		// 期限を持たない旧形式（HMAC-SHA256 の hex 64文字のみ）は受け付けない。
		// ただし旧形式を持っているのは正規の管理者なので、無効ではなく期限切れとして扱い
		// 「再ログインしてください」と案内できるようにする(#1155)。
		if isLegacyAdminToken(token) {
			return time.Time{}, ErrAdminTokenExpired
		}
		return time.Time{}, ErrAdminTokenInvalid
	}
	issuedAtUnix, err := strconv.ParseInt(rawIssuedAt, 10, 64)
	if err != nil {
		return time.Time{}, ErrAdminTokenInvalid
	}
	expected := adminTokenSignature(userID, email, secret, issuedAtUnix)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return time.Time{}, ErrAdminTokenInvalid
	}

	issuedAt := time.Unix(issuedAtUnix, 0)
	if issuedAt.After(now.Add(adminTokenClockSkew)) {
		// 未来日付の発行時刻は有効期限判定を無効化できてしまうため拒否する
		return time.Time{}, ErrAdminTokenInvalid
	}
	if now.Sub(issuedAt) > adminTokenTTL() {
		return issuedAt, ErrAdminTokenExpired
	}
	return issuedAt, nil
}

// isLegacyAdminToken は #1155 以前の形式（発行時刻を持たない HMAC-SHA256 の hex）かを判定する。
func isLegacyAdminToken(token string) bool {
	if len(token) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}

// VerifyAdminToken はトークンの署名と有効期限を検証する。
func VerifyAdminToken(token string, userID uint, email, secret string) bool {
	_, err := ParseAdminToken(token, userID, email, secret)
	return err == nil
}

// ValidateAdminTokenForUser は署名・有効期限に加えて、管理者1名単位の失効を検証する（#1155）。
//
// users.admin_token_not_before にその時刻より前に発行されたトークンを拒否させることで、
// ADMIN_SECRET をローテーションして全管理者を巻き込むことなく、特定の管理者のトークンだけを
// 失効できる。失効の手順は migration 000024 のコメントを参照。
//
// 戻り値は nil(有効) / ErrAdminTokenExpired / ErrAdminTokenInvalid。
// 期限切れだけを区別するのは、呼び出し側が「再ログインしてください」と案内できるようにするため
// （署名が正しい＝正規のトークンを持っていた相手にしか返らない）。
func ValidateAdminTokenForUser(token string, user *entity.User, secret string) error {
	if user == nil {
		return ErrAdminTokenInvalid
	}
	issuedAt, err := ParseAdminToken(token, user.ID, user.Email, secret)
	if err != nil {
		return err
	}
	if user.AdminTokenNotBefore != nil && issuedAt.Before(*user.AdminTokenNotBefore) {
		// 個別失効は「無効」として扱う（失効済みであることを相手に伝えない）
		return ErrAdminTokenInvalid
	}
	return nil
}
