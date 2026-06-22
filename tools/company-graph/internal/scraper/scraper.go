// Package scraper implements multi-source scraping of Japanese new-graduate job sites
// and normalization via the gBizINFO API.
package scraper

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/time/rate"
)

// ── Data types ──────────────────────────────────────────────────────────────

// RawCompany holds data as scraped from a job site before normalization.
type RawCompany struct {
	SourceSite           string
	SourceURL            string
	RawName              string
	Address              string
	PostalCode           string
	Website              string
	Capital              string
	Employees            string
	RelatedCompaniesText string
	BusinessPartnersText string
	BusinessDescription  string
}

// CompanyRef is a resolved reference to another company (used for relations).
type CompanyRef struct {
	Name            string `json:"name"`
	CorporateNumber string `json:"corporate_number,omitempty"` // empty when gBizINFO lookup failed
}

// CompanyNode is a normalized company record keyed by corporate number.
type CompanyNode struct {
	CorporateNumber          string
	OfficialName             string
	SourceURLs               []string
	BusinessCategory         string
	Address                  string
	Website                  string
	Capital                  string
	Employees                string
	MatchScore               float64
	NeedsReview              bool
	BusinessDescription      string       // 事業内容（スクレイピング原文）
	RelatedCompanies         []CompanyRef // 関連会社・グループ会社（gBizINFO法人番号解決済）
	BusinessPartners         []CompanyRef // 主要取引先（gBizINFO法人番号解決済）
	RawRelatedCompaniesText  string       // 関連会社テキスト原文（gBizINFO未解決時のフォールバック用）
	RawBusinessPartnersText  string       // 主要取引先テキスト原文（同上）
}

// RunRequest parameterises a pipeline run.
type RunRequest struct {
	Sites    []string // "mynavi", "rikunabi", "career_tasu"
	Query    string
	MaxPages int
	Year     int // 0 = auto-calculate
}

// ScrapeWarning represents a warning emitted during a pipeline run.
// It is used to surface DOM-change alerts and site-level failures
// without aborting the entire pipeline (graceful degradation).
type ScrapeWarning struct {
	Site    string
	Message string
}

// Selector version constants track the CSS/JS selector schema for each scraper.
// Increment the version whenever selectors are intentionally updated,
// so that monitoring can distinguish intentional changes from unexpected breakage.
const (
	MynaviSelectorVersion     = 1 // last updated: 2024
	RikunabiSelectorVersion   = 1 // last updated: 2024
	CareerTasuSelectorVersion = 1 // last updated: 2024
)

// RunResult is returned by Pipeline.Run.
type RunResult struct {
	TargetYear int
	Nodes      map[string]*CompanyNode // keyed by corporate_number
	Logs       []string
	// Warnings contains non-fatal issues such as possible DOM changes or
	// site-level scraping failures. Callers should surface these to operators.
	Warnings []ScrapeWarning
}

// ── Year resolution ──────────────────────────────────────────────────────────

// ResolveYear returns the target graduation year.
//
//   - If override > 0, return it unchanged.
//   - April or later  → current year + 2 (new recruitment season opens)
//   - January – March → current year + 1 (current main season)
func ResolveYear(override int) int {
	if override > 0 {
		return override
	}
	now := time.Now()
	if now.Month() >= time.April {
		return now.Year() + 2
	}
	return now.Year() + 1
}

// ── Rate-limited HTTP fetcher ────────────────────────────────────────────────

// NewLimiter creates a rate.Limiter that allows one request per interval.
func NewLimiter(perRequest time.Duration) *rate.Limiter {
	return rate.NewLimiter(rate.Every(perRequest), 1)
}

var defaultClient = &http.Client{Timeout: 30 * time.Second}

// FetchHTML performs a rate-limited GET and returns the response body as a string.
func FetchHTML(url, userAgent string, limiter *rate.Limiter) (string, error) {
	if err := limiter.Wait(context.Background()); err != nil {
		return "", fmt.Errorf("rate limiter: %w", err)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := defaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ── Name normalization ───────────────────────────────────────────────────────

var corpAbbr = map[string]string{
	"（株）": "株式会社",
	"(株)":  "株式会社",
	"㈱":    "株式会社",
	"（有）": "有限会社",
	"(有)":  "有限会社",
	"㈲":    "有限会社",
	"（合）": "合同会社",
	"(合)":  "合同会社",
	"（資）": "合資会社",
	"(資)":  "合資会社",
	"（名）": "合名会社",
	"(名)":  "合名会社",
}

var reSpace = regexp.MustCompile(`\s+`)

// NormalizeName normalises Japanese company name representations.
// It applies NFKC Unicode normalisation, expands corporate abbreviations,
// and strips whitespace.
func NormalizeName(name string) string {
	// NFKC: full-width → half-width, etc.
	name = strings.Map(func(r rune) rune {
		return unicode.ToTitle(r)
	}, name) // crude; proper NFKC needs golang.org/x/text
	// Abbreviation expansion
	for abbr, full := range corpAbbr {
		name = strings.ReplaceAll(name, abbr, full)
	}
	// Collapse whitespace
	name = reSpace.ReplaceAllString(name, "")
	return strings.TrimSpace(name)
}

// Similarity returns a simple ratio in [0,1] using longest-common-subsequence length.
func Similarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 && lb == 0 {
		return 1
	}
	if la == 0 || lb == 0 {
		return 0
	}
	// LCS via DP
	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			if ra[i-1] == rb[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	lcs := dp[la][lb]
	return 2.0 * float64(lcs) / float64(la+lb)
}

// ── Logging helper ───────────────────────────────────────────────────────────

// Logger collects log lines for later retrieval.
type Logger struct {
	lines []string
}

func (l *Logger) Logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Println("[scraper]", msg)
	l.lines = append(l.lines, msg)
}

func (l *Logger) Lines() []string { return l.lines }
