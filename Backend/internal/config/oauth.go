package config

import (
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type OAuthConfig struct {
	Google *oauth2.Config
	GitHub *oauth2.Config
}

func LoadOAuthConfig() *OAuthConfig {
	baseURL := OAuthBaseURL()

	return &OAuthConfig{
		Google: &oauth2.Config{
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  baseURL + "/api/auth/google/callback",
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
		GitHub: &oauth2.Config{
			ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			RedirectURL:  baseURL + "/api/auth/github/callback",
			Scopes: []string{
				"user:email",
				"read:user",
				"repo",     // プライベートリポジトリ・組織リポジトリへのアクセスに必要
				"read:org", // 所属組織のメンバーシップ情報取得に必要
			},
			Endpoint: github.Endpoint,
		},
	}
}
