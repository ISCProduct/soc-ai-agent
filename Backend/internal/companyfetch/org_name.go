package companyfetch

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// 法人格付きでも本体名が無い・曖昧な表記
	unclearOrgExact = map[string]struct{}{
		"不明": {}, "未定": {}, "その他": {}, "なし": {}, "無し": {}, "該当なし": {},
		"n/a": {}, "na": {}, "-": {}, "—": {}, "－": {},
		"複数社": {}, "各社": {}, "関係会社": {}, "グループ会社": {}, "グループ": {},
		"共同企業体": {}, "特定共同企業体": {}, "コンソーシアム": {}, "jv": {}, "ｊｖ": {},
		"株式会社": {}, "有限会社": {}, "合同会社": {}, "合資会社": {}, "合名会社": {},
		"一般社団法人": {}, "一般財団法人": {}, "公益社団法人": {}, "公益財団法人": {},
		"主要取引先": {}, "取引先": {},
	}
	unclearOrgContains = []string{
		"不明", "未定", "その他多数", "ほか数社", "他数社", "等数社",
		"共同企業体", "特定共同企業体",
	}
	corpPrefixRe = regexp.MustCompile(`^(株式会社|有限会社|合同会社|合資会社|合名会社|一般社団法人|一般財団法人|公益社団法人|公益財団法人)\s*`)
	corpSuffixRe = regexp.MustCompile(`\s*(株式会社|有限会社|合同会社|合資会社|合名会社|Inc\.?|Ltd\.?|LLC|Corp\.?)$`)
)

// IsClearOrganizationName は関係先として保存してよい、はっきりした組織名か。
// 曖昧・プレースホルダ・法人格のみ・短すぎる名前は false。
func IsClearOrganizationName(name string) bool {
	n := strings.TrimSpace(name)
	if n == "" {
		return false
	}
	lower := strings.ToLower(n)
	if _, ok := unclearOrgExact[lower]; ok {
		return false
	}
	if _, ok := unclearOrgExact[n]; ok {
		return false
	}
	for _, frag := range unclearOrgContains {
		if strings.Contains(n, frag) {
			return false
		}
	}

	core := strings.TrimSpace(corpPrefixRe.ReplaceAllString(n, ""))
	core = strings.TrimSpace(corpSuffixRe.ReplaceAllString(core, ""))
	core = strings.TrimSpace(core)
	if core == "" {
		return false
	}
	if _, ok := unclearOrgExact[strings.ToLower(core)]; ok {
		return false
	}
	if utf8.RuneCountInString(core) < 2 {
		return false
	}
	// 記号・数字だけの名前は除外
	letters := 0
	for _, r := range core {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			(r >= 0x3040 && r <= 0x30ff) || (r >= 0x4e00 && r <= 0x9fff) || (r >= 0xff66 && r <= 0xff9d) {
			letters++
		}
	}
	return letters >= 2
}
