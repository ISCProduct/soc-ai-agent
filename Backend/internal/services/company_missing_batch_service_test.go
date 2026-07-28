package services

import (
	"Backend/internal/models"
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestMissingNeedsFromCompany(t *testing.T) {
	now := time.Now()
	stale := now.Add(-200 * 24 * time.Hour)

	fresh := &models.Company{
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

	empty := &models.Company{}
	info, jobs, tech, rel = MissingNeedsFromCompany(empty)
	if !info || !jobs || !tech || !rel {
		t.Fatalf("empty company should need all, got info=%v jobs=%v tech=%v rel=%v", info, jobs, tech, rel)
	}

	staleInfo := &models.Company{
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
		t.Fatal("fresh InfoFetchedAt with empty description/url should still need info")
	}

	// infra / 開発手法のみでは技術取得済みとみなさない
	infraOnly := &models.Company{
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
	if got := clampMissingBatchConcurrency(0); got != defaultMissingBatchConcurrency {
		t.Fatalf("0 -> %d want %d", got, defaultMissingBatchConcurrency)
	}
	if got := clampMissingBatchConcurrency(3); got != 3 {
		t.Fatalf("3 -> %d", got)
	}
	if got := clampMissingBatchConcurrency(100); got != maxMissingBatchConcurrency {
		t.Fatalf("100 -> %d want %d", got, maxMissingBatchConcurrency)
	}
}

type missingBatchRepoStub struct {
	warmRepoStub
	companies []models.Company
	inFlight  atomic.Int32
	peak      atomic.Int32
}

func (s *missingBatchRepoStub) ListActiveMissingFetchCandidates(limit int, _ bool) ([]models.Company, error) {
	if limit < len(s.companies) {
		return s.companies[:limit], nil
	}
	return s.companies, nil
}

func (s *missingBatchRepoStub) FindByID(id uint) (*models.Company, error) {
	cur := s.inFlight.Add(1)
	defer s.inFlight.Add(-1)
	for {
		prev := s.peak.Load()
		if cur <= prev || s.peak.CompareAndSwap(prev, cur) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
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
