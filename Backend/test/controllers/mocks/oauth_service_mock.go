package mocks

import (
	"Backend/internal/services"
	"context"

	"github.com/stretchr/testify/mock"
)

type OAuthServiceMock struct {
	mock.Mock
}

func (m *OAuthServiceMock) GetGoogleAuthURL(state string) string {
	return m.Called(state).String(0)
}

func (m *OAuthServiceMock) GetGitHubAuthURL(state string) string {
	return m.Called(state).String(0)
}

func (m *OAuthServiceMock) HandleGoogleCallback(ctx context.Context, code string) (*services.AuthResponse, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AuthResponse), args.Error(1)
}

func (m *OAuthServiceMock) HandleGitHubCallback(ctx context.Context, code string) (*services.AuthResponse, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AuthResponse), args.Error(1)
}
