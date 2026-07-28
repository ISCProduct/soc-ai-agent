package companyfetch

import "strings"

// DefaultRelationDescription は relation_type から図・一覧に表示する既定ラベルを返す。
func DefaultRelationDescription(relationType string) string {
	switch strings.TrimSpace(relationType) {
	case "capital_subsidiary":
		return "子会社"
	case "capital_affiliate":
		return "関連会社"
	case "business_partner":
		return "主要取引先"
	case "business_procurement":
		return "調達・契約"
	case "business_subsidy":
		return "補助金連携"
	default:
		if strings.HasPrefix(relationType, "business_") {
			return "ビジネス関係"
		}
		if strings.HasPrefix(relationType, "capital_") {
			return "資本関係"
		}
		return "関連"
	}
}

// IsSourceTagDescription は取得元タグのプレースホルダ説明かどうかを返す。
// 例: web_search:株式会社サンプル, llm_web_search:サンプル
func IsSourceTagDescription(desc string) bool {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return true
	}
	idx := strings.Index(desc, ":")
	if idx <= 0 {
		return false
	}
	tag := strings.ToLower(strings.TrimSpace(desc[:idx]))
	switch tag {
	case SourceWebSearch, "llm_web_search", SourceGBiz, SourceScrape, SourceLLMExtract, "official", "manual", "job_site", "scraping":
		return true
	default:
		return false
	}
}

// NormalizeRelationDescription は表示用の関係説明を返す。
func NormalizeRelationDescription(description, relationType string) string {
	desc := strings.TrimSpace(description)
	if desc != "" && !IsSourceTagDescription(desc) {
		return desc
	}
	return DefaultRelationDescription(relationType)
}
