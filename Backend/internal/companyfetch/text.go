package companyfetch

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	TTLInfo = 90 * 24 * time.Hour
	TTLJobs = 7 * 24 * time.Hour
	TTLTech = 30 * 24 * time.Hour

	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"

	SourceScrape    = "scrape"
	SourceWebSearch = "web_search"

	defaultMaxTextRunes = 1500
	defaultMinTextRunes = 80
)

var (
	scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	tagRe    = regexp.MustCompile(`(?is)<[^>]+>`)
	spaceRe  = regexp.MustCompile(`\s+`)
)

// IsFresh は fetchedAt が TTL 以内なら true。
func IsFresh(fetchedAt *time.Time, ttl time.Duration) bool {
	if fetchedAt == nil {
		return false
	}
	return time.Since(*fetchedAt) < ttl
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

// FetchURLText は URL から HTML を取得し、正規化・トリムしたテキストを返す。
func FetchURLText(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("url is empty")
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SocAI/1.0; +https://example.com/bot)")
	client := &http.Client{Timeout: 15 * time.Second}
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
func FirstFetchableText(urls []string) (text, sourceURL string, err error) {
	var lastErr error
	for _, u := range urls {
		t, e := FetchURLText(u)
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
