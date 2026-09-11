package companyauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"Backend/internal/config"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/email"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	bcryptCost = 12
	// refreshTokenTTL はリフレッシュトークンの有効期間
	refreshTokenTTL = 30 * 24 * time.Hour
	// rotationGracePeriod はローテーション直後の旧トークンを許容する猶予。
	// 並行リクエストが同時にリフレッシュした場合のログアウト事故を防ぐ。
	rotationGracePeriod = 60 * time.Second
)

var (
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInviteNotFound      = errors.New("invalid invite token")
	ErrInviteExpired       = errors.New("invite token expired")
	ErrEmailExists         = errors.New("email already exists")
	ErrCompanyNotFound     = errors.New("company not found")
	ErrCompanyNotVerified  = errors.New("company is not verified")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrAccountDisabled     = errors.New("account is disabled")
	ErrResetTokenInvalid   = errors.New("invalid password reset token")
	ErrResetTokenExpired   = errors.New("password reset token expired")
	ErrUserNotFound        = errors.New("company user not found")
)

type InviteRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AcceptInviteRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type AuthResponse struct {
	CompanyUserID uint   `json:"company_user_id"`
	CompanyID     uint   `json:"company_id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Role          string `json:"role"`
	Token         string `json:"token"`
	RefreshToken  string `json:"refresh_token,omitempty"`
}

type CompanyUserService struct {
	users   *repositories.CompanyUserRepository
	refresh *repositories.CompanyUserRefreshTokenRepository
	email   *email.EmailService
	db      *gorm.DB
	secret  string
	now     func() time.Time
}

func NewCompanyUserService(
	db *gorm.DB,
	users *repositories.CompanyUserRepository,
	refresh *repositories.CompanyUserRefreshTokenRepository,
	emailSvc *email.EmailService,
	secret string,
) *CompanyUserService {
	return &CompanyUserService{
		users:   users,
		refresh: refresh,
		email:   emailSvc,
		db:      db,
		secret:  secret,
		now:     time.Now,
	}
}

func (s *CompanyUserService) Invite(companyID uint, req InviteRequest) (*models.CompanyUser, error) {
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)
	role := strings.TrimSpace(req.Role)
	if emailAddr == "" || name == "" {
		return nil, errors.New("email and name are required")
	}
	if _, err := mail.ParseAddress(emailAddr); err != nil {
		return nil, errors.New("email is invalid")
	}
	if role == "" {
		role = models.CompanyUserRoleMember
	}
	if role != models.CompanyUserRoleOwner && role != models.CompanyUserRoleMember {
		return nil, errors.New("invalid role")
	}

	var company models.Company
	if err := s.db.First(&company, companyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCompanyNotFound
		}
		return nil, err
	}
	if !company.IsVerified {
		return nil, ErrCompanyNotVerified
	}

	existing, err := s.users.FindByEmail(emailAddr)
	if err != nil {
		return nil, err
	}
	// パスワード設定済み＝既に使われているアカウント。重複作成は許さない。
	// 復旧が必要な場合はパスワードリセットを使う（#1196 以前はここで詰んでいた）。
	if existing != nil && existing.PasswordSet() {
		return nil, ErrEmailExists
	}
	// 招待メールを紛失した等で受諾前のまま残っているアカウントは、
	// トークンを発行し直して再招待できるようにする。
	if existing != nil && existing.CompanyID != companyID {
		return nil, ErrEmailExists
	}

	inviteToken, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate invite token: %w", err)
	}
	tokenHash := hashToken(inviteToken)
	expires := s.now().Add(config.PendingRegistrationTokenTTL)

	user := existing
	if user == nil {
		user = &models.CompanyUser{CompanyID: companyID, Email: emailAddr}
	}
	user.Name = name
	user.Role = role
	user.InviteTokenHash = &tokenHash
	user.InviteExpiresAt = &expires
	// 再招待は無効化されたアカウントの復帰も兼ねる。
	user.DisabledAt = nil

	if user.ID == 0 {
		if err := s.users.Create(user); err != nil {
			return nil, err
		}
	} else if err := s.users.Update(user); err != nil {
		return nil, err
	}

	if s.email != nil {
		if err := s.email.SendCompanyUserInvite(emailAddr, company.Name, inviteToken); err != nil {
			log.Printf("[CompanyUserService] invite email failed company_user_id=%d error=%v", user.ID, err)
		}
	}
	return user, nil
}

func (s *CompanyUserService) AcceptInvite(req AcceptInviteRequest) (*AuthResponse, error) {
	token := strings.TrimSpace(req.Token)
	password := req.Password
	name := strings.TrimSpace(req.Name)
	if token == "" || password == "" {
		return nil, errors.New("token and password are required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	user, err := s.users.FindByInviteTokenHash(hashToken(token))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInviteNotFound
	}
	if user.Disabled() {
		return nil, ErrAccountDisabled
	}
	if user.InviteExpiresAt != nil && s.now().After(*user.InviteExpiresAt) {
		return nil, ErrInviteExpired
	}
	if user.PasswordSet() {
		return nil, errors.New("invite already accepted")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}
	user.Password = string(hashed)
	user.InviteTokenHash = nil
	user.InviteExpiresAt = nil
	if name != "" {
		user.Name = name
	}
	if err := s.users.Update(user); err != nil {
		return nil, err
	}
	return s.buildAuthResponse(user, true)
}

func (s *CompanyUserService) Login(req LoginRequest) (*AuthResponse, error) {
	emailAddr := strings.TrimSpace(req.Email)
	if emailAddr == "" || req.Password == "" {
		return nil, errors.New("email and password are required")
	}

	user, err := s.users.FindByEmail(emailAddr)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.PasswordSet() {
		return nil, ErrInvalidCredentials
	}
	// パスワード照合より先に無効化を返すと、無効なアカウントの存在が分かってしまう。
	// 資格情報が正しい場合にだけ「無効化されている」と伝える。
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if user.Disabled() {
		return nil, ErrAccountDisabled
	}
	return s.buildAuthResponse(user, true)
}

func (s *CompanyUserService) GetMe(companyUserID uint) (*AuthResponse, error) {
	user, err := s.users.FindByID(companyUserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	return s.buildAuthResponse(user, false)
}

func (s *CompanyUserService) ListByCompany(companyID uint) ([]models.CompanyUser, error) {
	return s.users.ListByCompanyID(companyID)
}

func (s *CompanyUserService) EnsureCompanyAccess(companyUserID, companyID uint) error {
	user, err := s.users.FindByID(companyUserID)
	if err != nil {
		return err
	}
	if user == nil || user.CompanyID != companyID {
		return errors.New("forbidden")
	}
	return nil
}

func (s *CompanyUserService) buildAuthResponse(user *models.CompanyUser, includeRefresh bool) (*AuthResponse, error) {
	if s.secret == "" {
		return nil, errors.New("COMPANY_USER_SECRET is not configured")
	}
	token, err := middleware.GenerateJWT(user.ID, user.Email, s.secret)
	if err != nil {
		return nil, err
	}
	resp := &AuthResponse{
		CompanyUserID: user.ID,
		CompanyID:     user.CompanyID,
		Email:         user.Email,
		Name:          user.Name,
		Role:          user.Role,
		Token:         token,
	}
	if includeRefresh {
		refresh, err := s.issueRefreshToken(user.ID)
		if err != nil {
			log.Printf("[CompanyUserService] refresh token issue failed company_user_id=%d error=%v", user.ID, err)
		} else {
			resp.RefreshToken = refresh
		}
	}
	return resp, nil
}

// RefreshSession はリフレッシュトークンをローテーションして新しいトークンペアを返す (#616踏襲)
func (s *CompanyUserService) RefreshSession(plain string) (*AuthResponse, error) {
	if plain == "" || s.refresh == nil {
		return nil, ErrInvalidRefreshToken
	}
	token, err := s.refresh.FindByHash(hashRefreshToken(plain))
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, ErrInvalidRefreshToken
	}
	now := s.now()
	if now.After(token.ExpiresAt) {
		return nil, ErrInvalidRefreshToken
	}
	if token.RevokedAt != nil && now.Sub(*token.RevokedAt) > rotationGracePeriod {
		return nil, ErrInvalidRefreshToken
	}
	if token.RevokedAt == nil {
		if err := s.refresh.Revoke(token.ID, now); err != nil {
			return nil, err
		}
	}

	user, err := s.users.FindByID(token.CompanyUserID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.PasswordSet() {
		return nil, ErrInvalidRefreshToken
	}
	// 無効化されたアカウントには新しいトークンを発行しない（#1196）。
	// SetDisabled は既存のリフレッシュトークンを失効させるが、
	// rotationGracePeriod(60秒)の間は失効済みトークンでもここまで到達する。
	// ここで止めないと、無効化直後にリフレッシュした利用者へ
	// 新しいJWTと未失効のリフレッシュトークンが渡り、無効化を恒久的に迂回できる。
	if user.Disabled() {
		return nil, ErrInvalidRefreshToken
	}
	return s.buildAuthResponse(user, true)
}

// LogoutSession はリフレッシュトークンを削除して即座に無効化する（見つからなくてもエラーにしない）。
// Revoke ではなく Delete を使うのは、Revoke のローテーション猶予期間（並行リフレッシュ対策）が
// 明示的なログアウトにも適用され、ログアウト直後の一定時間トークンが使えてしまうのを防ぐため。
func (s *CompanyUserService) LogoutSession(plain string) error {
	if plain == "" || s.refresh == nil {
		return nil
	}
	token, err := s.refresh.FindByHash(hashRefreshToken(plain))
	if err != nil {
		return err
	}
	if token == nil {
		return nil
	}
	return s.refresh.Delete(token.ID)
}

func (s *CompanyUserService) issueRefreshToken(companyUserID uint) (string, error) {
	if s.refresh == nil {
		return "", nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	plain := hex.EncodeToString(buf)
	hash := hashRefreshToken(plain)
	token := &models.CompanyUserRefreshToken{
		CompanyUserID: companyUserID,
		TokenHash:     hash,
		ExpiresAt:     s.now().Add(refreshTokenTTL),
	}
	if err := s.refresh.Create(token); err != nil {
		return "", err
	}
	return plain, nil
}

func hashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// hashToken は招待・パスワードリセットのトークンをDBに保存する形へ変換する。
// 平文はメールでしか流通させない（#1196）。
func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
