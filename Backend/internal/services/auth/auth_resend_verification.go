package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"
)

// maxEmailLength はメールアドレスの長さの上限(RFC 5321)。
// 認証不要の入口なので、制限器のキーや DB の検索に、巨大な文字列を通さない。
const maxEmailLength = 254

// normalizeResendEmail は再送の入力を、制限器・DB に通せる形(前後の空白除去・小文字)にそろえる。
// 長さの上限を超える、またはメールアドレスの形式でないものは、false を返す。
// 表示名つき(Name <a@b>)や複数指定は、ParseAddress が受け付けても、アドレス部分が入力と一致しないので弾く。
func normalizeResendEmail(raw string) (string, bool) {
	// 長さは、加工(小文字化)の前の入力で見る。巨大な入力を加工しない。
	if len(raw) > maxEmailLength {
		return "", false
	}
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", false
	}
	return email, true
}

// ResendLimiter は認証メールの再送をキー単位で制限する。middleware.KeyRateLimiter と同じ形。
type ResendLimiter interface {
	Allow(key string) bool
}

// SetResendLimiter は認証メール再送の、メールアドレス単位の制限器を設定する。
// 未設定の場合は制限しない(IP 単位の制限は、ルート側のミドルウェアが担う)。
func (s *AuthService) SetResendLimiter(l ResendLimiter) {
	s.resendLimiter = l
}

// ResendVerificationEmail は、メール未認証のユーザーに、認証メールを新しいトークンで再送する。
// 有効期限(24 時間)が切れた、またはメールが届かなかった場合に、自分で復帰できるようにするための入口。
//
// 呼び出し側へ「何が起きたか」を伝えない。存在しない・認証済み・OAuth・ゲスト・退会済み・
// 上限超過のどれでも、何も送らずに nil を返す(アカウントの有無を、応答から推測させない)。
// 返すエラーは、サーバー側の失敗(DB の保存失敗など)だけで、呼び出し側は握りつぶす前提。
func (s *AuthService) ResendVerificationEmail(email string) error {
	email, ok := normalizeResendEmail(email)
	if !ok {
		return nil
	}
	// 存在しないアドレスでも、上限を1回消費する。存在する人だけ回数が減る、という差を作らない。
	if s.resendLimiter != nil && !s.resendLimiter.Allow(email) {
		log.Printf("[AuthService] verification resend rate limited for %s", maskEmail(email))
		return nil
	}

	user, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil || user.IsGuest || user.IsWithdrawn() || user.OAuthProvider != "" || user.EmailVerifiedAt != nil {
		return nil
	}

	// 既存のトークンは新しいもので置き換える(古いリンクは使えなくなる)。
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate verification token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)
	expires := time.Now().Add(24 * time.Hour)
	// 保存できないままメールを送ると、届いたリンクが使えない(DB にトークンが無い)ので、送らない(#2002)。
	// 対象のカラムだけを、未認証の行に限って更新する。読み込んだ行を丸ごと Save すると、
	// 読み込み後に認証が完了していた場合に、認証済みの印(email_verified_at)を巻き戻してしまう。
	updated, err := s.userRepo.RenewEmailVerification(user.ID, token, expires)
	if err != nil {
		return fmt.Errorf("failed to save verification token: %w", err)
	}
	if !updated {
		// 読み込み後に認証済みになった。再送は不要。
		return nil
	}
	user.EmailVerificationToken = token
	user.EmailVerificationExpires = &expires

	s.dispatchVerificationEmail(user)
	return nil
}
