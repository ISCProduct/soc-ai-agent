package interfaces

import (
	"Backend/internal/services/auth"
	"context"
)

type OAuthService interface {
	GetGoogleAuthURL(state string) string
	GetGitHubAuthURL(state string) string
	HandleGoogleCallback(ctx context.Context, code string, tenantOrgID uint) (*auth.AuthResponse, error)
	HandleGitHubCallback(ctx context.Context, code string, tenantOrgID uint) (*auth.AuthResponse, error)
}
