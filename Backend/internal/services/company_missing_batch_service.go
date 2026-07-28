package services

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"context"
	"fmt"
	"log"
)

const (
	defaultMissingBatchLimit = 20
	maxMissingBatchLimit     = 50
)

// MissingBatchOptions は企業管理全体の不足データ一括取得オプション。
type MissingBatchOptions struct {
	Limit  int  `json:"limit"`
	DryRun bool `json:"dry_run"`
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
	DryRun     bool               `json:"dry_run"`
	Limit      int                `json:"limit"`
	CandidateN int                `json:"candidate_n"`
	Processed  int                `json:"processed"`
	InfoOK     int                `json:"info_ok"`
	JobsOK     int                `json:"jobs_ok"`
	TechOK     int                `json:"tech_ok"`
	RelationsOK int               `json:"relations_ok"`
	Skipped    int                `json:"skipped"`
	Errors     int                `json:"errors"`
	Items      []MissingBatchItem `json:"items"`
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

// Run は不足企業を最大 Limit 社まで処理する（force なし＝TTL内はスキップ）。
func (s *CompanyMissingBatchService) Run(ctx context.Context, opts MissingBatchOptions) (*MissingBatchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = defaultMissingBatchLimit
	}
	if opts.Limit > maxMissingBatchLimit {
		opts.Limit = maxMissingBatchLimit
	}

	candidates, err := s.repo.ListActiveMissingFetchCandidates(opts.Limit * 3)
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
		if !item.NeedJobs {
			if jobs, err := s.repo.ListJobPositions(&c.ID, 1); err == nil && len(jobs) == 0 {
				item.NeedJobs = true
			}
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
		DryRun:     opts.DryRun,
		Limit:      opts.Limit,
		CandidateN: len(items),
		Items:      items,
	}
	if opts.DryRun {
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
		s.processItem(ctx, item, result)
	}

	log.Printf(
		"fetch_missing_batch: processed=%d info_ok=%d jobs_ok=%d tech_ok=%d relations_ok=%d errors=%d",
		result.Processed, result.InfoOK, result.JobsOK, result.TechOK, result.RelationsOK, result.Errors,
	)
	return result, nil
}

func (s *CompanyMissingBatchService) processItem(ctx context.Context, item *MissingBatchItem, result *MissingBatchResult) {
	// 主3種: 基本情報 → 技術 → ビジネス関係。求人は最後。
	if item.NeedInfo {
		if s.infoFetcher == nil {
			item.InfoStatus = "skipped_no_fetcher"
			result.Skipped++
		} else if res, err := s.infoFetcher.FetchAndSave(ctx, item.CompanyID, false); err != nil {
			item.InfoStatus = "error"
			item.Error = err.Error()
			result.Errors++
		} else if res != nil && res.FromCache {
			item.InfoStatus = "skipped_cache"
			result.Skipped++
		} else if company, err := s.repo.FindByID(item.CompanyID); err == nil &&
			companyfetch.HasBasicInfo(company.Description, company.WebsiteURL) {
			item.InfoStatus = "ok"
			result.InfoOK++
		} else {
			item.InfoStatus = "empty"
			result.Skipped++
		}
	}
	if item.NeedTech {
		if s.techFetcher == nil {
			item.TechStatus = "skipped_no_fetcher"
			result.Skipped++
		} else if res, err := s.techFetcher.FetchAndSave(ctx, item.CompanyID, false); err != nil {
			item.TechStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			result.Errors++
		} else if res != nil && res.FromCache {
			item.TechStatus = "skipped_cache"
			result.Skipped++
		} else if company, err := s.repo.FindByID(item.CompanyID); err == nil &&
			companyfetch.HasTechData(company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
			item.TechStatus = "ok"
			result.TechOK++
		} else {
			item.TechStatus = "empty"
			result.Skipped++
		}
	}
	if item.NeedRelations {
		if s.relationsFetcher == nil {
			item.RelationsStatus = "skipped_no_fetcher"
			result.Skipped++
		} else if res, err := s.relationsFetcher.FetchAndSave(ctx, item.CompanyID, false); err != nil {
			item.RelationsStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			result.Errors++
		} else if res != nil && res.FromCache {
			item.RelationsStatus = "skipped_cache"
			result.Skipped++
		} else if s.relationsFetcher.HasStoredData(item.CompanyID) {
			item.RelationsStatus = "ok"
			result.RelationsOK++
		} else {
			item.RelationsStatus = "empty"
			result.Skipped++
		}
	}
	if item.NeedJobs {
		if s.jobFetcher == nil {
			item.JobsStatus = "skipped_no_fetcher"
			result.Skipped++
		} else if positions, err := s.jobFetcher.FetchAndSaveJobs(ctx, item.CompanyID, false); err != nil {
			item.JobsStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			result.Errors++
		} else if len(positions) == 0 {
			item.JobsStatus = "empty"
			result.Skipped++
		} else {
			item.JobsStatus = fmt.Sprintf("ok(%d)", len(positions))
			result.JobsOK++
		}
	}
}

// MissingNeedsFromCompany は不足判定ヘルパ（基本情報・技術・ビジネス関係・求人）。
// タイムスタンプが新しくても、中身が空なら不足とみなす。
// 関係の DB 実体欠落は Run() 側で HasStoredData により追加判定する。
func MissingNeedsFromCompany(c *models.Company) (needInfo, needJobs, needTech, needRelations bool) {
	if c == nil {
		return false, false, false, false
	}
	needInfo = !companyfetch.IsFresh(c.InfoFetchedAt, companyfetch.TTLInfo) ||
		!companyfetch.HasBasicInfo(c.Description, c.WebsiteURL)
	needJobs = !companyfetch.IsFresh(c.JobsFetchedAt, companyfetch.TTLJobs)
	needTech = !companyfetch.IsFresh(c.TechFetchedAt, companyfetch.TTLTech) ||
		!companyfetch.HasTechData(c.TechStack, c.InfraStack, c.CicdTools, c.DevelopmentStyle)
	needRelations = !companyfetch.IsFresh(c.RelationsFetchedAt, companyfetch.TTLRelations)
	return
}
