package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateAdminToken はユーザーID・メールアドレス・シークレットからHMAC-SHA256トークンを生成する
func GenerateAdminToken(userID uint, email, secret string) string {
	payload := fmt.Sprintf("%d:%s", userID, email)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyAdminToken はトークンがユーザーID・メールアドレス・シークレットと一致するか検証する
func VerifyAdminToken(token string, userID uint, email, secret string) bool {
	expected := GenerateAdminToken(userID, email, secret)
	return hmac.Equal([]byte(token), []byte(expected))
}
