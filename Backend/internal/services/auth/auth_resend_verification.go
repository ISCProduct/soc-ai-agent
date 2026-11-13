package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"
)

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
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
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
	user.EmailVerificationToken = base64.URLEncoding.EncodeToString(tokenBytes)
	expires := time.Now().Add(24 * time.Hour)
	user.EmailVerificationExpires = &expires
	// 保存できないままメールを送ると、届いたリンクが使えない(DB にトークンが無い)ので、送らない(#2002)。
	if err := s.userRepo.UpdateUser(user); err != nil {
		return fmt.Errorf("failed to save verification token: %w", err)
	}

	s.dispatchVerificationEmail(user)
	return nil
}
