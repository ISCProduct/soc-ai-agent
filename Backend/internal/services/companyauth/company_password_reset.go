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

	user.PasswordResetTokenHash = &tokenHash
	user.PasswordResetExpiresAt = &expires
	if err := s.users.Update(user); err != nil {
		return err
	}

	if s.email != nil {
		if err := s.email.SendCompanyUserPasswordReset(user.Email, token); err != nil {
			// メール送信の失敗でエラーを返すと、送信可否からアカウントの
			// 存在が漏れる。ログに残して成功として返す。
			log.Printf("[CompanyUserService] password reset email failed company_user_id=%d error=%v", user.ID, err)
		}
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
	user.Password = string(hashed)
	user.PasswordResetTokenHash = nil
	user.PasswordResetExpiresAt = nil
	if err := s.users.Update(user); err != nil {
		return nil, err
	}

	// 既存セッションを全て切る。失敗しても再設定自体は成立させる。
	if s.refresh != nil {
		if err := s.refresh.RevokeAllByCompanyUser(user.ID, s.now()); err != nil {
			log.Printf("[CompanyUserService] revoke refresh tokens failed company_user_id=%d error=%v", user.ID, err)
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

	if disabled {
		now := s.now()
		user.DisabledAt = &now
		// 無効化と同時にリセット導線も塞ぐ。
		user.PasswordResetTokenHash = nil
		user.PasswordResetExpiresAt = nil
	} else {
		user.DisabledAt = nil
	}
	if err := s.users.Update(user); err != nil {
		return nil, err
	}

	if disabled && s.refresh != nil {
		if err := s.refresh.RevokeAllByCompanyUser(user.ID, s.now()); err != nil {
			log.Printf("[CompanyUserService] revoke refresh tokens failed company_user_id=%d error=%v", user.ID, err)
		}
	}
	return user, nil
}
