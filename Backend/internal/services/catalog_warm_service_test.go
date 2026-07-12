package services

import (
	"Backend/internal/models"
	"context"
	"testing"
	"time"
)

type warmRepoStub struct {
	rows  []models.CompanyL1WarmRow
	stats *models.L1CoverageStats
}

func (s *warmRepoStub) ListPublishedL1WarmCandidates(limit int, _ time.Duration) ([]models.CompanyL1WarmRow, error) {
	if limit < len(s.rows) {
		return s.rows[:limit], nil
	}
	return s.rows, nil
}

func (s *warmRepoStub) CountL1Coverage(_ time.Duration) (*models.L1CoverageStats, error) {
	return s.stats, nil
}

// 以下は CatalogWarmService が repository.CompanyRepository 全体を要求するため最小スタブ
func (s *warmRepoStub) FindAllActive(int, int) ([]models.Company, error) { return nil, nil }
func (s *warmRepoStub) FindAllActiveNames(string) ([]models.CompanyName, error) {
	return nil, nil
}
func (s *warmRepoStub) CountActive() (int64, error)                                { return 0, nil }
func (s *warmRepoStub) FindAllPublished(int, int) ([]models.Company, error)        { return nil, nil }
func (s *warmRepoStub) CountPublished() (int64, error)                             { return 0, nil }
func (s *warmRepoStub) FindByID(uint) (*models.Company, error)                     { return nil, nil }
func (s *warmRepoStub) FindByName(string) (*models.Company, error)                 { return nil, nil }
func (s *warmRepoStub) FindByCorporateNumber(string) (*models.Company, error)      { return nil, nil }
func (s *warmRepoStub) GetWeightProfile(uint, *uint) (*models.CompanyWeightProfile, error) {
	return nil, nil
}
func (s *warmRepoStub) Create(*models.Company) error { return nil }
func (s *warmRepoStub) Update(*models.Company) error { return nil }
func (s *warmRepoStub) FindJobPositionByCompanyAndTitle(uint, string) (*models.CompanyJobPosition, error) {
	return nil, nil
}
func (s *warmRepoStub) FindJobPositionByID(uint) (*models.CompanyJobPosition, error) { return nil, nil }
func (s *warmRepoStub) CreateJobPosition(*models.CompanyJobPosition) error           { return nil }
func (s *warmRepoStub) UpdateJobPosition(*models.CompanyJobPosition) error           { return nil }
func (s *warmRepoStub) FindJobPositionsByCompany(uint) ([]models.CompanyJobPosition, error) {
	return nil, nil
}
func (s *warmRepoStub) ListJobPositions(*uint, int) ([]models.CompanyJobPosition, error) {
	return nil, nil
}
func (s *warmRepoStub) CreateOrUpdateWeightProfile(*models.CompanyWeightProfile) error { return nil }
func (s *warmRepoStub) CountWeightProfiles() (int64, error)                            { return 0, nil }

func TestL1WarmPriority_SIFirst(t *testing.T) {
	if got := L1WarmPriority("株式会社テスト", "情報サービス・SI"); got != 0 {
		t.Fatalf("SI industry priority=%d want 0", got)
	}
	if got := L1WarmPriority("飲食店ABC", "外食"); got != 1 {
		t.Fatalf("non-IT priority=%d want 1", got)
	}
}

func TestWarmL1_DryRunOrdersSIFirst(t *testing.T) {
	repo := &warmRepoStub{
		rows: []models.CompanyL1WarmRow{
			{Company: models.Company{ID: 1, Name: "飲食A", Industry: "外食"}, HasWeightProfile: false},
			{Company: models.Company{ID: 2, Name: "中小SI株式会社", Industry: "システム開発"}, HasWeightProfile: false},
			{Company: models.Company{ID: 3, Name: "メーカーB", Industry: "製造"}, HasWeightProfile: false},
		},
		stats: &models.L1CoverageStats{PublishedTotal: 3, NeedsWarm: 3},
	}
	svc := NewCatalogWarmService(repo, nil, nil)
	res, err := svc.WarmL1(context.Background(), L1WarmOptions{
		Limit: 2, DryRun: true, IncludeInfo: true, IncludePersona: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.CandidateN != 2 {
		t.Fatalf("candidate_count=%d want 2", res.CandidateN)
	}
	if res.Items[0].CompanyID != 2 {
		t.Fatalf("first=%d want SI company 2", res.Items[0].CompanyID)
	}
}

func TestCoverage_Rates(t *testing.T) {
	repo := &warmRepoStub{
		stats: &models.L1CoverageStats{
			PublishedTotal: 100,
			InfoFresh:      80,
			HasProfile:     50,
			NeedsWarm:      40,
		},
	}
	svc := NewCatalogWarmService(repo, nil, nil)
	cov, err := svc.Coverage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cov.InfoRate != 0.8 {
		t.Fatalf("info_rate=%v want 0.8", cov.InfoRate)
	}
	if cov.ProfileRate != 0.5 {
		t.Fatalf("profile_rate=%v want 0.5", cov.ProfileRate)
	}
	if !cov.BelowTarget {
		t.Fatal("expected below_target")
	}
	if len(cov.Alerts) == 0 {
		t.Fatal("expected alerts")
	}
}
