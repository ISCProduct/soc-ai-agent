package mocks

import (
	"Backend/internal/services/auth"

	"github.com/stretchr/testify/mock"
)

type AuthServiceMock struct {
	mock.Mock
}

func (m *AuthServiceMock) Register(req auth.RegisterRequest) (*auth.AuthResponse, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *AuthServiceMock) Login(req auth.LoginRequest) (*auth.AuthResponse, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *AuthServiceMock) CreateGuestUser() (*auth.AuthResponse, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *AuthServiceMock) GetUser(userID uint) (*auth.AuthResponse, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *AuthServiceMock) UpdateProfile(req auth.UpdateProfileRequest) (*auth.AuthResponse, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *AuthServiceMock) RequestRegistration(email string) error {
	return m.Called(email).Error(0)
}

func (m *AuthServiceMock) ValidateRegistrationToken(token string) (string, error) {
	args := m.Called(token)
	return args.String(0), args.Error(1)
}

func (m *AuthServiceMock) RequestPasswordReset(email string) error {
	return m.Called(email).Error(0)
}

func (m *AuthServiceMock) ResetPassword(token, newPassword string) error {
	return m.Called(token, newPassword).Error(0)
}

func (m *AuthServiceMock) VerifyEmail(token string) error {
	return m.Called(token).Error(0)
}

func (m *AuthServiceMock) DeleteAccount(userID uint) error {
	return m.Called(userID).Error(0)
}

func (m *AuthServiceMock) RefreshSession(refreshToken string) (*auth.AuthResponse, error) {
	args := m.Called(refreshToken)
	if resp, ok := args.Get(0).(*auth.AuthResponse); ok {
		return resp, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *AuthServiceMock) LogoutSession(refreshToken string) error {
	return m.Called(refreshToken).Error(0)
}
