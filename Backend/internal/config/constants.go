package config

import (
	"os"
	"strconv"
	"time"
)

const (
	DefaultOAuthBaseURL     = "http://localhost:8080"
	DefaultAppURL           = "http://localhost:3000"
	DefaultGuestEmailDomain = "temp.local"
	// DefaultSchoolName はテナント（学園サブドメイン）が解決できない場合のフォールバック。
	// 特定の実在校名を入れると無関係なユーザーにその学校が紐付いて見えるため空にする。
	// 学園専用インスタンスとして固定したい場合のみ DEFAULT_SCHOOL_NAME 環境変数で明示的に設定する。
	DefaultSchoolName              = ""
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

func CompanyTTLInfoDays() int      { return getIntOrDefault("COMPANY_TTL_INFO_DAYS", 90) }
func CompanyTTLJobsDays() int      { return getIntOrDefault("COMPANY_TTL_JOBS_DAYS", 7) }
func CompanyTTLTechDays() int      { return getIntOrDefault("COMPANY_TTL_TECH_DAYS", 30) }
func CompanyTTLRelationsDays() int { return getIntOrDefault("COMPANY_TTL_RELATIONS_DAYS", 60) }

func MissingBatchDefaultLimit() int { return getIntOrDefault("MISSING_BATCH_DEFAULT_LIMIT", 30) }

func MissingBatchMaxLimit() int { return getIntOrDefault("MISSING_BATCH_MAX_LIMIT", 50) }

// ResumeCompletenessThreshold は履歴書リマインダーの閾値(1-100)。
// 最新レビューのスコアがこの値未満なら「要対応」と判定する(#1030)。
//
// 0以下・数値以外は既定値に落ちるため、閾値0によるリマインダー全停止はできない。
// 停止したい場合は100超を指定するのではなく(100超は全員が要対応になる)、
// 範囲外を100へ丸めたうえで機能フラグを別途用意すること。
func ResumeCompletenessThreshold() int {
	v := getIntOrDefault("RESUME_COMPLETENESS_THRESHOLD", 60)
	if v > 100 {
		return 100
	}
	return v
}

// MissingBatchMaxConcurrency は企業間並列の上限。既定8は Fargate 0.25vCPU/512MB と OpenAI RPM の天井。
func MissingBatchMaxConcurrency() int { return getIntOrDefault("MISSING_BATCH_MAX_CONCURRENCY", 8) }

func RelationGraphMaxDepth() int { return getIntOrDefault("RELATION_GRAPH_MAX_DEPTH", 4) }
func RelationGraphMaxNodes() int { return getIntOrDefault("RELATION_GRAPH_MAX_NODES", 60) }

func RelationEnrichMaxTargets() int { return getIntOrDefault("RELATION_ENRICH_MAX_TARGETS", 12) }

func ValidationCacheTTLMinutes() int { return getIntOrDefault("VALIDATION_CACHE_TTL_MINUTES", 30) }

// MatchingReasonAITopN は AI マッチング理由を生成する上位件数（#1061）。
// 表示側は match_score 降順の上位のみ読むため（GetTopMatches の既定10件、レポート経路は5件）、
// 全公開企業ぶん生成しても大半が捨てられる。limit クエリでの上振れを吸収して 20 を既定とする。
func MatchingReasonAITopN() int { return getIntOrDefault("MATCHING_REASON_AI_TOP_N", 20) }

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
