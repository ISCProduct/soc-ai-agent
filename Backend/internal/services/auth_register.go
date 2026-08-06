package services

import (
	"Backend/domain/entity"
	"Backend/internal/config"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *AuthService) RequestRegistration(email string) error {
	if email == "" {
		return errors.New("email is required")
	}

	// 既存ユーザーチェック
	existing, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if existing != nil {
		return errors.New("email already exists")
	}

	// 以前の仮登録を削除
	if err := s.pendingRepo.DeleteByEmail(email); err != nil {
		log.Printf("[AuthService] failed to delete previous pending registration for %s: %v", email, err)
	}

	// トークン生成
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(b)

	pending := &entity.PendingRegistration{
		Token:     token,
		Email:     email,
		ExpiresAt: time.Now().Add(config.PendingRegistrationTokenTTL),
	}
	if err := s.pendingRepo.Create(pending); err != nil {
		return fmt.Errorf("failed to save pending registration: %w", err)
	}

	if s.jobs != nil {
		if err := s.jobs.EnqueueEmailRegistration(email, token); err != nil {
			log.Printf("[AuthService] enqueue registration email failed, fallback sync: %v", err)
			return s.emailService.SendRegistrationEmail(email, token)
		}
		return nil
	}
	return s.emailService.SendRegistrationEmail(email, token)
}

// ValidateRegistrationToken 仮登録トークンを検証してメールアドレスを返す
func (s *AuthService) ValidateRegistrationToken(token string) (string, error) {
	pending, err := s.pendingRepo.FindByToken(token)
	if err != nil {
		return "", fmt.Errorf("failed to find token: %w", err)
	}
	if pending == nil {
		return "", errors.New("invalid or expired token")
	}
	return pending.Email, nil
}

// Register 新規ユーザー登録
func (s *AuthService) Register(req RegisterRequest) (*AuthResponse, error) {
	// バリデーション
	if req.Email == "" || req.Password == "" {
		return nil, errors.New("email and password are required")
	}
	if len(req.Password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	// トークン検証
	if req.RegistrationToken != "" {
		pending, err := s.pendingRepo.FindByToken(req.RegistrationToken)
		if err != nil {
			return nil, fmt.Errorf("failed to validate token: %w", err)
		}
		if pending == nil || pending.Email != req.Email {
			return nil, errors.New("invalid or expired registration token")
		}
		// 使用済みトークンを削除
		if err := s.pendingRepo.DeleteByEmail(req.Email); err != nil {
			log.Printf("[AuthService] failed to delete used registration token for %s: %v", req.Email, err)
		}
	}
	if req.TargetLevel == "" {
		req.TargetLevel = "新卒"
	}
	if req.TargetLevel != "新卒" && req.TargetLevel != "中途" {
		return nil, errors.New("target_level must be '新卒' or '中途'")
	}
	if strings.TrimSpace(req.SchoolName) == "" {
		req.SchoolName = config.SchoolName()
	}

	// 既存ユーザーチェック
	existingUser, err := s.userRepo.GetUserByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, errors.New("email already exists")
	}

	// パスワードハッシュ化
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// ユーザー作成
	user := &entity.User{
		Email:                    req.Email,
		Password:                 string(hashedPassword),
		Name:                     req.Name,
		IsGuest:                  false,
		TargetLevel:              req.TargetLevel,
		SchoolName:               req.SchoolName,
		IsAdmin:                  false,
		CertificationsAcquired:   req.CertificationsAcquired,
		CertificationsInProgress: req.CertificationsInProgress,
	}

	// メール認証トークン生成（有効期限 24 時間）（#330）
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate verification token: %w", err)
	}
	user.EmailVerificationToken = base64.URLEncoding.EncodeToString(tokenBytes)
	emailVerExpires := time.Now().Add(24 * time.Hour)
	user.EmailVerificationExpires = &emailVerExpires

	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 認証メール送信（失敗しても登録は成功扱い）
	appURL := config.AppURL()
	if s.jobs != nil {
		if err := s.jobs.EnqueueEmailVerification(user.ID, user.Email, user.Name, user.EmailVerificationToken, appURL); err != nil {
			log.Printf("[AuthService] enqueue verification email failed, fallback goroutine: %v", err)
			go func() {
				if err := s.emailService.SendVerificationEmail(user, user.EmailVerificationToken, appURL); err != nil {
					log.Printf("[AuthService] failed to send verification email to %s: %v", user.Email, err)
				}
			}()
		}
	} else {
		go func() {
			if err := s.emailService.SendVerificationEmail(user, user.EmailVerificationToken, appURL); err != nil {
				log.Printf("[AuthService] failed to send verification email to %s: %v", user.Email, err)
			}
		}()
	}

	return &AuthResponse{
		UserID:                   user.ID,
		Email:                    user.Email,
		Name:                     user.Name,
		IsGuest:                  user.IsGuest,
		TargetLevel:              user.TargetLevel,
		SchoolName:               user.SchoolName,
		IsAdmin:                  user.IsAdmin,
		CertificationsAcquired:   user.CertificationsAcquired,
		CertificationsInProgress: user.CertificationsInProgress,
		EmailVerified:            false,
	}, nil
}

// VerifyEmail トークンを検証してメールを認証済みにする（#330: 有効期限チェック追加）
func (s *AuthService) VerifyEmail(token string) error {
	if token == "" {
		return errors.New("token is required")
	}
	user, err := s.userRepo.GetUserByVerificationToken(token)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return errors.New("invalid or expired token")
	}
	// 有効期限チェック（ExpiresAt が設定されている場合のみ）
	if user.EmailVerificationExpires != nil && time.Now().After(*user.EmailVerificationExpires) {
		return errors.New("invalid or expired token")
	}
	now := time.Now()
	user.EmailVerifiedAt = &now
	user.EmailVerificationToken = ""
	user.EmailVerificationExpires = nil
	return s.userRepo.UpdateUser(user)
}
