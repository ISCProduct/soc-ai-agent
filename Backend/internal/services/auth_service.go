package services

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/config"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// bcryptCost パスワードハッシュのコストパラメータ（#328）
// OWASP 推奨: 12 以上。DefaultCost(10) は現代の GPU 攻撃に対して不十分。
const bcryptCost = 12

type AuthService struct {
	userRepo     repository.UserRepository
	pendingRepo  repository.PendingRegistrationRepository
	emailService *EmailService
	db           *gorm.DB
}

func NewAuthService(userRepo repository.UserRepository, pendingRepo repository.PendingRegistrationRepository, emailService *EmailService) *AuthService {
	return &AuthService{userRepo: userRepo, pendingRepo: pendingRepo, emailService: emailService}
}

// SetDB はアカウント削除に使用する DB を設定する
func (s *AuthService) SetDB(db *gorm.DB) {
	s.db = db
}

// DeleteAccount ユーザーアカウントとその全データを削除する（個人情報保護法第28条対応）
func (s *AuthService) DeleteAccount(userID uint) error {
	if s.db == nil {
		return errors.New("database not configured")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		sessionIDs, err := collectUserSessionIDs(tx, userID)
		if err != nil {
			return err
		}
		if len(sessionIDs) > 0 {
			if err := tx.Where("session_id IN ?", sessionIDs).Delete(&models.SessionValidation{}).Error; err != nil {
				return err
			}
		}

		interviewSessionIDs, err := collectInterviewSessionIDs(tx, userID)
		if err != nil {
			return err
		}
		if len(interviewSessionIDs) > 0 {
			if err := tx.Where("interview_session_id IN ?", interviewSessionIDs).Delete(&models.RealtimeUsageLog{}).Error; err != nil {
				return err
			}
			if err := tx.Where("session_id IN ?", interviewSessionIDs).Delete(&models.InterviewUtterance{}).Error; err != nil {
				return err
			}
			if err := tx.Where("session_id IN ?", interviewSessionIDs).Delete(&models.InterviewReport{}).Error; err != nil {
				return err
			}
			// InterviewVideo は後続の user_id = ? による一括削除でカバーされるため、ここでは削除しない
		}

		resumeDocumentIDs, err := collectResumeDocumentIDs(tx, userID)
		if err != nil {
			return err
		}
		if len(resumeDocumentIDs) > 0 {
			resumeReviewIDs, err := collectResumeReviewIDs(tx, resumeDocumentIDs)
			if err != nil {
				return err
			}
			if len(resumeReviewIDs) > 0 {
				if err := tx.Where("review_id IN ?", resumeReviewIDs).Delete(&models.ResumeReviewItem{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("document_id IN ?", resumeDocumentIDs).Delete(&models.ResumeTextBlock{}).Error; err != nil {
				return err
			}
			if err := tx.Where("document_id IN ?", resumeDocumentIDs).Delete(&models.ResumeReview{}).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("user_id = ?", userID).Delete(&models.ChatMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.AIGeneratedQuestion{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.ConversationContext{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserWeightScore{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserAnalysisProgress{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserEmbedding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.VariantAssignment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserCompanyMatch{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserApplicationStatus{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.CompanyReview{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.ResumeDocument{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.InterviewSession{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.InterviewVideo{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.RealtimeUsageLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.ScheduleEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.SkillScore{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubRepoSummary{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubLanguageStat{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubRepo{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubProfile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("email = ?", user.Email).Delete(&models.PendingRegistration{}).Error; err != nil {
			return err
		}

		if err := tx.Delete(&models.User{}, userID).Error; err != nil {
			return err
		}
		return nil
	})
}

func collectUserSessionIDs(tx *gorm.DB, userID uint) ([]string, error) {
	sessions := map[string]struct{}{}
	collect := func(model any) error {
		var ids []string
		if err := tx.Model(model).
			Where("user_id = ?", userID).
			Distinct().
			Pluck("session_id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if id != "" {
				sessions[id] = struct{}{}
			}
		}
		return nil
	}

	if err := collect(&models.ChatMessage{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserWeightScore{}); err != nil {
		return nil, err
	}
	if err := collect(&models.ConversationContext{}); err != nil {
		return nil, err
	}
	if err := collect(&models.AIGeneratedQuestion{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserAnalysisProgress{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserEmbedding{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserCompanyMatch{}); err != nil {
		return nil, err
	}
	if err := collect(&models.VariantAssignment{}); err != nil {
		return nil, err
	}
	if err := collect(&models.ResumeDocument{}); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(sessions))
	for id := range sessions {
		result = append(result, id)
	}
	return result, nil
}

func collectInterviewSessionIDs(tx *gorm.DB, userID uint) ([]uint, error) {
	var sessionIDs []uint
	if err := tx.Model(&models.InterviewSession{}).
		Where("user_id = ?", userID).
		Pluck("id", &sessionIDs).Error; err != nil {
		return nil, err
	}
	return sessionIDs, nil
}

func collectResumeDocumentIDs(tx *gorm.DB, userID uint) ([]uint, error) {
	var documentIDs []uint
	if err := tx.Model(&models.ResumeDocument{}).
		Where("user_id = ?", userID).
		Pluck("id", &documentIDs).Error; err != nil {
		return nil, err
	}
	return documentIDs, nil
}

func collectResumeReviewIDs(tx *gorm.DB, documentIDs []uint) ([]uint, error) {
	var reviewIDs []uint
	if err := tx.Model(&models.ResumeReview{}).
		Where("document_id IN ?", documentIDs).
		Pluck("id", &reviewIDs).Error; err != nil {
		return nil, err
	}
	return reviewIDs, nil
}

// RegisterRequest ユーザー登録リクエスト
type RegisterRequest struct {
	Email                    string `json:"email"`
	Password                 string `json:"password"`
	Name                     string `json:"name"`
	TargetLevel              string `json:"target_level"`
	SchoolName               string `json:"school_name"`
	CertificationsAcquired   string `json:"certifications_acquired"`
	CertificationsInProgress string `json:"certifications_in_progress"`
	RegistrationToken        string `json:"registration_token"`
}

// LoginRequest ログインリクエスト
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UpdateProfileRequest プロフィール更新リクエスト
type UpdateProfileRequest struct {
	UserID                   uint   `json:"user_id"`
	Name                     string `json:"name"`
	TargetLevel              string `json:"target_level"`
	SchoolName               string `json:"school_name"`
	CertificationsAcquired   string `json:"certifications_acquired"`
	CertificationsInProgress string `json:"certifications_in_progress"`
}

// AuthResponse 認証レスポンス
type AuthResponse struct {
	UserID                   uint   `json:"user_id"`
	Email                    string `json:"email"`
	Name                     string `json:"name"`
	IsGuest                  bool   `json:"is_guest"`
	TargetLevel              string `json:"target_level"`
	SchoolName               string `json:"school_name,omitempty"`
	IsAdmin                  bool   `json:"is_admin"`
	CertificationsAcquired   string `json:"certifications_acquired,omitempty"`
	CertificationsInProgress string `json:"certifications_in_progress,omitempty"`
	AvatarURL                string `json:"avatar_url,omitempty"`
	OAuthProvider            string `json:"oauth_provider,omitempty"` // OAuth連携プロバイダ
	Token                    string `json:"token,omitempty"`          // 管理者トークン（管理者ユーザーのみ）
	UserToken                string `json:"user_token,omitempty"`     // ユーザー認証トークン（全ユーザー）
	EmailVerified            bool   `json:"email_verified"`
	RequiresReVerification   bool   `json:"requires_re_verification,omitempty"`
}

// RequestRegistration メールアドレスに確認URLを送信して仮登録を作成
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
	_ = s.pendingRepo.DeleteByEmail(email)

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
		_ = s.pendingRepo.DeleteByEmail(req.Email)
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
		IsAdmin:                  isAdminIdentity(req.Email),
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
	go s.emailService.SendVerificationEmail(user, user.EmailVerificationToken, appURL)

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

// Login ログイン処理
func (s *AuthService) Login(req LoginRequest) (*AuthResponse, error) {
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

	promoteAdminIfMatched(user, s.userRepo)

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
			go s.emailService.SendReVerificationEmail(user, user.EmailVerificationToken, appURL)
			requiresReVerification = true
			return nil, errors.New("re_verification_required")
		}
	}

	// 最終ログイン更新
	now := time.Now()
	user.LastLoginAt = &now
	s.userRepo.UpdateUser(user)

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
	adminSecret := os.Getenv("ADMIN_SECRET")
	// 管理者ユーザーにはHMACトークンを付与する
	if user.IsAdmin && adminSecret != "" {
		resp.Token = middleware.GenerateAdminToken(user.ID, user.Email, adminSecret)
	}
	// 全ユーザーにユーザー認証トークンを付与する
	if adminSecret != "" {
		resp.UserToken = middleware.GenerateUserToken(user.ID, user.Email, adminSecret)
	}
	return resp, nil
}

// CreateGuestUser ゲストユーザー作成
func (s *AuthService) CreateGuestUser() (*AuthResponse, error) {
	// ランダムなゲストID生成
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random ID: %w", err)
	}
	guestID := base64.URLEncoding.EncodeToString(randomBytes)

	user := &entity.User{
		Email:       fmt.Sprintf("guest_%s@%s", guestID, config.GuestEmailDomain()),
		Password:    "", // ゲストユーザーはパスワード不要
		Name:        fmt.Sprintf("Guest_%s", guestID[:8]),
		IsGuest:     true,
		TargetLevel: "未設定",
		SchoolName:  config.SchoolName(),
	}

	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("failed to create guest user: %w", err)
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
		AvatarURL:                user.AvatarURL,
	}, nil
}

// GetUser ユーザー情報取得
func (s *AuthService) GetUser(userID uint) (*AuthResponse, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
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
		AvatarURL:                user.AvatarURL,
		OAuthProvider:            user.OAuthProvider,
	}, nil
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
	user.CertificationsAcquired = req.CertificationsAcquired
	user.CertificationsInProgress = req.CertificationsInProgress

	if err := s.userRepo.UpdateUser(user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
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
		AvatarURL:                user.AvatarURL,
	}, nil
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

	return s.userRepo.UpdateUser(user)
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
