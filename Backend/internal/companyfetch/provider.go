package companyfetch

import (
	"Backend/internal/openai"
	"context"
	"fmt"
	"os"
	"strings"
)

// CompanySearchProvider は企業情報 Write 用の Web 検索抽象（#590）。
// OpenAI Search Lite / Brave 等を差し替え可能にする。
type CompanySearchProvider interface {
	Name() string
	Search(ctx context.Context, query string, maxTokens int) (text string, providerID string, err error)
}

// NewSearchProviderFromEnv は COMPANY_SEARCH_PROVIDER に従い Provider を生成する。
// 未設定・不明値は openai。brave でキー未設定の場合はエラーを返す Provider ではなく openai にフォールバックし警告ログ相当の文言を Name に残す。
func NewSearchProviderFromEnv(client *openai.Client) CompanySearchProvider {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("COMPANY_SEARCH_PROVIDER")))
	switch name {
	case "brave":
		key := strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY"))
		if key == "" {
			// キー無しでは Brave を起動できないため OpenAI にフォールバック
			return NewOpenAISearchProvider(client)
		}
		return NewBraveSearchProvider(key, client)
	default:
		return NewOpenAISearchProvider(client)
	}
}

// ResolveSearchProviderName は実際に選ばれる Provider 名を返す（診断用）。
func ResolveSearchProviderName() string {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("COMPANY_SEARCH_PROVIDER")))
	if name == "brave" && strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY")) != "" {
		return "brave"
	}
	if name == "brave" {
		return "openai(fallback:brave_key_missing)"
	}
	return "openai"
}

// ensureProvider は LLM に Search が無いとき OpenAI を返す。
func (l *LLM) ensureProvider() CompanySearchProvider {
	if l == nil {
		return nil
	}
	if l.Search != nil {
		return l.Search
	}
	if l.Client == nil {
		return nil
	}
	return NewOpenAISearchProvider(l.Client)
}

func providerSearchError(provider string, err error) error {
	return fmt.Errorf("%s search failed: %w", provider, err)
}
