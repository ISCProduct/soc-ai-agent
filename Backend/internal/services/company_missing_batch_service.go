package services

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/companyfields"
	"Backend/internal/config"
	"Backend/internal/models"
	"context"
	"fmt"
	"log"
	"sync"

	"golang.org/x/sync/errgroup"
)

const (
	defaultMissingBatchConcurrency = 4
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
// 1社内の主3種は依存があるため直列、企業間は Concurrency 上限で並列化する。
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
			if jobs, err := s.repo.ListJobPositions(&c.ID, 1); err == nil && len(jobs) == 0 {
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
		return result, nil
	}

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

	log.Printf(
		"fetch_missing_batch: concurrency=%d processed=%d info_ok=%d jobs_ok=%d tech_ok=%d relations_ok=%d errors=%d",
		concurrency, result.Processed, result.InfoOK, result.JobsOK, result.TechOK, result.RelationsOK, result.Errors,
	)
	return result, nil
}

func (s *CompanyMissingBatchService) processItem(ctx context.Context, item *MissingBatchItem, result *MissingBatchResult, mu *sync.Mutex) {
	inc := func(apply func()) {
		mu.Lock()
		defer mu.Unlock()
		apply()
	}

	// 不足判定済みの項目は force=true で取りに行く。
	// force=false だと予算超過時に空キャッシュを「スキップ」扱いして実質未取得のまま終わるため。
	if item.NeedInfo {
		if s.infoFetcher == nil {
			item.InfoStatus = "skipped_no_fetcher"
			inc(func() { result.Skipped++ })
		} else if res, err := s.infoFetcher.FetchAndSave(ctx, item.CompanyID, true); err != nil {
			item.InfoStatus = "error"
			item.Error = err.Error()
			inc(func() { result.Errors++ })
		} else if res != nil && res.FromCache {
			company, _ := s.repo.FindByID(item.CompanyID)
			if company != nil && companyfetch.HasBasicInfo(company.Description, company.WebsiteURL) {
				item.InfoStatus = "skipped_cache"
				inc(func() { result.Skipped++ })
			} else if company != nil && companyfetch.HasBasicInfoFootprint(company.Description, company.WebsiteURL, company.Location) {
				item.InfoStatus = "empty"
				inc(func() { result.Skipped++ })
			} else {
				detail := "cache_without_data"
				if res.SkipReason != "" {
					detail = res.SkipReason
				}
				item.InfoStatus = "error"
				item.Error = "info: " + detail
				inc(func() { result.Errors++ })
			}
		} else if company, err := s.repo.FindByID(item.CompanyID); err == nil && company != nil &&
			companyfetch.HasBasicInfo(company.Description, company.WebsiteURL) {
			item.InfoStatus = "ok"
			inc(func() { result.InfoOK++ })
		} else {
			// 公開情報限定（概要なし）も empty として扱う（エラーではない）
			item.InfoStatus = "empty"
			inc(func() { result.Skipped++ })
		}
	}
	if item.NeedTech {
		if s.techFetcher == nil {
			item.TechStatus = "skipped_no_fetcher"
			inc(func() { result.Skipped++ })
		} else if res, err := s.techFetcher.FetchAndSave(ctx, item.CompanyID, true); err != nil {
			item.TechStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			inc(func() { result.Errors++ })
		} else if res != nil && res.FromCache {
			company, _ := s.repo.FindByID(item.CompanyID)
			if company != nil && companyfetch.HasTechDataForIndustry(company.Industry, company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
				item.TechStatus = "skipped_cache"
				inc(func() { result.Skipped++ })
			} else {
				detail := "cache_without_data"
				if res.SkipReason != "" {
					detail = res.SkipReason
				}
				item.TechStatus = "error"
				if item.Error == "" {
					item.Error = "tech: " + detail
				}
				inc(func() { result.Errors++ })
			}
		} else if company, err := s.repo.FindByID(item.CompanyID); err == nil && company != nil &&
			companyfetch.HasTechDataForIndustry(company.Industry, company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
			item.TechStatus = "ok"
			inc(func() { result.TechOK++ })
		} else {
			item.TechStatus = "empty"
			inc(func() { result.Skipped++ })
		}
	}
	if item.NeedRelations {
		if s.relationsFetcher == nil {
			item.RelationsStatus = "skipped_no_fetcher"
			inc(func() { result.Skipped++ })
		} else if res, err := s.relationsFetcher.FetchAndSave(ctx, item.CompanyID, true); err != nil {
			item.RelationsStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			inc(func() { result.Errors++ })
		} else if res != nil && res.FromCache {
			if s.relationsFetcher.HasStoredData(item.CompanyID) {
				item.RelationsStatus = "skipped_cache"
				inc(func() { result.Skipped++ })
			} else {
				detail := "cache_without_data"
				if res.SkipReason != "" {
					detail = res.SkipReason
				}
				item.RelationsStatus = "error"
				if item.Error == "" {
					item.Error = "relations: " + detail
				}
				inc(func() { result.Errors++ })
			}
		} else if s.relationsFetcher.HasStoredData(item.CompanyID) {
			item.RelationsStatus = "ok"
			inc(func() { result.RelationsOK++ })
		} else {
			item.RelationsStatus = "empty"
			inc(func() { result.Skipped++ })
		}
	}
	if item.NeedJobs {
		if s.jobFetcher == nil {
			item.JobsStatus = "skipped_no_fetcher"
			inc(func() { result.Skipped++ })
		} else if positions, err := s.jobFetcher.FetchAndSaveJobs(ctx, item.CompanyID, true); err != nil {
			item.JobsStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			inc(func() { result.Errors++ })
		} else if len(positions) == 0 {
			item.JobsStatus = "empty"
			inc(func() { result.Skipped++ })
		} else {
			item.JobsStatus = fmt.Sprintf("ok(%d)", len(positions))
			inc(func() { result.JobsOK++ })
		}
	}
}

// MissingNeedsFromCompany は不足判定ヘルパ（基本情報・技術・ビジネス関係・求人）。
// InfoFetchedAt / RelationsFetchedAt が TTL 内なら、公開情報限定の確認済みも含め再取得不要。
// 技術は中身が空なら不足とみなす。関係の DB 実体欠落は Run() 側で HasStoredData により追加判定する。
func MissingNeedsFromCompany(c *models.Company) (needInfo, needJobs, needTech, needRelations bool) {
	if c == nil {
		return false, false, false, false
	}
	needInfo = !companyfetch.IsFresh(c.InfoFetchedAt, companyfetch.TTLInfo)
	needJobs = !companyfetch.IsFresh(c.JobsFetchedAt, companyfetch.TTLJobs)
	needTech = companyfields.RequiresTech(c.Industry) &&
		(!companyfetch.IsFresh(c.TechFetchedAt, companyfetch.TTLTech) ||
			!companyfetch.HasTechDataForIndustry(c.Industry, c.TechStack, c.InfraStack, c.CicdTools, c.DevelopmentStyle))
	needRelations = !companyfetch.IsFresh(c.RelationsFetchedAt, companyfetch.TTLRelations)
	return
}
