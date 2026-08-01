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

// --- 企業管理系定数 ---

func CompanyTTLInfoDays() int       { return getIntOrDefault("COMPANY_TTL_INFO_DAYS", 90) }
func CompanyTTLJobsDays() int       { return getIntOrDefault("COMPANY_TTL_JOBS_DAYS", 7) }
func CompanyTTLTechDays() int       { return getIntOrDefault("COMPANY_TTL_TECH_DAYS", 30) }
func CompanyTTLRelationsDays() int  { return getIntOrDefault("COMPANY_TTL_RELATIONS_DAYS", 60) }

func MissingBatchDefaultLimit() int { return getIntOrDefault("MISSING_BATCH_DEFAULT_LIMIT", 20) }
func MissingBatchMaxLimit() int     { return getIntOrDefault("MISSING_BATCH_MAX_LIMIT", 50) }
func MissingBatchMaxConcurrency() int { return getIntOrDefault("MISSING_BATCH_MAX_CONCURRENCY", 8) }

func RelationGraphMaxDepth() int { return getIntOrDefault("RELATION_GRAPH_MAX_DEPTH", 4) }
func RelationGraphMaxNodes() int { return getIntOrDefault("RELATION_GRAPH_MAX_NODES", 60) }

func RelationEnrichMaxTargets() int { return getIntOrDefault("RELATION_ENRICH_MAX_TARGETS", 12) }

func ValidationCacheTTLMinutes() int { return getIntOrDefault("VALIDATION_CACHE_TTL_MINUTES", 30) }

func getIntOrDefault(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func DevAllowedOrigins() []string {
	return []string{
		"http://localhost:3000",
		"http://127.0.0.1:3000",
	}
}
