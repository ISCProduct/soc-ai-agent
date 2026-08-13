package interfaces

import "Backend/internal/services/auth"

type AuthService interface {
	Register(req auth.RegisterRequest, tenantOrgID uint) (*auth.AuthResponse, error)
	Login(req auth.LoginRequest, tenantOrgID uint) (*auth.AuthResponse, error)
	CreateGuestUser(tenantOrgID uint) (*auth.AuthResponse, error)
	GetUser(userID uint) (*auth.AuthResponse, error)
	UpdateProfile(req auth.UpdateProfileRequest) (*auth.AuthResponse, error)
	RequestRegistration(email string) error
	ValidateRegistrationToken(token string) (string, error)
	RequestPasswordReset(email string) error
	ResetPassword(token, newPassword string) error
	VerifyEmail(token string) error
	DeleteAccount(userID uint) error
	RefreshSession(refreshToken string) (*auth.AuthResponse, error)
	LogoutSession(refreshToken string) error
}
