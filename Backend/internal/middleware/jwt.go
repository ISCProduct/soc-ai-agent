package middleware

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const jwtTTL = 30 * 24 * time.Hour

type userClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateJWT はユーザーID・メールアドレス・シークレットからJWTを生成する
func GenerateJWT(userID uint, email, secret string) (string, error) {
	claims := userClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseJWT はJWTを検証してユーザーIDとメールアドレスを返す
func ParseJWT(tokenStr, secret string) (userID uint, email string, err error) {
	token, err := jwt.ParseWithClaims(tokenStr, &userClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return 0, "", err
	}

	claims, ok := token.Claims.(*userClaims)
	if !ok || !token.Valid {
		return 0, "", errors.New("invalid token claims")
	}

	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return 0, "", errors.New("missing subject in token")
	}

	var id uint64
	if _, err := fmt.Sscanf(sub, "%d", &id); err != nil {
		return 0, "", errors.New("invalid subject in token")
	}
	return uint(id), claims.Email, nil
}
