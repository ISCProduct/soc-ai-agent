package auth

import (
	"Backend/internal/config"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// GetUser ユーザー情報取得
func (s *AuthService) GetUser(userID uint) (*AuthResponse, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	resp := &AuthResponse{
		UserID:                   user.ID,
		Email:                    user.Email,
		Name:                     user.Name,
		IsGuest:                  user.IsGuest,
		TargetLevel:              user.TargetLevel,
		SchoolName:               user.SchoolName,
		IsAdmin:                  user.IsAdmin,
		CertificationsAcquired:   user.CertificationsAcquired,
		CertificationsInProgress: user.CertificationsInProgress,
		AvatarURL:                user.AvatarURL,
		OAuthProvider:            user.OAuthProvider,
	}
	s.attachAuthTokens(resp, user, false)
	return resp, nil
}

// UpdateProfile ユーザープロフィール更新
func (s *AuthService) UpdateProfile(req UpdateProfileRequest) (*AuthResponse, error) {
	if req.UserID == 0 {
		return nil, errors.New("user_id is required")
	}
	if req.TargetLevel != "" && req.TargetLevel != "新卒" && req.TargetLevel != "中途" {
		return nil, errors.New("target_level must be '新卒' or '中途'")
	}

	user, err := s.userRepo.GetUserByID(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.TargetLevel != "" {
		user.TargetLevel = req.TargetLevel
	}
	// Always persist the provided school name, even when it is an empty string.
	user.SchoolName = req.SchoolName
	user.SchoolID = s.resolveSchoolID(req.SchoolName)
	user.CertificationsAcquired = req.CertificationsAcquired
	user.CertificationsInProgress = req.CertificationsInProgress

	if err := s.userRepo.UpdateUser(user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	resp := &AuthResponse{
		UserID:                   user.ID,
		Email:                    user.Email,
		Name:                     user.Name,
		IsGuest:                  user.IsGuest,
		TargetLevel:              user.TargetLevel,
		SchoolName:               user.SchoolName,
		IsAdmin:                  user.IsAdmin,
		CertificationsAcquired:   user.CertificationsAcquired,
		CertificationsInProgress: user.CertificationsInProgress,
		AvatarURL:                user.AvatarURL,
	}
	s.attachAuthTokens(resp, user, false)
	return resp, nil
}

// RequestPasswordReset パスワードリセットメールを送信
func (s *AuthService) RequestPasswordReset(email string) error {
	user, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	// ユーザーが存在しない・OAuthユーザー・ゲストの場合でも成功を返す（情報漏洩防止）
	if user == nil || user.OAuthProvider != "" || user.IsGuest {
		return nil
	}

	// 32バイトのランダムトークンを生成
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(b)

	expiresAt := time.Now().Add(config.PasswordResetTokenTTL)
	user.PasswordResetToken = token
	user.PasswordResetExpiresAt = &expiresAt

	if err := s.userRepo.UpdateUser(user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	appURL := config.AppURL()
	if s.jobs != nil {
		if err := s.jobs.EnqueueEmailPasswordReset(user.Email, token, appURL); err != nil {
			log.Printf("[AuthService] enqueue password reset email failed, fallback sync: %v", err)
			return s.emailService.SendPasswordResetEmail(user.Email, token, appURL)
		}
		return nil
	}
	return s.emailService.SendPasswordResetEmail(user.Email, token, appURL)
}

// ResetPassword トークンを検証して新パスワードをセット
func (s *AuthService) ResetPassword(token, newPassword string) error {
	if token == "" {
		return errors.New("token is required")
	}

	user, err := s.userRepo.GetUserByPasswordResetToken(token)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return errors.New("invalid or expired token")
	}
	if user.PasswordResetExpiresAt == nil || time.Now().After(*user.PasswordResetExpiresAt) {
		return errors.New("invalid or expired token")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.Password = string(hashedPassword)
	user.PasswordResetToken = ""
	user.PasswordResetExpiresAt = nil

	if err := s.userRepo.UpdateUser(user); err != nil {
		return err
	}

	// パスワード変更時は全端末のリフレッシュトークンを失効させる (#616)
	if s.refreshTokens != nil {
		if err := s.refreshTokens.RevokeAllForUser(user.ID); err != nil {
			log.Printf("refresh token revoke all failed user_id=%d error=%v", user.ID, err)
		}
	}
	return nil
}
