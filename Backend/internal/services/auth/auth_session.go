package auth

import (
	"Backend/domain/entity"
	"Backend/internal/config"
	"Backend/internal/services/refreshtoken"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Login はメール・パスワードでログインする。
// tenantOrgID はHostサブドメインから解決された組織ID（0の場合はテナント制約なし）。
func (s *AuthService) Login(req LoginRequest, tenantOrgID uint) (*AuthResponse, error) {
	// バリデーション
	if req.Email == "" || req.Password == "" {
		return nil, errors.New("email and password are required")
	}

	// ユーザー取得
	user, err := s.userRepo.GetUserByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("invalid email or password")
	}
	if user.IsWithdrawn() {
		return nil, errors.New("account has been withdrawn")
	}
	if tenantOrgID != 0 && !user.IsAdmin && user.OrganizationID != tenantOrgID {
		return nil, errors.New("tenant mismatch")
	}

	// ゲストユーザーはログイン不可
	if user.IsGuest {
		return nil, errors.New("guest users cannot login")
	}

	// パスワード検証
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	// コストが古い場合は再ハッシュ（#328: bcryptCost=12 への移行）
	if cost, err := bcrypt.Cost([]byte(user.Password)); err == nil && cost < bcryptCost {
		if rehashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost); err == nil {
			user.Password = string(rehashed)
			_ = s.userRepo.UpdateUser(user)
		}
	}

	isOAuth := user.OAuthProvider != ""
	emailVerified := user.EmailVerifiedAt != nil
	requiresReVerification := false

	if !isOAuth {
		// メール認証チェック
		if !emailVerified {
			return nil, errors.New("email_not_verified")
		}

		// 10日以上ログインなし → 再認証
		if user.LastLoginAt != nil && time.Since(*user.LastLoginAt) > config.ReVerificationInactiveDuration {
			tokenBytes := make([]byte, 24)
			if _, err := rand.Read(tokenBytes); err != nil {
				return nil, fmt.Errorf("failed to generate re-verification token: %w", err)
			}
			user.EmailVerificationToken = base64.URLEncoding.EncodeToString(tokenBytes)
			reVerExpires := time.Now().Add(24 * time.Hour)
			user.EmailVerificationExpires = &reVerExpires
			user.EmailVerifiedAt = nil
			s.userRepo.UpdateUser(user)
			appURL := config.AppURL()
			if s.jobs != nil {
				if err := s.jobs.EnqueueEmailReVerification(user.ID, user.Email, user.Name, user.EmailVerificationToken, appURL); err != nil {
					log.Printf("[AuthService] enqueue re-verification email failed, fallback goroutine: %v", err)
					go s.emailService.SendReVerificationEmail(user, user.EmailVerificationToken, appURL)
				}
			} else {
				go s.emailService.SendReVerificationEmail(user, user.EmailVerificationToken, appURL)
			}
			requiresReVerification = true
			return nil, errors.New("re_verification_required")
		}
	}

	// 最終ログイン更新
	now := time.Now()
	user.LastLoginAt = &now
	err = s.userRepo.UpdateUser(user)
	if err != nil {
		return nil, err
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
		EmailVerified:            emailVerified,
		RequiresReVerification:   requiresReVerification,
	}
	s.attachAuthTokens(resp, user, true)
	return resp, nil
}

// CreateGuestUser ゲストユーザー作成
// tenantOrgID はHostサブドメインから解決された組織ID（0の場合はデフォルト組織へ所属）。
func (s *AuthService) CreateGuestUser(tenantOrgID uint) (*AuthResponse, error) {
	// ランダムなゲストID生成
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random ID: %w", err)
	}
	guestID := base64.URLEncoding.EncodeToString(randomBytes)

	user := &entity.User{
		Email:          fmt.Sprintf("guest_%s@%s", guestID, config.GuestEmailDomain()),
		Password:       "", // ゲストユーザーはパスワード不要
		Name:           fmt.Sprintf("Guest_%s", guestID[:8]),
		IsGuest:        true,
		TargetLevel:    "未設定",
		SchoolName:     config.SchoolName(),
		OrganizationID: tenantOrgID,
	}

	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("failed to create guest user: %w", err)
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
	s.attachAuthTokens(resp, user, true)
	return resp, nil
}

// RefreshSession はリフレッシュトークンをローテーションし、新しいトークンペアを返す (#616)
func (s *AuthService) RefreshSession(refreshToken string) (*AuthResponse, error) {
	if s.refreshTokens == nil {
		return nil, errors.New("refresh token service not configured")
	}
	userSecret := os.Getenv("USER_SECRET")
	if userSecret == "" {
		return nil, errors.New("USER_SECRET is not configured")
	}

	userID, newRefreshToken, err := s.refreshTokens.Rotate(refreshToken)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// 退会済みユーザーはリフレッシュ不可 (#613)
	if user == nil || user.IsWithdrawn() {
		return nil, refreshtoken.ErrInvalidRefreshToken
	}

	resp := &AuthResponse{
		UserID:       user.ID,
		Email:        user.Email,
		Name:         user.Name,
		IsGuest:      user.IsGuest,
		IsAdmin:      user.IsAdmin,
		RefreshToken: newRefreshToken,
	}
	s.attachAuthTokens(resp, user, false)
	return resp, nil
}

// LogoutSession はリフレッシュトークンを失効させる (#616)
func (s *AuthService) LogoutSession(refreshToken string) error {
	if s.refreshTokens == nil || refreshToken == "" {
		return nil
	}
	return s.refreshTokens.Revoke(refreshToken)
}
