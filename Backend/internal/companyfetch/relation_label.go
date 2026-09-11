package companyfetch

import "strings"

// DefaultRelationDescription は relation_type から図・一覧に表示する既定ラベルを返す。
// 種別の表示用であり、取引内容の代替テキストとして DB に保存してはいけない。
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

// IsGenericRelationLabel は関係種別のラベルだけで、取引内容ではない文字列か。
func IsGenericRelationLabel(desc string) bool {
	switch strings.TrimSpace(desc) {
	case "子会社", "関連会社", "主要取引先", "取引先", "調達・契約", "補助金連携",
		"ビジネス関係", "資本関係", "関連", "調達（gBiz）", "補助金（gBiz）":
		return true
	default:
		return false
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

// SanitizeRelationDescription は DB 保存用に、実質的な関係・取引内容だけを残す。
// 取得元タグや「主要取引先」などの種別ラベルだけの場合は空文字にする。
func SanitizeRelationDescription(description string) string {
	desc := strings.TrimSpace(description)
	if desc == "" || IsSourceTagDescription(desc) || IsGenericRelationLabel(desc) {
		return ""
	}
	return desc
}

// NormalizeRelationDescription は図・一覧の表示用ラベルを返す。
// 実取引内容があればそれを優先し、無ければ関係種別の既定ラベルを使う。
func NormalizeRelationDescription(description, relationType string) string {
	if desc := SanitizeRelationDescription(description); desc != "" {
		return desc
	}
	return DefaultRelationDescription(relationType)
}
