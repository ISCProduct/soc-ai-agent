package company

import (
	"Backend/internal/config"
	"Backend/internal/models"
	"context"
	"testing"
	"time"
)

func TestMissingNeedsFromCompany(t *testing.T) {
	now := time.Now()
	stale := now.Add(-200 * 24 * time.Hour)

	fresh := &models.Company{
		Industry:           "IT・ソフトウェア",
		Description:        "概要",
		WebsiteURL:         "https://example.com",
		TechStack:          `["Go"]`,
		InfoFetchedAt:      &now,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	info, jobs, tech, rel := MissingNeedsFromCompany(fresh)
	if info || jobs || tech || rel {
		t.Fatalf("fresh company should need nothing, got info=%v jobs=%v tech=%v rel=%v", info, jobs, tech, rel)
	}

	empty := &models.Company{Industry: "IT・ソフトウェア"}
	info, jobs, tech, rel = MissingNeedsFromCompany(empty)
	if !info || !jobs || !tech || !rel {
		t.Fatalf("empty IT company should need all, got info=%v jobs=%v tech=%v rel=%v", info, jobs, tech, rel)
	}

	financeEmpty := &models.Company{Industry: "金融・保険業"}
	_, _, tech, _ = MissingNeedsFromCompany(financeEmpty)
	if tech {
		t.Fatal("finance company should not need tech stack")
	}

	mfgEmpty := &models.Company{Industry: "製造業"}
	_, _, tech, _ = MissingNeedsFromCompany(mfgEmpty)
	if tech {
		t.Fatal("manufacturing company should not require tech for publish/missing batch")
	}

	staleInfo := &models.Company{
		Industry:           "IT・ソフトウェア",
		Description:        "概要",
		WebsiteURL:         "https://example.com",
		TechStack:          `["Go"]`,
		InfoFetchedAt:      &stale,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	info, jobs, tech, rel = MissingNeedsFromCompany(staleInfo)
	if !info || jobs || tech || rel {
		t.Fatalf("stale info only: info=%v jobs=%v tech=%v rel=%v", info, jobs, tech, rel)
	}

	emptyTech := &models.Company{
		Industry:           "IT・ソフトウェア",
		Description:        "概要",
		WebsiteURL:         "https://example.com",
		TechStack:          "[]",
		InfoFetchedAt:      &now,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	_, _, tech, _ = MissingNeedsFromCompany(emptyTech)
	if !tech {
		t.Fatal("empty tech payload [] should need tech")
	}

	emptyInfoDespiteStamp := &models.Company{
		Industry:           "IT・ソフトウェア",
		Description:        "",
		WebsiteURL:         "",
		TechStack:          `["Go"]`,
		InfoFetchedAt:      &now,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	info, _, _, _ = MissingNeedsFromCompany(emptyInfoDespiteStamp)
	if !info {
		t.Fatal("fresh InfoFetchedAt without description/website should still need info (FE missingAspects)")
	}

	sparseFootprintStamped := &models.Company{
		Industry:           "IT・ソフトウェア",
		Description:        "",
		WebsiteURL:         "https://example.com",
		Location:           "東京都",
		TechStack:          `["Go"]`,
		InfoFetchedAt:      &now,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	info, _, _, _ = MissingNeedsFromCompany(sparseFootprintStamped)
	if !info {
		t.Fatal("stamped sparse basic info (no description) should need refetch to match FE")
	}

	// infra / 開発手法のみでは技術取得済みとみなさない
	infraOnly := &models.Company{
		Industry:           "IT・ソフトウェア",
		Description:        "概要",
		WebsiteURL:         "https://example.com",
		TechStack:          "",
		InfraStack:         `["AWS"]`,
		DevelopmentStyle:   "スクラム",
		InfoFetchedAt:      &now,
		JobsFetchedAt:      &now,
		TechFetchedAt:      &now,
		RelationsFetchedAt: &now,
	}
	_, _, tech, _ = MissingNeedsFromCompany(infraOnly)
	if !tech {
		t.Fatal("infra/style only should still need tech_stack")
	}
}

func TestClampMissingBatchConcurrency(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"未指定はデフォルト", 0, defaultMissingBatchConcurrency},
		{"範囲内はそのまま", 3, 3},
		{"上限でクランプ", 100, config.MissingBatchMaxConcurrency()},
		{"負数はデフォルト", -1, defaultMissingBatchConcurrency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampMissingBatchConcurrency(tt.in); got != tt.want {
				t.Fatalf("%d -> %d want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestClassifyInfoCacheResult(t *testing.T) {
	sparse := &models.Company{
		Description: "",
		WebsiteURL:  "https://example.com",
		Location:    "東京都",
	}
	full := &models.Company{
		Description: "概要",
		WebsiteURL:  "https://example.com",
	}
	empty := &models.Company{}

	tests := []struct {
		name       string
		res        *CompanyInfoResult
		company    *models.Company
		wantStatus string
		wantErr    string
	}{
		{
			name: "予算超過は footprint より先に error",
			res: &CompanyInfoResult{
				FromCache:      true,
				BudgetExceeded: true,
				SkipReason:     "budget",
			},
			company:    sparse,
			wantStatus: "error",
			wantErr:    "info: budget",
		},
		{
			name: "SkipReason=budget のみでも error",
			res: &CompanyInfoResult{
				FromCache:  true,
				SkipReason: "budget",
			},
			company:    sparse,
			wantStatus: "error",
			wantErr:    "info: budget",
		},
		{
			name: "充足キャッシュは skipped_cache",
			res: &CompanyInfoResult{
				FromCache:  true,
				SkipReason: "ttl",
			},
			company:    full,
			wantStatus: "skipped_cache",
		},
		{
			name: "疎データは empty",
			res: &CompanyInfoResult{
				FromCache:  true,
				SkipReason: "ttl",
			},
			company:    sparse,
			wantStatus: "empty",
		},
		{
			name: "手がかりなしは error",
			res: &CompanyInfoResult{
				FromCache:  true,
				SkipReason: "ttl",
			},
			company:    empty,
			wantStatus: "error",
			wantErr:    "info: ttl",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, errMsg := classifyInfoCacheResult(tt.res, tt.company)
			if status != tt.wantStatus {
				t.Fatalf("status=%q want %q", status, tt.wantStatus)
			}
			if errMsg != tt.wantErr {
				t.Fatalf("errMsg=%q want %q", errMsg, tt.wantErr)
			}
		})
	}
}

type missingBatchRepoStub struct {
	warmRepoStub
	companies []models.Company
}

func (s *missingBatchRepoStub) ListActiveMissingFetchCandidates(limit int, _ bool) ([]models.Company, error) {
	if limit < len(s.companies) {
		return s.companies[:limit], nil
	}
	return s.companies, nil
}

func (s *missingBatchRepoStub) FindByID(id uint) (*models.Company, error) {
	for i := range s.companies {
		if s.companies[i].ID == id {
			c := s.companies[i]
			return &c, nil
		}
	}
	return nil, nil
}

func TestCompanyMissingBatchService_RunParallelNilFetchers(t *testing.T) {
	companies := make([]models.Company, 8)
	for i := range companies {
		companies[i] = models.Company{ID: uint(i + 1), Name: "社", DataStatus: "draft"}
	}
	repo := &missingBatchRepoStub{companies: companies}
	svc := NewCompanyMissingBatchService(repo, nil, nil, nil, nil)

	result, err := svc.Run(context.Background(), MissingBatchOptions{
		Limit:       8,
		PrimaryOnly: true,
		Concurrency: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Concurrency != 4 {
		t.Fatalf("concurrency=%d", result.Concurrency)
	}
	if result.Processed != 8 {
		t.Fatalf("processed=%d", result.Processed)
	}
	if result.Skipped < 8 {
		t.Fatalf("skipped=%d want >=8 (nil fetchers)", result.Skipped)
	}
	for _, item := range result.Items {
		if item.InfoStatus != "skipped_no_fetcher" {
			t.Fatalf("info_status=%q", item.InfoStatus)
		}
	}
}
