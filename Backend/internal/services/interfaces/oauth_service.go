package interfaces

import (
	"Backend/internal/services"
	"context"
)

type OAuthService interface {
	GetGoogleAuthURL(state string) string
	GetGitHubAuthURL(state string) string
	HandleGoogleCallback(ctx context.Context, code string) (*services.AuthResponse, error)
	HandleGitHubCallback(ctx context.Context, code string) (*services.AuthResponse, error)
}
