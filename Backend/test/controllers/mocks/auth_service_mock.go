package mocks

import (
	"Backend/internal/services"

	"github.com/stretchr/testify/mock"
)

type AuthServiceMock struct {
	mock.Mock
}

func (m *AuthServiceMock) Register(req services.RegisterRequest, tenantOrgID uint) (*services.AuthResponse, error) {
	args := m.Called(req, tenantOrgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AuthResponse), args.Error(1)
}

func (m *AuthServiceMock) Login(req services.LoginRequest, tenantOrgID uint) (*services.AuthResponse, error) {
	args := m.Called(req, tenantOrgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AuthResponse), args.Error(1)
}

func (m *AuthServiceMock) CreateGuestUser(tenantOrgID uint) (*services.AuthResponse, error) {
	args := m.Called(tenantOrgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AuthResponse), args.Error(1)
}

func (m *AuthServiceMock) GetUser(userID uint) (*services.AuthResponse, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AuthResponse), args.Error(1)
}

func (m *AuthServiceMock) UpdateProfile(req services.UpdateProfileRequest) (*services.AuthResponse, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AuthResponse), args.Error(1)
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

func (m *AuthServiceMock) RefreshSession(refreshToken string) (*services.AuthResponse, error) {
	args := m.Called(refreshToken)
	if resp, ok := args.Get(0).(*services.AuthResponse); ok {
		return resp, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *AuthServiceMock) LogoutSession(refreshToken string) error {
	return m.Called(refreshToken).Error(0)
}
