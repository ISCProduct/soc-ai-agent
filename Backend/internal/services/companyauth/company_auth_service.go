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

const bcryptCost = 12

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInviteNotFound     = errors.New("invalid invite token")
	ErrInviteExpired      = errors.New("invite token expired")
	ErrEmailExists        = errors.New("email already exists")
	ErrCompanyNotFound    = errors.New("company not found")
	ErrCompanyNotVerified = errors.New("company is not verified")
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
	if existing != nil {
		return nil, ErrEmailExists
	}

	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate invite token: %w", err)
	}
	inviteToken := base64.URLEncoding.EncodeToString(tokenBytes)
	expires := s.now().Add(config.PendingRegistrationTokenTTL)

	user := &models.CompanyUser{
		CompanyID:       companyID,
		Email:           emailAddr,
		Name:            name,
		Role:            role,
		InviteToken:     &inviteToken,
		InviteExpiresAt: &expires,
	}
	if err := s.users.Create(user); err != nil {
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

	user, err := s.users.FindByInviteToken(token)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInviteNotFound
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
	user.InviteToken = nil
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
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
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
		ExpiresAt:     s.now().Add(30 * 24 * time.Hour),
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
