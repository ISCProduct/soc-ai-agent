package auth

import (
	"Backend/domain/repository"
	"Backend/internal/services/email"
	"Backend/internal/services/refreshtoken"
	"Backend/internal/services/shared"
	"log"

	"gorm.io/gorm"
)

// bcryptCost パスワードハッシュのコストパラメータ（#328）
// OWASP 推奨: 12 以上。DefaultCost(10) は現代の GPU 攻撃に対して不十分。
const bcryptCost = 12

type AuthService struct {
	userRepo         repository.UserRepository
	pendingRepo      repository.PendingRegistrationRepository
	emailService     *email.EmailService
	db               *gorm.DB
	object           ObjectDeleter
	audit            auditRecorder
	deletion         *UserDeletionService
	refreshTokens    *refreshtoken.RefreshTokenService
	jobs             shared.JobEnqueuer
	ownershipClaimer CompanyOwnershipClaimer
	schoolRepo       repository.SchoolRepository
}

// CompanyOwnershipClaimer は本登録時に企業クレームを行う（#754）
type CompanyOwnershipClaimer interface {
	ClaimOwnershipOnRegister(userID uint, companyID, submissionID *uint)
}

func NewAuthService(userRepo repository.UserRepository, pendingRepo repository.PendingRegistrationRepository, emailService *email.EmailService) *AuthService {
	return &AuthService{userRepo: userRepo, pendingRepo: pendingRepo, emailService: emailService}
}

// SetJobEnqueuer はメール送信等の非同期ジョブ投入先を設定する（#617）。
func (s *AuthService) SetJobEnqueuer(j shared.JobEnqueuer) {
	s.jobs = j
}

// SetCompanyOwnershipClaimer は企業クレーム処理を注入する
func (s *AuthService) SetCompanyOwnershipClaimer(c CompanyOwnershipClaimer) {
	s.ownershipClaimer = c
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
func (s *AuthService) SetRefreshTokenService(rts *refreshtoken.RefreshTokenService) {
	s.refreshTokens = rts
}

// SetSchoolRepo は個別校の自動解決に使うリポジトリを注入する。未設定なら解決をスキップする。
func (s *AuthService) SetSchoolRepo(repo repository.SchoolRepository) {
	s.schoolRepo = repo
}

// resolveSchoolID は school_name から school_id をベストエフォートで解決する。
// 完全一致のあと正規化した表記ゆれでも照合する。一致しない・未設定・エラー時は nil。
func (s *AuthService) resolveSchoolID(schoolName string) *uint {
	if s.schoolRepo == nil || schoolName == "" {
		return nil
	}
	school, err := s.schoolRepo.FindByName(schoolName)
	if err != nil || school == nil {
		return nil
	}
	return &school.ID
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
