package companyfetch

import (
	"os"
	"strings"
)

const (
	defaultExtractModel = "gpt-4o-mini"
	// Responses API の web_search + mini。Chat Completions の search-preview / search-api は使わない。
	defaultSearchModel     = "gpt-4o-mini"
	defaultDeepSearchModel = "gpt-4o-mini"
	defaultParseModel      = "gpt-4o-mini"
	defaultParseAdvanced   = "gpt-4o"
)

// ExtractModel はスクレイプ/Search結果からの JSON 抽出モデル。
func ExtractModel() string {
	return envOr("OPENAI_COMPANY_EXTRACT_MODEL", defaultExtractModel)
}

// SearchModel は軽量 Web 検索モデル。
func SearchModel() string {
	return envOr("OPENAI_COMPANY_SEARCH_MODEL", defaultSearchModel)
}

// DeepSearchModel は不足時の本格 Web 検索モデル。
func DeepSearchModel() string {
	return envOr("OPENAI_COMPANY_DEEP_SEARCH_MODEL", defaultDeepSearchModel)
}

// ParseModel は Search 結果の構造化モデル。
func ParseModel() string {
	return envOr("OPENAI_COMPANY_PARSE_MODEL", defaultParseModel)
}

// ParseModelAdvanced は長文向け高精度 Parse モデル。
func ParseModelAdvanced() string {
	return envOr("OPENAI_COMPANY_PARSE_MODEL_ADVANCED", defaultParseAdvanced)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
