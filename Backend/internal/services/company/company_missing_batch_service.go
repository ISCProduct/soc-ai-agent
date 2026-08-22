package company

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/companyfields"
	"Backend/internal/config"
	"Backend/internal/logger"
	"Backend/internal/models"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// 企業取得は gBiz/OpenAI 待ちが支配的。6→8 は CPU をほぼ増やさず壁時計を短縮する。
	defaultMissingBatchConcurrency = 8
)

// MissingBatchOptions は企業管理全体の不足データ一括取得オプション。
type MissingBatchOptions struct {
	Limit  int  `json:"limit"`
	DryRun bool `json:"dry_run"`
	// PrimaryOnly が true のとき基本・技術・関係のみ対象（求人は除外）。
	PrimaryOnly bool `json:"primary_only"`
	// Concurrency は同時に処理する企業数（未指定時は defaultMissingBatchConcurrency）。
	Concurrency int `json:"concurrency"`
}

// MissingBatchItem は1社分の不足埋め結果。
type MissingBatchItem struct {
	CompanyID       uint   `json:"company_id"`
	Name            string `json:"name"`
	DataStatus      string `json:"data_status"`
	NeedInfo        bool   `json:"need_info"`
	NeedJobs        bool   `json:"need_jobs"`
	NeedTech        bool   `json:"need_tech"`
	NeedRelations   bool   `json:"need_relations"`
	InfoStatus      string `json:"info_status,omitempty"`
	JobsStatus      string `json:"jobs_status,omitempty"`
	TechStatus      string `json:"tech_status,omitempty"`
	RelationsStatus string `json:"relations_status,omitempty"`
	Error           string `json:"error,omitempty"`
}

// MissingBatchResult は一括取得の集計結果。
type MissingBatchResult struct {
	DryRun      bool               `json:"dry_run"`
	Limit       int                `json:"limit"`
	PrimaryOnly bool               `json:"primary_only"`
	Concurrency int                `json:"concurrency"`
	CandidateN  int                `json:"candidate_n"`
	Processed   int                `json:"processed"`
	InfoOK      int                `json:"info_ok"`
	JobsOK      int                `json:"jobs_ok"`
	TechOK      int                `json:"tech_ok"`
	RelationsOK int                `json:"relations_ok"`
	Skipped     int                `json:"skipped"`
	Errors      int                `json:"errors"`
	StopReason  string             `json:"stop_reason,omitempty"`
	Items       []MissingBatchItem `json:"items"`
}

// CompanyMissingBatchService は未取得フィールドだけを上限付きで埋める。
type CompanyMissingBatchService struct {
	repo             repository.CompanyRepository
	infoFetcher      *CompanyInfoFetcher
	jobFetcher       *JobFetchService
	techFetcher      *TechStackFetcher
	relationsFetcher *CompanyRelationsFetcher
}

func NewCompanyMissingBatchService(
	repo repository.CompanyRepository,
	info *CompanyInfoFetcher,
	jobs *JobFetchService,
	tech *TechStackFetcher,
	relations *CompanyRelationsFetcher,
) *CompanyMissingBatchService {
	return &CompanyMissingBatchService{
		repo:             repo,
		infoFetcher:      info,
		jobFetcher:       jobs,
		techFetcher:      tech,
		relationsFetcher: relations,
	}
}

func clampMissingBatchConcurrency(n int) int {
	if n <= 0 {
		return defaultMissingBatchConcurrency
	}
	if maxC := config.MissingBatchMaxConcurrency(); n > maxC {
		return maxC
	}
	return n
}

// Run は不足企業を最大 Limit 社まで並列処理する（不足判定済みの項目は force=true で取得）。
// 概要を先に取り、技術・関連・求人は企業内で並列。企業間は Concurrency 上限。
func (s *CompanyMissingBatchService) Run(ctx context.Context, opts MissingBatchOptions) (*MissingBatchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = config.MissingBatchDefaultLimit()
	}
	if maxL := config.MissingBatchMaxLimit(); opts.Limit > maxL {
		opts.Limit = maxL
	}
	concurrency := clampMissingBatchConcurrency(opts.Concurrency)

	candidates, err := s.repo.ListActiveMissingFetchCandidates(opts.Limit*3, opts.PrimaryOnly)
	if err != nil {
		return nil, fmt.Errorf("list missing candidates: %w", err)
	}

	items := make([]MissingBatchItem, 0, opts.Limit)
	for _, c := range candidates {
		item := MissingBatchItem{
			CompanyID:  c.ID,
			Name:       c.Name,
			DataStatus: c.DataStatus,
		}
		item.NeedInfo, item.NeedJobs, item.NeedTech, item.NeedRelations = MissingNeedsFromCompany(&c)
		// DB 実データの欠落も不足扱い（*_fetched_at だけ進んでいる残骸対策）
		if !opts.PrimaryOnly && !item.NeedJobs {
			if jobs, err := s.repo.ListJobPositions(&c.ID, nil, 1); err == nil && len(jobs) == 0 {
				item.NeedJobs = true
			}
		}
		if opts.PrimaryOnly {
			item.NeedJobs = false
		}
		if !item.NeedRelations && s.relationsFetcher != nil && !s.relationsFetcher.HasStoredData(c.ID) {
			item.NeedRelations = true
		}
		if !item.NeedInfo && !item.NeedJobs && !item.NeedTech && !item.NeedRelations {
			continue
		}
		items = append(items, item)
		if len(items) >= opts.Limit {
			break
		}
	}

	result := &MissingBatchResult{
		DryRun:      opts.DryRun,
		Limit:       opts.Limit,
		PrimaryOnly: opts.PrimaryOnly,
		Concurrency: concurrency,
		CandidateN:  len(items),
		Items:       items,
	}
	if opts.DryRun {
		result.StopReason = missingBatchStopReason(result)
		return result, nil
	}

	started := time.Now()
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)
	var mu sync.Mutex

	for i := range result.Items {
		item := &result.Items[i]
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				mu.Lock()
				item.Error = err.Error()
				result.Errors++
				mu.Unlock()
				slog.Warn("fetch_missing_batch item",
					"company_id", item.CompanyID,
					"name", item.Name,
					"error", item.Error,
					"cancelled", true,
				)
				return nil
			}
			mu.Lock()
			result.Processed++
			mu.Unlock()
			s.processItem(gctx, item, result, &mu)
			return nil
		})
	}
	_ = g.Wait()

	result.StopReason = missingBatchStopReason(result)
	logMissingBatchResult(result, time.Since(started))
	return result, nil
}

func (s *CompanyMissingBatchService) processItem(ctx context.Context, item *MissingBatchItem, result *MissingBatchResult, mu *sync.Mutex) {
	inc := func(apply func()) {
		mu.Lock()
		defer mu.Unlock()
		apply()
	}

	// 概要が公式URLを埋めるので先に取る。技術・関連・求人は独立なので並列。
	s.runInfoFetch(ctx, item, result, inc)
	g, gctx := errgroup.WithContext(ctx)
	if item.NeedTech {
		g.Go(func() error {
			s.runTechFetch(gctx, item, result, inc)
			return nil
		})
	}
	if item.NeedRelations {
		g.Go(func() error {
			s.runRelationsFetch(gctx, item, result, inc)
			return nil
		})
	}
	if item.NeedJobs {
		g.Go(func() error {
			s.runJobsFetch(gctx, item, result, inc)
			return nil
		})
	}
	_ = g.Wait()
	logMissingBatchItem(item)
}

func (s *CompanyMissingBatchService) runInfoFetch(ctx context.Context, item *MissingBatchItem, result *MissingBatchResult, inc func(func())) {
	if !item.NeedInfo {
		return
	}
	if s.infoFetcher == nil {
		inc(func() {
			item.InfoStatus = "skipped_no_fetcher"
			result.Skipped++
		})
		return
	}
	res, err := s.infoFetcher.FetchAndSave(ctx, item.CompanyID, true)
	inc(func() {
		if err != nil {
			item.InfoStatus = "error"
			item.Error = err.Error()
			result.Errors++
			return
		}
		if res != nil && res.FromCache {
			company, _ := s.repo.FindByID(item.CompanyID)
			status, errMsg := classifyInfoCacheResult(res, company)
			item.InfoStatus = status
			if status == "error" {
				item.Error = errMsg
				result.Errors++
			} else {
				result.Skipped++
			}
			return
		}
		company, findErr := s.repo.FindByID(item.CompanyID)
		if findErr == nil && company != nil && companyfetch.HasBasicInfo(company.Description, company.WebsiteURL) {
			item.InfoStatus = "ok"
			result.InfoOK++
			return
		}
		item.InfoStatus = "empty"
		result.Skipped++
	})
}

func (s *CompanyMissingBatchService) runTechFetch(ctx context.Context, item *MissingBatchItem, result *MissingBatchResult, inc func(func())) {
	if s.techFetcher == nil {
		inc(func() {
			item.TechStatus = "skipped_no_fetcher"
			result.Skipped++
		})
		return
	}
	res, err := s.techFetcher.FetchAndSave(ctx, item.CompanyID, true)
	inc(func() {
		if err != nil {
			item.TechStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			result.Errors++
			return
		}
		if res != nil && res.FromCache {
			company, _ := s.repo.FindByID(item.CompanyID)
			if company != nil && companyfetch.HasTechDataForIndustry(company.Industry, company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
				item.TechStatus = "skipped_cache"
				result.Skipped++
				return
			}
			detail := "cache_without_data"
			if res.SkipReason != "" {
				detail = res.SkipReason
			}
			item.TechStatus = "error"
			if item.Error == "" {
				item.Error = "tech: " + detail
			}
			result.Errors++
			return
		}
		company, findErr := s.repo.FindByID(item.CompanyID)
		if findErr == nil && company != nil &&
			companyfetch.HasTechDataForIndustry(company.Industry, company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
			item.TechStatus = "ok"
			result.TechOK++
			return
		}
		item.TechStatus = "empty"
		result.Skipped++
	})
}

func (s *CompanyMissingBatchService) runRelationsFetch(ctx context.Context, item *MissingBatchItem, result *MissingBatchResult, inc func(func())) {
	if s.relationsFetcher == nil {
		inc(func() {
			item.RelationsStatus = "skipped_no_fetcher"
			result.Skipped++
		})
		return
	}
	res, err := s.relationsFetcher.FetchAndSave(ctx, item.CompanyID, true)
	inc(func() {
		if err != nil {
			item.RelationsStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			result.Errors++
			return
		}
		if res != nil && res.FromCache {
			if s.relationsFetcher.HasStoredData(item.CompanyID) {
				item.RelationsStatus = "skipped_cache"
				result.Skipped++
				return
			}
			detail := "cache_without_data"
			if res.SkipReason != "" {
				detail = res.SkipReason
			}
			item.RelationsStatus = "error"
			if item.Error == "" {
				item.Error = "relations: " + detail
			}
			result.Errors++
			return
		}
		if s.relationsFetcher.HasStoredData(item.CompanyID) {
			item.RelationsStatus = "ok"
			result.RelationsOK++
			return
		}
		item.RelationsStatus = "empty"
		result.Skipped++
	})
}

func (s *CompanyMissingBatchService) runJobsFetch(ctx context.Context, item *MissingBatchItem, result *MissingBatchResult, inc func(func())) {
	if s.jobFetcher == nil {
		inc(func() {
			item.JobsStatus = "skipped_no_fetcher"
			result.Skipped++
		})
		return
	}
	positions, err := s.jobFetcher.FetchAndSaveJobs(ctx, item.CompanyID, true)
	inc(func() {
		if err != nil {
			item.JobsStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			result.Errors++
			return
		}
		if len(positions) == 0 {
			item.JobsStatus = "empty"
			result.Skipped++
			return
		}
		item.JobsStatus = fmt.Sprintf("ok(%d)", len(positions))
		result.JobsOK++
	})
}

// missingBatchStopReason は FE が波を打ち切るか・なぜ止まったかをログで辿るための理由。
func missingBatchStopReason(r *MissingBatchResult) string {
	if r == nil {
		return "empty_result"
	}
	if r.DryRun {
		return "dry_run"
	}
	filled := r.InfoOK + r.JobsOK + r.TechOK + r.RelationsOK
	switch {
	case r.Processed == 0:
		return "no_candidates"
	case r.Errors > 0 && filled == 0:
		return "all_failed"
	case filled == 0:
		return "no_fills"
	case r.Limit > 0 && r.Processed < r.Limit:
		return "partial_wave"
	default:
		return "wave_full"
	}
}

func missingBatchItemNeedsLog(item MissingBatchItem) bool {
	if item.Error != "" {
		return true
	}
	for _, st := range []string{item.InfoStatus, item.TechStatus, item.RelationsStatus, item.JobsStatus} {
		if st == "error" || st == "empty" || strings.HasPrefix(st, "skipped") {
			return true
		}
	}
	return false
}

func logMissingBatchItem(item *MissingBatchItem) {
	if item == nil || !missingBatchItemNeedsLog(*item) {
		return
	}
	slog.Warn("fetch_missing_batch item",
		"company_id", item.CompanyID,
		"name", item.Name,
		"need_info", item.NeedInfo,
		"need_tech", item.NeedTech,
		"need_relations", item.NeedRelations,
		"info", item.InfoStatus,
		"tech", item.TechStatus,
		"relations", item.RelationsStatus,
		"jobs", item.JobsStatus,
		"error", item.Error,
	)
}

func logMissingBatchResult(result *MissingBatchResult, elapsed time.Duration) {
	if result == nil {
		return
	}
	attrs := []any{
		"stop_reason", result.StopReason,
		"elapsed_ms", elapsed.Milliseconds(),
		"processed", result.Processed,
		"limit", result.Limit,
		"concurrency", result.Concurrency,
		"info_ok", result.InfoOK,
		"tech_ok", result.TechOK,
		"relations_ok", result.RelationsOK,
		"jobs_ok", result.JobsOK,
		"skipped", result.Skipped,
		"errors", result.Errors,
		"failures", missingBatchFailureLog(result.Items, 20),
	}
	if result.Errors > 0 {
		if key := logger.PutErrorJSON("fetch_missing_batch", map[string]any{
			"stop_reason":  result.StopReason,
			"processed":    result.Processed,
			"limit":        result.Limit,
			"concurrency":  result.Concurrency,
			"info_ok":      result.InfoOK,
			"tech_ok":      result.TechOK,
			"relations_ok": result.RelationsOK,
			"jobs_ok":      result.JobsOK,
			"skipped":      result.Skipped,
			"errors":       result.Errors,
			"failures":     result.FailureSamples(50),
		}); key != "" {
			attrs = append(attrs, "s3_key", key)
		}
	}
	if result.StopReason == "all_failed" || result.StopReason == "no_fills" || result.Errors > 0 {
		slog.Warn("fetch_missing_batch done", attrs...)
		return
	}
	slog.Info("fetch_missing_batch done", attrs...)
}

func missingBatchFailureLog(items []MissingBatchItem, max int) string {
	if max <= 0 {
		max = 20
	}
	parts := make([]string, 0, max)
	for _, it := range items {
		if it.Error == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("id=%d %s: %s", it.CompanyID, it.Name, it.Error))
		if len(parts) >= max {
			break
		}
	}
	return strings.Join(parts, " | ")
}

func (r *MissingBatchResult) FailureSamples(max int) []map[string]any {
	if r == nil || max <= 0 {
		return nil
	}
	out := make([]map[string]any, 0, max)
	for _, it := range r.Items {
		if it.Error == "" {
			continue
		}
		out = append(out, map[string]any{
			"company_id": it.CompanyID,
			"name":       it.Name,
			"error":      it.Error,
			"info":       it.InfoStatus,
			"tech":       it.TechStatus,
			"relations":  it.RelationsStatus,
		})
		if len(out) >= max {
			break
		}
	}
	return out
}

// classifyInfoCacheResult は FetchAndSave の FromCache 結果をバッチ用ステータスへ変換する。
// 予算超過は公開情報不足（empty）より先に判定し、警告経路を落とさない。
func classifyInfoCacheResult(res *CompanyInfoResult, company *models.Company) (status, errMsg string) {
	if res != nil && (res.BudgetExceeded || res.SkipReason == "budget") {
		return "error", "info: budget"
	}
	if company != nil && companyfetch.HasBasicInfo(company.Description, company.WebsiteURL) {
		return "skipped_cache", ""
	}
	if company != nil && companyfetch.HasBasicInfoFootprint(company.Description, company.WebsiteURL, company.Location) {
		return "empty", ""
	}
	detail := "cache_without_data"
	if res != nil && res.SkipReason != "" {
		detail = res.SkipReason
	}
	return "error", "info: " + detail
}

// MissingNeedsFromCompany は不足判定ヘルパ（基本情報・技術・ビジネス関係・求人）。
// 基本情報は TTL 切れ、または概要+公式URLが揃っていない場合に不足（FE missingAspects と揃える）。
// 技術は中身が空なら不足とみなす。関係の DB 実体欠落は Run() 側で HasStoredData により追加判定する。
func MissingNeedsFromCompany(c *models.Company) (needInfo, needJobs, needTech, needRelations bool) {
	if c == nil {
		return false, false, false, false
	}
	needInfo = !companyfetch.IsFresh(c.InfoFetchedAt, companyfetch.TTLInfo) ||
		!companyfetch.HasBasicInfo(c.Description, c.WebsiteURL)
	needJobs = !companyfetch.IsFresh(c.JobsFetchedAt, companyfetch.TTLJobs)
	needTech = companyfields.RequiresTech(c.Industry) &&
		(!companyfetch.IsFresh(c.TechFetchedAt, companyfetch.TTLTech) ||
			!companyfetch.HasTechDataForIndustry(c.Industry, c.TechStack, c.InfraStack, c.CicdTools, c.DevelopmentStyle))
	needRelations = !companyfetch.IsFresh(c.RelationsFetchedAt, companyfetch.TTLRelations)
	return
}
