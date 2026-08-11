package services

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"context"
	"testing"
	"time"
)

// matchingCompanyRepo は CalculateMatching 用の最小スタブ。
type matchingCompanyRepo struct {
	companies []models.Company
	profiles  map[uint]*models.CompanyWeightProfile
}

func (s *matchingCompanyRepo) FindAllPublished(int, int) ([]models.Company, error) {
	return s.companies, nil
}
func (s *matchingCompanyRepo) GetWeightProfilesByCompanyIDs([]uint) (map[uint]*models.CompanyWeightProfile, error) {
	return s.profiles, nil
}
func (s *matchingCompanyRepo) FindAllActive(int, int) ([]models.Company, error) { return nil, nil }
func (s *matchingCompanyRepo) FindAllActiveNames(string) ([]models.CompanyName, error) {
	return nil, nil
}
func (s *matchingCompanyRepo) CountActive() (int64, error) { return 0, nil }
func (s *matchingCompanyRepo) ListActiveFiltered(int, int, string, string, string, string, string, *uint) ([]models.Company, int64, error) {
	return nil, 0, nil
}
func (s *matchingCompanyRepo) ListActiveIndustries() ([]string, error) { return nil, nil }
func (s *matchingCompanyRepo) CountPublished() (int64, error)          { return int64(len(s.companies)), nil }
func (s *matchingCompanyRepo) FindByID(uint) (*models.Company, error)  { return nil, nil }
func (s *matchingCompanyRepo) FindByIDs([]uint) ([]models.Company, error) {
	return nil, nil
}
func (s *matchingCompanyRepo) FindByName(string) (*models.Company, error)            { return nil, nil }
func (s *matchingCompanyRepo) FindByCorporateNumber(string) (*models.Company, error) { return nil, nil }
func (s *matchingCompanyRepo) GetWeightProfile(uint, *uint) (*models.CompanyWeightProfile, error) {
	return nil, nil
}
func (s *matchingCompanyRepo) Create(*models.Company) error { return nil }
func (s *matchingCompanyRepo) Update(*models.Company) error { return nil }
func (s *matchingCompanyRepo) FindJobPositionByCompanyAndTitle(uint, string) (*models.CompanyJobPosition, error) {
	return nil, nil
}
func (s *matchingCompanyRepo) FindJobPositionByID(uint) (*models.CompanyJobPosition, error) {
	return nil, nil
}
func (s *matchingCompanyRepo) CreateJobPosition(*models.CompanyJobPosition) error { return nil }
func (s *matchingCompanyRepo) UpdateJobPosition(*models.CompanyJobPosition) error { return nil }
func (s *matchingCompanyRepo) FindJobPositionsByCompany(uint) ([]models.CompanyJobPosition, error) {
	return nil, nil
}
func (s *matchingCompanyRepo) ListJobPositions(*uint, *uint, int) ([]models.CompanyJobPosition, error) {
	return nil, nil
}
func (s *matchingCompanyRepo) CreateOrUpdateWeightProfile(*models.CompanyWeightProfile) error {
	return nil
}
func (s *matchingCompanyRepo) CountWeightProfiles() (int64, error) { return int64(len(s.profiles)), nil }
func (s *matchingCompanyRepo) ListPublishedL1WarmCandidates(int, time.Duration) ([]models.CompanyL1WarmRow, error) {
	return nil, nil
}
func (s *matchingCompanyRepo) CountL1Coverage(time.Duration) (*models.L1CoverageStats, error) {
	return nil, nil
}
func (s *matchingCompanyRepo) ListActiveMissingFetchCandidates(int, bool) ([]models.Company, error) {
	return nil, nil
}

type matchingScoreRepo struct {
	scores []entity.UserWeightScore
}

func (s *matchingScoreRepo) SetScore(uint, string, string, int) error  { return nil }
func (s *matchingScoreRepo) AddScore(uint, string, string, int) error  { return nil }
func (s *matchingScoreRepo) FindByUserAndSession(uint, string) ([]entity.UserWeightScore, error) {
	return s.scores, nil
}
func (s *matchingScoreRepo) FindTopCategories(uint, string, int) ([]entity.UserWeightScore, error) {
	return nil, nil
}
func (s *matchingScoreRepo) FindByUserSessionAndCategory(uint, string, string) (*entity.UserWeightScore, error) {
	return nil, nil
}
func (s *matchingScoreRepo) CountByUserAndSession(uint, string) (int64, error) {
	return int64(len(s.scores)), nil
}

type matchingMatchRepo struct {
	batchCalls int
	saved      int
}

func (s *matchingMatchRepo) CreateOrUpdate(*entity.UserCompanyMatch) error { return nil }
func (s *matchingMatchRepo) CreateOrUpdateBatch(matches []*entity.UserCompanyMatch) (int, error) {
	s.batchCalls++
	s.saved = len(matches)
	return len(matches), nil
}
func (s *matchingMatchRepo) FindTopMatchesByUserAndSession(uint, string, int) ([]*entity.UserCompanyMatch, error) {
	return nil, nil
}
func (s *matchingMatchRepo) FindByID(uint) (*entity.UserCompanyMatch, error) { return nil, nil }
func (s *matchingMatchRepo) MarkAsViewed(uint) error                        { return nil }
func (s *matchingMatchRepo) ToggleFavorite(uint) error                      { return nil }
func (s *matchingMatchRepo) MarkAsApplied(uint) error                       { return nil }
func (s *matchingMatchRepo) FindFavoritesByUser(uint, string) ([]*entity.UserCompanyMatch, error) {
	return nil, nil
}
func (s *matchingMatchRepo) GetMatchStatistics(uint, string) (map[string]any, error) {
	return nil, nil
}

func TestCalculateMatching_UsesBatchProfileAndUpsert(t *testing.T) {
	companyRepo := &matchingCompanyRepo{
		companies: []models.Company{
			{ID: 1, Name: "A社", Industry: "IT"},
			{ID: 2, Name: "B社", Industry: "製造"},
			{ID: 3, Name: "C社", Industry: "IT"}, // プロファイルなし → スキップ
		},
		profiles: map[uint]*models.CompanyWeightProfile{
			1: {CompanyID: 1, TechnicalOrientation: 80},
			2: {CompanyID: 2, TechnicalOrientation: 40},
		},
	}
	scoreRepo := &matchingScoreRepo{
		scores: []entity.UserWeightScore{
			{WeightCategory: "技術志向", Score: 70},
		},
	}
	matchRepo := &matchingMatchRepo{}

	svc := NewMatchingService(scoreRepo, companyRepo, matchRepo, nil)
	if err := svc.CalculateMatching(context.Background(), 10, "sess-1"); err != nil {
		t.Fatalf("CalculateMatching: %v", err)
	}
	if matchRepo.batchCalls != 1 {
		t.Fatalf("CreateOrUpdateBatch calls=%d want 1", matchRepo.batchCalls)
	}
	if matchRepo.saved != 2 {
		t.Fatalf("saved matches=%d want 2 (skip company without profile)", matchRepo.saved)
	}
}
