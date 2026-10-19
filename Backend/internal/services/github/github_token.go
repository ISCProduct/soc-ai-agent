package github

import (
	"Backend/internal/crypto"
	"Backend/internal/models"
	"fmt"
	"log"
	"os"
)

// StoreAccessToken GitHubアクセストークンを AES-GCM で暗号化してDBに保存する（#326）
func (s *GitHubService) StoreAccessToken(userID uint, login, accessToken string) error {
	encryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
	storedToken := accessToken
	if encryptionKey != "" {
		encrypted, err := crypto.EncryptToken(accessToken, encryptionKey)
		if err != nil {
			log.Printf("[GitHubService] failed to encrypt access token for user %d: %v", userID, err)
			return fmt.Errorf("failed to encrypt access token: %w", err)
		}
		storedToken = encrypted
	} else {
		log.Printf("[GitHubService] WARNING: TOKEN_ENCRYPTION_KEY not set, storing access token in plaintext for user %d", userID)
	}
	profile := &models.GitHubProfile{
		UserID:      userID,
		GitHubLogin: login,
		AccessToken: storedToken,
	}
	return s.githubRepo.UpsertProfile(profile)
}

// loadAccessToken StoreAccessToken と対になる復号。鍵未設定時は平文保存として扱う（#326）
func loadAccessToken(userID uint, stored string) (string, error) {
	encryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if encryptionKey == "" {
		return stored, nil
	}
	decrypted, err := crypto.DecryptToken(stored, encryptionKey)
	if err != nil {
		log.Printf("[GitHubService] failed to decrypt access token for user %d: %v", userID, err)
		return "", &GitHubReauthRequiredError{Reason: "GitHubアクセストークンの復号に失敗しました。GitHubアカウントを再連携してください。"}
	}
	return decrypted, nil
}
