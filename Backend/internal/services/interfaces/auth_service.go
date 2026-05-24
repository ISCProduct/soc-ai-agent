package interfaces

import "Backend/internal/services"

type AuthService interface {
	Register(req services.RegisterRequest) (*services.AuthResponse, error)
	Login(req services.LoginRequest) (*services.AuthResponse, error)
	CreateGuestUser() (*services.AuthResponse, error)
	GetUser(userID uint) (*services.AuthResponse, error)
	UpdateProfile(req services.UpdateProfileRequest) (*services.AuthResponse, error)
	RequestRegistration(email string) error
	ValidateRegistrationToken(token string) (string, error)
	RequestPasswordReset(email string) error
	ResetPassword(token, newPassword string) error
	VerifyEmail(token string) error
	DeleteAccount(userID uint) error
}
