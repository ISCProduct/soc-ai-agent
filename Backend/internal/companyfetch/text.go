package companyfetch

import (
	"Backend/internal/companyfields"
	"Backend/internal/config"
	"context"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	TTLInfo      = 90 * 24 * time.Hour
	TTLJobs      = 7 * 24 * time.Hour
	TTLTech      = 30 * 24 * time.Hour
	TTLRelations = 60 * 24 * time.Hour
)

func init() {
	TTLInfo = time.Duration(config.CompanyTTLInfoDays()) * 24 * time.Hour
	TTLJobs = time.Duration(config.CompanyTTLJobsDays()) * 24 * time.Hour
	TTLTech = time.Duration(config.CompanyTTLTechDays()) * 24 * time.Hour
	TTLRelations = time.Duration(config.CompanyTTLRelationsDays()) * 24 * time.Hour
}

const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"

	SourceScrape     = "scrape"
	SourceWebSearch  = "web_search"
	SourceGBiz       = "gbizinfo"
	SourceLLMExtract = "llm_extract" // WebSearchなしの安価抽出（鮮度保証なし）

	defaultMaxTextRunes = 1500
	defaultMinTextRunes = 80
	maxRedirects        = 5
)

// allowPrivateURLs は単体テスト専用。本番では常に false。
var allowPrivateURLs bool

var (
	scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	tagRe    = regexp.MustCompile(`(?is)<[^>]+>`)
	spaceRe  = regexp.MustCompile(`\s+`)
)

// IsFresh は fetchedAt が TTL 以内なら true。
// フィールド TTL 判定には対応する *_fetched_at のみを渡し、SourceFetchedAt / GBizLastSyncedAt は使わない。
func IsFresh(fetchedAt *time.Time, ttl time.Duration) bool {
	if fetchedAt == nil {
		return false
	}
	return time.Since(*fetchedAt) < ttl
}

// IsEmptyTechPayload は tech_stack が未設定／空配列相当なら true。
func IsEmptyTechPayload(techStack string) bool {
	t := strings.TrimSpace(techStack)
	return t == "" || t == "[]" || t == "null" || t == "{}"
}

// HasTechData は IT 向け: tech_stack に実データがあるか。
// infra / CI/CD / 開発手法だけ埋まっていても「技術取得済み」とはみなさない。
// 業界別判定は HasTechDataForIndustry を使うこと。
func HasTechData(techStack, infraStack, cicdTools, developmentStyle string) bool {
	return !IsEmptyTechPayload(techStack)
}

// HasTechDataForIndustry は業界プロファイルに応じた技術・専門情報の充足判定。
// 技術タブ非対象業種は常に true（不足扱いにしない）。
func HasTechDataForIndustry(industry, techStack, infraStack, cicdTools, developmentStyle string) bool {
	profile := companyfields.Resolve(industry)
	if !profile.TechAspectEnabled {
		return true
	}
	switch profile.ID {
	case companyfields.ProfileManufacturing:
		// 製造業: 主要技術・設備・生産方式のいずれかがあれば充足
		return !IsEmptyTechPayload(techStack) ||
			!IsEmptyTechPayload(infraStack) ||
			strings.TrimSpace(developmentStyle) != ""
	default:
		// IT など: 言語・フレームワーク（tech_stack）必須
		return HasTechData(techStack, infraStack, cicdTools, developmentStyle)
	}
}

// HasBasicInfo は公開向けに十分な基本情報（概要・公式URL）があるか。
func HasBasicInfo(description, websiteURL string) bool {
	return strings.TrimSpace(description) != "" && strings.TrimSpace(websiteURL) != ""
}

// HasBasicInfoFootprint は取得フロー上の手がかり（概要+URL、または公式URL/所在地）があるか。
// 非上場などで概要が公開事実として取れない場合でも、再取得ループを止める判定に使う。
func HasBasicInfoFootprint(description, websiteURL, location string) bool {
	if HasBasicInfo(description, websiteURL) {
		return true
	}
	return strings.TrimSpace(websiteURL) != "" || strings.TrimSpace(location) != ""
}

// HasMeaningfulMarketInfo は市場区分が確定しているか（上場・非上場どちらも含む）。
// unlisted 確定も取得成功とみなし、RelationsFetchedAt のスタンプ対象にする。
// 再取得は force / TTL 切れで行う。
func HasMeaningfulMarketInfo(isListed bool, marketType, stockCode string) bool {
	if isListed || strings.TrimSpace(stockCode) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(marketType)) {
	case "prime", "standard", "growth", "unlisted":
		return true
	default:
		return false
	}
}

// IsConfirmedUnlisted は証券コードなしの非上場確定か。
func IsConfirmedUnlisted(isListed bool, marketType, stockCode string) bool {
	if isListed || strings.TrimSpace(stockCode) != "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(marketType), "unlisted")
}

// NormalizeHTMLText は HTML をプレーンテキストへ正規化する。
func NormalizeHTMLText(rawHTML string) string {
	clean := scriptRe.ReplaceAllString(rawHTML, " ")
	clean = styleRe.ReplaceAllString(clean, " ")
	clean = tagRe.ReplaceAllString(clean, " ")
	clean = html.UnescapeString(clean)
	clean = strings.ReplaceAll(clean, "\u00a0", " ")
	clean = spaceRe.ReplaceAllString(clean, " ")
	return strings.TrimSpace(clean)
}

// TrimText は抽出用にテキストを maxRunes 字以内へ切り詰める。
func TrimText(text string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = defaultMaxTextRunes
	}
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxRunes])
}

// ValidatePublicHTTPURL は SSRF 対策のため http(s) かつ非プライベート宛先のみ許可する。
func ValidatePublicHTTPURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("url is empty")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("only http/https urls are allowed")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("url host is empty")
	}
	if !allowPrivateURLs {
		if host == "localhost" || strings.HasSuffix(strings.ToLower(host), ".localhost") {
			return fmt.Errorf("requests to localhost are not allowed")
		}
		if ip := net.ParseIP(host); ip != nil {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
				return fmt.Errorf("requests to internal ip addresses are not allowed")
			}
		}
	}
	return nil
}

// FetchURLText は URL から HTML を取得し、正規化・トリムしたテキストを返す。
func FetchURLText(ctx context.Context, rawURL string) (string, error) {
	if err := ValidatePublicHTTPURL(rawURL); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SocAI/1.0; +https://example.com/bot)")
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if err := ValidatePublicHTTPURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch failed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", err
	}
	text := NormalizeHTMLText(string(body))
	if utf8.RuneCountInString(text) < defaultMinTextRunes {
		return "", fmt.Errorf("page text too short")
	}
	return TrimText(text, defaultMaxTextRunes), nil
}

// CandidateCareerURLs は公式サイトから試行する採用ページ URL 候補を返す。
func CandidateCareerURLs(websiteURL string) []string {
	base := strings.TrimRight(strings.TrimSpace(websiteURL), "/")
	if base == "" {
		return nil
	}
	paths := []string{"", "/careers", "/recruit", "/recruitment", "/jobs", "/join", "/engineering", "/about"}
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, p := range paths {
		u := base + p
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}

// FirstFetchableText は候補 URL を順に試し、最初に成功したテキストと URL を返す。
func FirstFetchableText(ctx context.Context, urls []string) (text, sourceURL string, err error) {
	var lastErr error
	for _, u := range urls {
		t, e := FetchURLText(ctx, u)
		if e == nil && t != "" {
			return t, u, nil
		}
		if e != nil {
			lastErr = e
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no fetchable url")
	}
	return "", "", lastErr
}
