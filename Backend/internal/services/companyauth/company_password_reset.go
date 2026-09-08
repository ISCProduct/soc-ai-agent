package companyauth

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"Backend/internal/config"
	"Backend/internal/models"

	"golang.org/x/crypto/bcrypt"
)

// passwordResetTTL はリセットリンクの有効期間。
// 学生側 (config.PasswordResetTokenTTL) と揃える。
func passwordResetTTL() time.Duration { return config.PasswordResetTokenTTL }

// RequestPasswordReset はパスワード再設定用のトークンを発行してメールを送る。
//
// 呼び出し側は結果に関わらず同じレスポンスを返すこと。
// アカウントが存在しない・招待未受諾・無効化済みのいずれでも nil を返すのは、
// レスポンスの differences からアカウントの存在を推測されないようにするため。
func (s *CompanyUserService) RequestPasswordReset(req ForgotPasswordRequest) error {
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	if emailAddr == "" {
		return nil
	}

	user, err := s.users.FindByEmail(emailAddr)
	if err != nil {
		return err
	}
	// 存在しない / まだ招待を受諾していない / 無効化済み のいずれもメールを送らない。
	// ただし呼び出し側には成功として返す。
	if user == nil || !user.PasswordSet() || user.Disabled() {
		return nil
	}

	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}
	tokenHash := hashToken(token)
	expires := s.now().Add(passwordResetTTL())

	// 列を指定して更新する。Save(全カラム更新)だと、並行する無効化操作を
	// 巻き戻して disabled_at を消してしまう（レビュー指摘）。
	if err := s.users.SetPasswordResetToken(user.ID, &tokenHash, &expires); err != nil {
		return err
	}

	if s.email != nil {
		// メール送信をリクエストパスから外す。
		// 同期送信だと「未登録=SELECTのみ(〜1ms)」と「登録済み=外部API往復(数百ms)」で
		// 応答時間が2〜3桁変わり、本文とステータスを揃えてもタイミングで存在が漏れる。
		//
		// 送信失敗はログのみ。エラーを返すと送信可否から存在が漏れる。
		addr, userID := user.Email, user.ID
		go func() {
			if err := s.email.SendCompanyUserPasswordReset(addr, token); err != nil {
				log.Printf("[CompanyUserService] password reset email failed company_user_id=%d error=%v", userID, err)
			}
		}()
	}
	return nil
}

// ResetPassword はトークンを検証してパスワードを再設定し、そのままログインさせる。
// 再設定時には既存のリフレッシュトークンを全て失効させる。
// パスワード流出を前提とした操作なので、他端末のセッションを残さない。
func (s *CompanyUserService) ResetPassword(req ResetPasswordRequest) (*AuthResponse, error) {
	token := strings.TrimSpace(req.Token)
	if token == "" || req.Password == "" {
		return nil, errors.New("token and password are required")
	}
	if len(req.Password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	user, err := s.users.FindByPasswordResetTokenHash(hashToken(token))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrResetTokenInvalid
	}
	if user.PasswordResetExpiresAt == nil || s.now().After(*user.PasswordResetExpiresAt) {
		return nil, ErrResetTokenExpired
	}
	if user.Disabled() {
		return nil, ErrAccountDisabled
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, err
	}
	// パスワード設定とトークンの使い捨てを1文で行う。
	// disabled_at IS NULL を条件に入れているので、bcrypt(約300ms)の最中に
	// 管理者が無効化した場合は 0 行更新になり、ここで失敗する。
	affected, err := s.users.ApplyPasswordReset(user.ID, string(hashed))
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, ErrAccountDisabled
	}
	user.Password = string(hashed)
	user.PasswordResetTokenHash = nil
	user.PasswordResetExpiresAt = nil

	// 既存セッションを全て切る。
	//
	// ここを失敗のままログだけにして新しいセッションを返すと、
	// 「パスワード変更で他端末を全て切る」という保証が成立しない。
	// パスワード流出を前提とした操作なので、失効できないなら失敗させる。
	// パスワード自体は既に変わっているので、利用者は再試行すればよい。
	if s.refresh != nil {
		if err := s.refresh.RevokeAllByCompanyUser(user.ID, s.now()); err != nil {
			log.Printf("[CompanyUserService] revoke refresh tokens failed company_user_id=%d error=%v", user.ID, err)
			return nil, fmt.Errorf("既存セッションの失効に失敗しました: %w", err)
		}
	}

	return s.buildAuthResponse(user, true)
}

// SetDisabled は企業ユーザーの有効/無効を切り替える（admin 操作）。
// 無効化時は既存のリフレッシュトークンを失効させ、その場でアクセスを断つ。
//
// 行を削除しないのは、company_student_tags.created_by が参照しており、
// 削除するとその担当者が付けた自社タグまで失われるため（#1196）。
func (s *CompanyUserService) SetDisabled(companyID, companyUserID uint, disabled bool) (*models.CompanyUser, error) {
	user, err := s.users.FindByID(companyUserID)
	if err != nil {
		return nil, err
	}
	// 他社の企業ユーザーは操作できない。
	if user == nil || user.CompanyID != companyID {
		return nil, ErrUserNotFound
	}

	var disabledAt *time.Time
	if disabled {
		now := s.now()
		disabledAt = &now
	}
	// 列指定で更新する。Save だと古いモデルの全カラム書き戻しで
	// 並行更新を巻き戻し、無効化済みアカウントが復活しうる。
	// リセット導線を塞ぐのはリポジトリ側で行う。
	if err := s.users.SetDisabled(user.ID, disabledAt); err != nil {
		return nil, err
	}
	user.DisabledAt = disabledAt
	if disabled {
		user.PasswordResetTokenHash = nil
		user.PasswordResetExpiresAt = nil
	}

	if disabled && s.refresh != nil {
		if err := s.refresh.RevokeAllByCompanyUser(user.ID, s.now()); err != nil {
			log.Printf("[CompanyUserService] revoke refresh tokens failed company_user_id=%d error=%v", user.ID, err)
		}
	}
	return user, nil
}
