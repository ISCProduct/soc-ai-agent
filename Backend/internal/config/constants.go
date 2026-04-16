package config

import (
	"os"
	"strconv"
	"time"
)

const (
	DefaultOAuthBaseURL            = "http://localhost:8080"
	DefaultAppURL                  = "http://localhost:3000"
	DefaultGuestEmailDomain        = "temp.local"
	DefaultSchoolName              = "学校法人岩崎学園情報科学専門学校"
	DefaultCompanyGraphThreshold   = 0.75
	PendingRegistrationTokenTTL    = 24 * time.Hour
	ReVerificationInactiveDuration = 10 * 24 * time.Hour
	PasswordResetTokenTTL          = time.Hour
)

func OAuthBaseURL() string {
	return get("BASE_URL", DefaultOAuthBaseURL)
}

func AppURL() string {
	return get("APP_URL", DefaultAppURL)
}

func GuestEmailDomain() string {
	return get("GUEST_EMAIL_DOMAIN", DefaultGuestEmailDomain)
}

func SchoolName() string {
	return get("DEFAULT_SCHOOL_NAME", DefaultSchoolName)
}

func CompanyGraphThreshold() float64 {
	raw := os.Getenv("COMPANY_GRAPH_THRESHOLD")
	if raw == "" {
		return DefaultCompanyGraphThreshold
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return DefaultCompanyGraphThreshold
	}
	return value
}

func DevAllowedOrigins() []string {
	return []string{
		"http://localhost:3000",
		"http://127.0.0.1:3000",
	}
}
