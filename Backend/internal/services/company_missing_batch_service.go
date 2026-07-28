package services

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"context"
	"fmt"
	"log"
	"strings"
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
		item.NeedInfo = !companyfetch.IsFresh(c.InfoFetchedAt, companyfetch.TTLInfo) ||
			strings.TrimSpace(c.Description) == "" || strings.TrimSpace(c.WebsiteURL) == ""
		item.NeedJobs = !companyfetch.IsFresh(c.JobsFetchedAt, companyfetch.TTLJobs)
		item.NeedTech = !companyfetch.IsFresh(c.TechFetchedAt, companyfetch.TTLTech) ||
			strings.TrimSpace(c.TechStack) == ""
		item.NeedRelations = !companyfetch.IsFresh(c.RelationsFetchedAt, companyfetch.TTLRelations)
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
	if item.NeedInfo {
		if s.infoFetcher == nil {
			item.InfoStatus = "skipped_no_fetcher"
			result.Skipped++
		} else if _, err := s.infoFetcher.FetchAndSave(ctx, item.CompanyID, false); err != nil {
			item.InfoStatus = "error"
			item.Error = err.Error()
			result.Errors++
		} else {
			item.InfoStatus = "ok"
			result.InfoOK++
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
		} else {
			item.JobsStatus = fmt.Sprintf("ok(%d)", len(positions))
			result.JobsOK++
		}
	}
	if item.NeedTech {
		if s.techFetcher == nil {
			item.TechStatus = "skipped_no_fetcher"
			result.Skipped++
		} else if _, err := s.techFetcher.FetchAndSave(ctx, item.CompanyID, false); err != nil {
			item.TechStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			result.Errors++
		} else {
			item.TechStatus = "ok"
			result.TechOK++
		}
	}
	if item.NeedRelations {
		if s.relationsFetcher == nil {
			item.RelationsStatus = "skipped_no_fetcher"
			result.Skipped++
		} else if _, err := s.relationsFetcher.FetchAndSave(ctx, item.CompanyID, false); err != nil {
			item.RelationsStatus = "error"
			if item.Error == "" {
				item.Error = err.Error()
			}
			result.Errors++
		} else {
			item.RelationsStatus = "ok"
			result.RelationsOK++
		}
	}
}

// MissingNeedsFromCompany は単体テスト用の不足判定ヘルパ。
func MissingNeedsFromCompany(c *models.Company) (needInfo, needJobs, needTech, needRelations bool) {
	if c == nil {
		return false, false, false, false
	}
	needInfo = !companyfetch.IsFresh(c.InfoFetchedAt, companyfetch.TTLInfo) ||
		strings.TrimSpace(c.Description) == "" || strings.TrimSpace(c.WebsiteURL) == ""
	needJobs = !companyfetch.IsFresh(c.JobsFetchedAt, companyfetch.TTLJobs)
	needTech = !companyfetch.IsFresh(c.TechFetchedAt, companyfetch.TTLTech) ||
		strings.TrimSpace(c.TechStack) == ""
	needRelations = !companyfetch.IsFresh(c.RelationsFetchedAt, companyfetch.TTLRelations)
	return
}
