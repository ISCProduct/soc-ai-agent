package services

import (
	"Backend/internal/crypto"
	"Backend/internal/models"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
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

func encryptGitHubToken(token string) (string, error) {
	key, err := getGitHubTokenEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(token), nil)
	payload := append(nonce, sealed...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func decryptGitHubToken(encrypted string) (string, error) {
	key, err := getGitHubTokenEncryptionKey()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("encrypted token payload too short")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func getGitHubTokenEncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(githubTokenEncryptionKeyEnv))
	if raw == "" {
		return nil, fmt.Errorf("%s is required", githubTokenEncryptionKeyEnv)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", githubTokenEncryptionKeyEnv, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%s must be base64-encoded 32-byte key", githubTokenEncryptionKeyEnv)
	}
	return key, nil
}
