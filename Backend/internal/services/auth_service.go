package services

import (
	"Backend/domain/repository"
	"log"

	"gorm.io/gorm"
)

// bcryptCost パスワードハッシュのコストパラメータ（#328）
// OWASP 推奨: 12 以上。DefaultCost(10) は現代の GPU 攻撃に対して不十分。
const bcryptCost = 12

type AuthService struct {
	userRepo      repository.UserRepository
	pendingRepo   repository.PendingRegistrationRepository
	emailService  *EmailService
	db            *gorm.DB
	object        ObjectDeleter
	audit         auditRecorder
	deletion      *UserDeletionService
	refreshTokens *RefreshTokenService
}

func NewAuthService(userRepo repository.UserRepository, pendingRepo repository.PendingRegistrationRepository, emailService *EmailService) *AuthService {
	return &AuthService{userRepo: userRepo, pendingRepo: pendingRepo, emailService: emailService}
}

// SetDB はアカウント削除に使用する DB を設定する
func (s *AuthService) SetDB(db *gorm.DB) {
	s.db = db
	s.rebuildDeletionService()
}

// SetObjectDeleter は退会時の S3 オブジェクト削除先を設定する
func (s *AuthService) SetObjectDeleter(d ObjectDeleter) {
	s.object = d
	s.rebuildDeletionService()
}

// SetAuditLog は退会監査ログの出力先を設定する
func (s *AuthService) SetAuditLog(audit auditRecorder) {
	s.audit = audit
	s.rebuildDeletionService()
}

func (s *AuthService) rebuildDeletionService() {
	if s.db == nil {
		s.deletion = nil
		return
	}
	s.deletion = NewUserDeletionService(s.db, s.object, s.audit)
}

// SetRefreshTokenService はリフレッシュトークン管理サービスを注入する (#616)
func (s *AuthService) SetRefreshTokenService(rts *RefreshTokenService) {
	s.refreshTokens = rts
}

// issueRefreshToken はリフレッシュトークンを発行する（サービス未注入時は空文字を返す）
func (s *AuthService) issueRefreshToken(userID uint) string {
	if s.refreshTokens == nil {
		return ""
	}
	token, err := s.refreshTokens.Issue(userID)
	if err != nil {
		log.Printf("refresh token issue failed user_id=%d error=%v", userID, err)
		return ""
	}
	return token
}

type RegisterRequest struct {
	Email                    string `json:"email"    validate:"required,email"`
	Password                 string `json:"password" validate:"required,min=8"`
	Name                     string `json:"name"     validate:"required"`
	TargetLevel              string `json:"target_level"`
	SchoolName               string `json:"school_name"`
	CertificationsAcquired   string `json:"certifications_acquired"`
	CertificationsInProgress string `json:"certifications_in_progress"`
	RegistrationToken        string `json:"registration_token"`
}

// LoginRequest ログインリクエスト
type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
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
	UserToken                string `json:"user_token,omitempty"`     // ユーザー認証トークン（全ユーザー・短TTL）
	RefreshToken             string `json:"refresh_token,omitempty"`  // リフレッシュトークン (#616)
	EmailVerified            bool   `json:"email_verified"`
	RequiresReVerification   bool   `json:"requires_re_verification,omitempty"`
}
