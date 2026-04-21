package services

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/config"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"golang.org/x/oauth2"
)

type OAuthService struct {
	userRepo      repository.UserRepository
	oauthConfig   *config.OAuthConfig
	githubService *GitHubService
}

func NewOAuthService(userRepo repository.UserRepository, oauthConfig *config.OAuthConfig, githubService *GitHubService) *OAuthService {
	return &OAuthService{
		userRepo:      userRepo,
		oauthConfig:   oauthConfig,
		githubService: githubService,
	}
}

// GoogleUserInfo Google APIから取得するユーザー情報
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// GitHubUserInfo GitHub APIから取得するユーザー情報
type GitHubUserInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubEmail GitHub APIから取得するメールアドレス情報
type GitHubEmail struct {
	Email      string `json:"email"`
	Primary    bool   `json:"primary"`
	Verified   bool   `json:"verified"`
	Visibility string `json:"visibility"`
}

// GetGoogleAuthURL Google OAuth認証URLを取得
func (s *OAuthService) GetGoogleAuthURL(state string) string {
	return s.oauthConfig.Google.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// GetGitHubAuthURL GitHub OAuth認証URLを取得
func (s *OAuthService) GetGitHubAuthURL(state string) string {
	return s.oauthConfig.GitHub.AuthCodeURL(state)
}

// HandleGoogleCallback Google OAuth認証後のコールバック処理
func (s *OAuthService) HandleGoogleCallback(ctx context.Context, code string) (*AuthResponse, error) {
	token, err := s.oauthConfig.Google.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	// ユーザー情報取得
	client := s.oauthConfig.Google.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user info: %w", err)
	}

	if !userInfo.VerifiedEmail {
		return nil, errors.New("email not verified")
	}

	// 既存ユーザーチェック（OAuth）
	user, err := s.userRepo.GetUserByOAuth("google", userInfo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by oauth: %w", err)
	}

	// 既存ユーザーがいない場合は新規作成
	if user == nil {
		// メールアドレスで既存ユーザーチェック
		existingUser, err := s.userRepo.GetUserByEmail(userInfo.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing user: %w", err)
		}

		if existingUser != nil {
			// 既存ユーザーにOAuth情報を紐付け
			existingUser.OAuthProvider = "google"
			existingUser.OAuthID = userInfo.ID
			existingUser.AvatarURL = userInfo.Picture
			if existingUser.Name == "" {
				existingUser.Name = userInfo.Name
			}
			if err := s.userRepo.UpdateUser(existingUser); err != nil {
				return nil, fmt.Errorf("failed to update user: %w", err)
			}
			promoteAdminIfMatched(existingUser, s.userRepo)
			user = existingUser
		} else {
			// 新規ユーザー作成
			user = &entity.User{
				Email:         userInfo.Email,
				Name:          userInfo.Name,
				OAuthProvider: "google",
				OAuthID:       userInfo.ID,
				AvatarURL:     userInfo.Picture,
				IsGuest:       false,
				TargetLevel:   "未設定",
				SchoolName:    "学校法人岩崎学園情報科学専門学校",
				IsAdmin:       isAdminIdentity(userInfo.Email, userInfo.Name),
			}
			if err := s.userRepo.CreateUser(user); err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}
		}
	}

	return &AuthResponse{
		UserID:                   user.ID,
		Email:                    user.Email,
		Name:                     user.Name,
		IsGuest:                  user.IsGuest,
		TargetLevel:              user.TargetLevel,
		SchoolName:               user.SchoolName,
		IsAdmin:                  user.IsAdmin,
		CertificationsAcquired:   user.CertificationsAcquired,
		CertificationsInProgress: user.CertificationsInProgress,
		AvatarURL:                user.AvatarURL,
		OAuthProvider:            "google",
	}, nil
}

// HandleGitHubCallback GitHub OAuth認証後のコールバック処理
func (s *OAuthService) HandleGitHubCallback(ctx context.Context, code string) (*AuthResponse, error) {
	token, err := s.oauthConfig.GitHub.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	// ユーザー情報取得
	client := s.oauthConfig.GitHub.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var userInfo GitHubUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user info: %w", err)
	}

	// メールアドレスが公開されていない場合は別途取得
	email := userInfo.Email
	if email == "" {
		email, err = s.getGitHubPrimaryEmail(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to get email: %w", err)
		}
	}

	if email == "" {
		return nil, errors.New("email not found")
	}

	// 既存ユーザーチェック（OAuth）
	oauthID := fmt.Sprintf("%d", userInfo.ID)
	user, err := s.userRepo.GetUserByOAuth("github", oauthID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by oauth: %w", err)
	}

	// 既存ユーザーがいない場合は新規作成
	if user == nil {
		// メールアドレスで既存ユーザーチェック
		existingUser, err := s.userRepo.GetUserByEmail(email)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing user: %w", err)
		}

		if existingUser != nil {
			// 既存ユーザーにOAuth情報を紐付け
			existingUser.OAuthProvider = "github"
			existingUser.OAuthID = oauthID
			existingUser.AvatarURL = userInfo.AvatarURL
			if existingUser.Name == "" {
				name := userInfo.Name
				if name == "" {
					name = userInfo.Login
				}
				existingUser.Name = name
			}
			if err := s.userRepo.UpdateUser(existingUser); err != nil {
				return nil, fmt.Errorf("failed to update user: %w", err)
			}
			promoteAdminIfMatched(existingUser, s.userRepo)
			user = existingUser
		} else {
			// 新規ユーザー作成
			name := userInfo.Name
			if name == "" {
				name = userInfo.Login
			}
			user = &entity.User{
				Email:         email,
				Name:          name,
				OAuthProvider: "github",
				OAuthID:       oauthID,
				AvatarURL:     userInfo.AvatarURL,
				IsGuest:       false,
				TargetLevel:   "未設定",
				SchoolName:    "学校法人岩崎学園情報科学専門学校",
				IsAdmin:       isAdminIdentity(email, name),
			}
			if err := s.userRepo.CreateUser(user); err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}
		}
	}

	// アクセストークン保存 & 非同期データ同期
	if s.githubService != nil {
		accessToken := token.AccessToken
		if err := s.githubService.StoreAccessToken(user.ID, userInfo.Login, accessToken); err != nil {
			// トークン保存失敗はログのみ（ログイン自体は成功扱い）
			log.Printf("[OAuthService] failed to store github access token for user %d: %v\n", user.ID, err)
		} else {
			s.githubService.TriggerAsyncSync(user.ID, false)
		}
	}

	return &AuthResponse{
		UserID:                   user.ID,
		Email:                    user.Email,
		Name:                     user.Name,
		IsGuest:                  user.IsGuest,
		TargetLevel:              user.TargetLevel,
		SchoolName:               user.SchoolName,
		IsAdmin:                  user.IsAdmin,
		CertificationsAcquired:   user.CertificationsAcquired,
		CertificationsInProgress: user.CertificationsInProgress,
		AvatarURL:                user.AvatarURL,
		OAuthProvider:            "github",
	}, nil
}

// getGitHubPrimaryEmail GitHubのプライマリメールアドレスを取得
func (s *OAuthService) getGitHubPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var emails []GitHubEmail
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}

	// プライマリかつ検証済みのメールアドレスを探す
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	// プライマリがない場合は検証済みの最初のメールアドレス
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", errors.New("no verified email found")
}
