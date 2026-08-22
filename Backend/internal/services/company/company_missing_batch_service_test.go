package company

import (
	"Backend/internal/config"
	"Backend/internal/models"
	"context"
	"strings"
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

func TestMissingBatchDefaults(t *testing.T) {
	if defaultMissingBatchConcurrency != 8 {
		t.Fatalf("default concurrency=%d want 8", defaultMissingBatchConcurrency)
	}
	if got := config.MissingBatchDefaultLimit(); got != 30 {
		t.Fatalf("default limit=%d want 30", got)
	}
	if got := config.MissingBatchMaxConcurrency(); got != 8 {
		t.Fatalf("max concurrency=%d want 8", got)
	}
}

func TestRun_UsesDefaultConcurrencyAndLimit(t *testing.T) {
	svc := NewCompanyMissingBatchService(&missingBatchRepoStub{}, nil, nil, nil, nil)
	result, err := svc.Run(context.Background(), MissingBatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Concurrency != defaultMissingBatchConcurrency {
		t.Fatalf("concurrency=%d want %d", result.Concurrency, defaultMissingBatchConcurrency)
	}
	if result.Limit != config.MissingBatchDefaultLimit() {
		t.Fatalf("limit=%d want %d", result.Limit, config.MissingBatchDefaultLimit())
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
	if result.StopReason != "no_fills" {
		t.Fatalf("stop_reason=%q want no_fills", result.StopReason)
	}
}

func TestMissingBatchStopReason(t *testing.T) {
	tests := []struct {
		name string
		in   *MissingBatchResult
		want string
	}{
		{"nil", nil, "empty_result"},
		{"dry_run", &MissingBatchResult{DryRun: true, Processed: 6}, "dry_run"},
		{"no_candidates", &MissingBatchResult{Limit: 6}, "no_candidates"},
		{"all_failed", &MissingBatchResult{Processed: 6, Errors: 6, Limit: 6}, "all_failed"},
		{"no_fills", &MissingBatchResult{Processed: 6, Skipped: 6, Limit: 6}, "no_fills"},
		{"partial_wave", &MissingBatchResult{Processed: 2, InfoOK: 2, Limit: 6}, "partial_wave"},
		{"wave_full", &MissingBatchResult{Processed: 6, InfoOK: 3, Limit: 6}, "wave_full"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := missingBatchStopReason(tt.in); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestMissingBatchFailureLog(t *testing.T) {
	got := missingBatchFailureLog([]MissingBatchItem{
		{CompanyID: 1, Name: "A社", Error: "info: budget"},
		{CompanyID: 2, Name: "B社", Error: ""},
		{CompanyID: 3, Name: "C社", Error: "openai client is nil"},
	}, 20)
	if !strings.Contains(got, "id=1 A社: info: budget") {
		t.Fatalf("missing first failure: %s", got)
	}
	if !strings.Contains(got, "id=3 C社: openai client is nil") {
		t.Fatalf("missing third failure: %s", got)
	}
	if strings.Contains(got, "B社") {
		t.Fatalf("empty error should be omitted: %s", got)
	}
}

func TestEnrichGapsWithAI_KeepsGBizWhenSearchFails(t *testing.T) {
	f := NewCompanyInfoFetcher(nil, nil)
	base := &CompanyInfoResult{
		Location:   "東京都千代田区",
		WebsiteURL: "https://example.co.jp",
		Source:     "gbizinfo",
		ModelUsed:  "gbizinfo",
	}
	got, err := f.enrichGapsWithAI(context.Background(), "テスト株式会社", "", base)
	if err != nil {
		t.Fatalf("gBiz result must not be discarded: %v", err)
	}
	if got.Location != "東京都千代田区" || got.WebsiteURL != "https://example.co.jp" {
		t.Fatalf("lost gbiz fields: %+v", got)
	}
	if !strings.Contains(got.ModelUsed, "ai_enrich_failed") {
		t.Fatalf("model_used=%q", got.ModelUsed)
	}
}
