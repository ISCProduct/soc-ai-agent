package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// GBizRecord is one entry from the gBizINFO API.
type GBizRecord struct {
	CorporateNumber  string `json:"corporate_number"`
	Name             string `json:"name"`
	PostalCode       string `json:"postal_code"`
	Location         string `json:"location"`
	BusinessSummary  struct {
		MajorClassificationName string `json:"major_classification_name"`
	} `json:"business_summary"`
	CompanyURL string `json:"company_url"`
}

// GBizClient calls the gBizINFO API and normalises company names.
type GBizClient struct {
	BaseURL string
	Token   string
	Client  *http.Client
	Limiter *rate.Limiter

	mu    sync.Mutex
	cache map[string][]GBizRecord
}

func NewGBizClient(baseURL, token string) *GBizClient {
	if baseURL == "" {
		baseURL = "https://info.gbiz.go.jp/hojin/v1/hojin"
	}
	return &GBizClient{
		BaseURL: baseURL,
		Token:   token,
		Client:  &http.Client{Timeout: 15 * time.Second},
		Limiter: NewLimiter(1 * time.Second),
		cache:   map[string][]GBizRecord{},
	}
}

// Search queries gBizINFO for a company name (with optional postal code).
func (c *GBizClient) Search(ctx context.Context, name, postalCode string) ([]GBizRecord, error) {
	cacheKey := name + "|" + postalCode
	c.mu.Lock()
	if cached, ok := c.cache[cacheKey]; ok {
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}

	params := url.Values{"name": {name}, "limit": {"10"}}
	if postalCode != "" {
		params.Set("postal_code", postalCode)
	}
	reqURL := c.BaseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set("X-hojinInfo-api-token", c.Token)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gbizinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("gbizinfo: 401 Unauthorized (check GBIZINFO_API_TOKEN)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gbizinfo: HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Records []GBizRecord `json:"hojin-infos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("gbizinfo decode: %w", err)
	}

	c.mu.Lock()
	c.cache[cacheKey] = payload.Records
	c.mu.Unlock()

	return payload.Records, nil
}

// Match finds the best gBizINFO record for a RawCompany.
// Returns (record, score, error). When no record is found or token is missing,
// a fallback CompanyNode with corporate_number = "UNKNOWN_{name}" is returned.
func (c *GBizClient) Match(ctx context.Context, raw *RawCompany, threshold float64) (*CompanyNode, error) {
	normalized := NormalizeName(raw.RawName)

	// Try with postal code first, then without
	var candidates []GBizRecord
	var err error
	if raw.PostalCode != "" {
		candidates, err = c.Search(ctx, normalized, raw.PostalCode)
	}
	if err != nil || len(candidates) == 0 {
		candidates, err = c.Search(ctx, normalized, "")
	}

	if err != nil || len(candidates) == 0 {
		return fallbackNode(raw), nil
	}

	// Score each candidate
	best, bestScore := candidates[0], 0.0
	for _, rec := range candidates {
		s := Similarity(normalized, NormalizeName(rec.Name))
		if s > bestScore {
			bestScore = s
			best = rec
		}
	}

	needsReview := bestScore < threshold

	node := &CompanyNode{
		CorporateNumber:         best.CorporateNumber,
		OfficialName:            best.Name,
		SourceURLs:              []string{raw.SourceURL},
		BusinessCategory:        best.BusinessSummary.MajorClassificationName,
		Address:                 best.Location,
		Website:                 best.CompanyURL,
		Capital:                 raw.Capital,
		Employees:               raw.Employees,
		MatchScore:              bestScore,
		NeedsReview:             needsReview,
		BusinessDescription:     raw.BusinessDescription,
		RawRelatedCompaniesText: raw.RelatedCompaniesText,
		RawBusinessPartnersText: raw.BusinessPartnersText,
	}

	// gBizINFO で関連会社・主要取引先の法人番号を解決する
	node.RelatedCompanies = c.ResolveCompanyRefs(ctx, raw.RelatedCompaniesText)
	node.BusinessPartners = c.ResolveCompanyRefs(ctx, raw.BusinessPartnersText)

	return node, nil
}

// ResolveCompanyRefs parses a free-text list of company names (delimited by 、,, or newlines)
// and resolves each name to a CompanyRef via gBizINFO. Names that cannot be resolved
// still appear in the result with an empty CorporateNumber.
func (c *GBizClient) ResolveCompanyRefs(ctx context.Context, text string) []CompanyRef {
	if text == "" {
		return nil
	}

	// 区切り文字で分割（全角読点・半角カンマ・改行）
	sep := func(r rune) bool {
		return r == '、' || r == ',' || r == '\n' || r == '・'
	}
	names := strings.FieldsFunc(text, sep)

	var refs []CompanyRef
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		ref := CompanyRef{Name: name}

		// gBizINFO で法人番号を取得（ベストエフォート）
		normalized := NormalizeName(name)
		if normalized != "" {
			if candidates, err := c.Search(ctx, normalized, ""); err == nil && len(candidates) > 0 {
				best, bestScore := candidates[0], 0.0
				for _, rec := range candidates {
					s := Similarity(normalized, NormalizeName(rec.Name))
					if s > bestScore {
						bestScore = s
						best = rec
					}
				}
				// 0.7 以上のマッチのみ法人番号を付与（精度優先）
				if bestScore >= 0.7 {
					ref.CorporateNumber = best.CorporateNumber
				}
			}
		}

		refs = append(refs, ref)
	}
	return refs
}

func fallbackNode(raw *RawCompany) *CompanyNode {
	return &CompanyNode{
		CorporateNumber:         "UNKNOWN_" + NormalizeName(raw.RawName),
		OfficialName:            raw.RawName,
		SourceURLs:              []string{raw.SourceURL},
		Address:                 raw.Address,
		Website:                 raw.Website,
		Capital:                 raw.Capital,
		Employees:               raw.Employees,
		MatchScore:              0,
		NeedsReview:             true,
		BusinessDescription:     raw.BusinessDescription,
		RawRelatedCompaniesText: raw.RelatedCompaniesText,
		RawBusinessPartnersText: raw.BusinessPartnersText,
		// fallback時はgBizINFO解決なし（テキスト原文はRaw*フィールドに保持）
	}
}
