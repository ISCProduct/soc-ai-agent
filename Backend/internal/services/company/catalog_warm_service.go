package company

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultL1WarmLimit = 100
	maxL1WarmLimit     = 300
)

// CatalogWarmService はマッチング用 L1 カタログ（基本情報 + WeightProfile）を温存する。
type CatalogWarmService struct {
	repo        repository.CompanyRepository
	infoFetcher *CompanyInfoFetcher
	jobFetcher  *JobFetchService
}

func NewCatalogWarmService(
	repo repository.CompanyRepository,
	infoFetcher *CompanyInfoFetcher,
	jobFetcher *JobFetchService,
) *CatalogWarmService {
	return &CatalogWarmService{
		repo:        repo,
		infoFetcher: infoFetcher,
		jobFetcher:  jobFetcher,
	}
}

// L1Coverage は公開カタログの L1 充足率。
type L1Coverage struct {
	PublishedTotal int64    `json:"published_total"`
	InfoFresh      int64    `json:"info_fresh"`
	HasProfile     int64    `json:"has_profile"`
	NeedsWarm      int64    `json:"needs_warm"`
	InfoRate       float64  `json:"info_rate"`
	ProfileRate    float64  `json:"profile_rate"`
	InfoTarget     float64  `json:"info_target"`
	ProfileTarget  float64  `json:"profile_target"`
	BelowTarget    bool     `json:"below_target"`
	Alerts         []string `json:"alerts,omitempty"`
}

// L1WarmOptions はバッチ実行オプション。
type L1WarmOptions struct {
	Limit          int  `json:"limit"`
	DryRun         bool `json:"dry_run"`
	Force          bool `json:"force"`
	IncludeInfo    bool `json:"include_info"`
	IncludePersona bool `json:"include_persona"`
}

// L1WarmItem は 1 社分の処理結果。
type L1WarmItem struct {
	CompanyID     uint   `json:"company_id"`
	Name          string `json:"name"`
	Industry      string `json:"industry"`
	Priority      int    `json:"priority"`
	NeedInfo      bool   `json:"need_info"`
	NeedPersona   bool   `json:"need_persona"`
	InfoStatus    string `json:"info_status,omitempty"`
	PersonaStatus string `json:"persona_status,omitempty"`
	Error         string `json:"error,omitempty"`
}

// L1WarmResult はバッチ全体の結果。
type L1WarmResult struct {
	DryRun     bool         `json:"dry_run"`
	Limit      int          `json:"limit"`
	CandidateN int          `json:"candidate_count"`
	Processed  int          `json:"processed"`
	InfoOK     int          `json:"info_ok"`
	PersonaOK  int          `json:"persona_ok"`
	Skipped    int          `json:"skipped"`
	Errors     int          `json:"errors"`
	Items      []L1WarmItem `json:"items"`
	Coverage   *L1Coverage  `json:"coverage,omitempty"`
}

// Coverage は公開企業の L1 充足率を返す。
func (s *CatalogWarmService) Coverage(ctx context.Context) (*L1Coverage, error) {
	_ = ctx
	stats, err := s.repo.CountL1Coverage(companyfetch.TTLInfo)
	if err != nil {
		return nil, err
	}
	cov := &L1Coverage{
		PublishedTotal: stats.PublishedTotal,
		InfoFresh:      stats.InfoFresh,
		HasProfile:     stats.HasProfile,
		NeedsWarm:      stats.NeedsWarm,
		InfoTarget:     0.80,
		ProfileTarget:  0.95,
	}
	if stats.PublishedTotal > 0 {
		cov.InfoRate = float64(stats.InfoFresh) / float64(stats.PublishedTotal)
		cov.ProfileRate = float64(stats.HasProfile) / float64(stats.PublishedTotal)
	}
	if cov.InfoRate < cov.InfoTarget {
		cov.BelowTarget = true
		cov.Alerts = append(cov.Alerts, fmt.Sprintf("info_rate=%.0f%% < target %.0f%%", cov.InfoRate*100, cov.InfoTarget*100))
	}
	if cov.ProfileRate < cov.ProfileTarget {
		cov.BelowTarget = true
		cov.Alerts = append(cov.Alerts, fmt.Sprintf("profile_rate=%.0f%% < target %.0f%%", cov.ProfileRate*100, cov.ProfileTarget*100))
	}
	if cov.BelowTarget {
		log.Printf("l1_coverage ALERT published=%d needs_warm=%d alerts=%v", cov.PublishedTotal, cov.NeedsWarm, cov.Alerts)
	}
	return cov, nil
}

// WarmL1 は不足している published 企業へ info / persona を書き込む（Read 経路では呼ばない）。
func (s *CatalogWarmService) WarmL1(ctx context.Context, opts L1WarmOptions) (*L1WarmResult, error) {
	opts = normalizeL1WarmOptions(opts)
	candidates, err := s.repo.ListPublishedL1WarmCandidates(opts.Limit*3, companyfetch.TTLInfo)
	if err != nil {
		return nil, fmt.Errorf("list L1 candidates: %w", err)
	}

	scored := make([]L1WarmItem, 0, len(candidates))
	for _, c := range candidates {
		item := L1WarmItem{
			CompanyID:   c.ID,
			Name:        c.Name,
			Industry:    c.Industry,
			Priority:    L1WarmPriority(c.Name, c.Industry),
			NeedInfo:    opts.IncludeInfo && (opts.Force || !companyfetch.IsFresh(c.InfoFetchedAt, companyfetch.TTLInfo)),
			NeedPersona: opts.IncludePersona && (opts.Force || !c.HasWeightProfile),
		}
		if !item.NeedInfo && !item.NeedPersona {
			continue
		}
		scored = append(scored, item)
	}
	sortL1WarmItems(scored)
	if len(scored) > opts.Limit {
		scored = scored[:opts.Limit]
	}

	result := &L1WarmResult{
		DryRun:     opts.DryRun,
		Limit:      opts.Limit,
		CandidateN: len(scored),
		Items:      scored,
	}
	if opts.DryRun {
		cov, _ := s.Coverage(ctx)
		result.Coverage = cov
		return result, nil
	}

	for i := range result.Items {
		item := &result.Items[i]
		if err := ctx.Err(); err != nil {
			item.Error = err.Error()
			result.Errors++
			break
		}
		result.Processed++
		if item.NeedInfo {
			if s.infoFetcher == nil {
				item.InfoStatus = "skipped_no_fetcher"
				result.Skipped++
			} else if _, err := s.infoFetcher.FetchAndSave(ctx, item.CompanyID, opts.Force); err != nil {
				item.InfoStatus = "error"
				item.Error = err.Error()
				result.Errors++
				log.Printf("warm_l1: info failed id=%d: %v", item.CompanyID, err)
			} else {
				item.InfoStatus = "ok"
				result.InfoOK++
			}
		}
		if item.NeedPersona {
			if s.jobFetcher == nil {
				item.PersonaStatus = "skipped_no_fetcher"
				result.Skipped++
			} else if _, err := s.jobFetcher.FetchAndSavePersona(ctx, item.CompanyID, opts.Force); err != nil {
				item.PersonaStatus = "error"
				if item.Error == "" {
					item.Error = err.Error()
				}
				result.Errors++
				log.Printf("warm_l1: persona failed id=%d: %v", item.CompanyID, err)
			} else {
				item.PersonaStatus = "ok"
				result.PersonaOK++
			}
		}
	}

	cov, _ := s.Coverage(ctx)
	result.Coverage = cov
	log.Printf(
		"warm_l1: processed=%d info_ok=%d persona_ok=%d errors=%d",
		result.Processed, result.InfoOK, result.PersonaOK, result.Errors,
	)
	return result, nil
}

func normalizeL1WarmOptions(opts L1WarmOptions) L1WarmOptions {
	if !opts.IncludeInfo && !opts.IncludePersona {
		opts.IncludeInfo = true
		opts.IncludePersona = true
	}
	if opts.Limit <= 0 {
		opts.Limit = defaultL1WarmBatchLimit()
	}
	if opts.Limit > maxL1WarmLimit {
		opts.Limit = maxL1WarmLimit
	}
	return opts
}

func defaultL1WarmBatchLimit() int {
	raw := strings.TrimSpace(os.Getenv("L1_WARM_BATCH_LIMIT"))
	if raw == "" {
		return defaultL1WarmLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultL1WarmLimit
	}
	if n > maxL1WarmLimit {
		return maxL1WarmLimit
	}
	return n
}

// L1WarmPriority は Core / 中小SI 寄りを低く（先に処理）する。値が小さいほど優先。
func L1WarmPriority(name, industry string) int {
	text := strings.ToLower(name + " " + industry)
	keywords := []string{
		"si", "sier", "システムインテグレ", "システム開発", "受託", "ses",
		"情報サービス", "ソフトウェア", "itコンサル", "エンジニアリング",
		"ソリューション", "テック", "tech", "saas",
	}
	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			return 0
		}
	}
	return 1
}

func sortL1WarmItems(items []L1WarmItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return l1WarmLess(items[i], items[j])
	})
}

func l1WarmLess(a, b L1WarmItem) bool {
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	if a.NeedPersona != b.NeedPersona {
		return a.NeedPersona
	}
	if a.NeedInfo != b.NeedInfo {
		return a.NeedInfo
	}
	return a.CompanyID < b.CompanyID
}
