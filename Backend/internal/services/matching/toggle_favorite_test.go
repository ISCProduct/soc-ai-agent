package matching

import (
	"Backend/internal/models"
	"errors"
	"testing"

	"Backend/internal/services/shared"

	"gorm.io/gorm"
	"time"
)

// stubToggleFavoriteMatchRepo は ToggleFavorite のテスト用最小モック。
type stubToggleFavoriteMatchRepo struct {
	findByIDResult *models.UserCompanyMatch
	findByIDErr    error
	toggleCalled   bool
	favorited      []*models.UserCompanyMatch
	unfavUser      uint
	unfavCompany   uint
}

func (s *stubToggleFavoriteMatchRepo) CreateOrUpdate(*models.UserCompanyMatch) error { return nil }
func (s *stubToggleFavoriteMatchRepo) CreateOrUpdateBatch([]*models.UserCompanyMatch) (int, error) {
	return 0, nil
}
func (s *stubToggleFavoriteMatchRepo) FindTopMatchesByUserAndSession(uint, string, int) ([]*models.UserCompanyMatch, error) {
	return nil, nil
}
func (s *stubToggleFavoriteMatchRepo) FindFavoritedByUser(uint) ([]*models.UserCompanyMatch, error) {
	return s.favorited, nil
}
func (s *stubToggleFavoriteMatchRepo) FindByID(uint) (*models.UserCompanyMatch, error) {
	return s.findByIDResult, s.findByIDErr
}
func (s *stubToggleFavoriteMatchRepo) MarkAsViewed(uint) error { return nil }
func (s *stubToggleFavoriteMatchRepo) ToggleFavorite(uint) error {
	s.toggleCalled = true
	return nil
}
func (s *stubToggleFavoriteMatchRepo) UnfavoriteCompany(userID uint, companyID uint) error {
	s.unfavUser = userID
	s.unfavCompany = companyID
	return nil
}
func (s *stubToggleFavoriteMatchRepo) MarkAsApplied(uint) error { return nil }
func (s *stubToggleFavoriteMatchRepo) GetMatchStatistics(uint, string) (map[string]any, error) {
	return nil, nil
}

// #924: 存在しない(削除済み)match_idを指定した場合、汎用エラーではなく
// shared.ErrNotFoundを返し、コントローラーが404にマッピングできるようにする。
func TestToggleFavorite_NotFoundOnMissingMatch(t *testing.T) {
	repo := &stubToggleFavoriteMatchRepo{findByIDErr: gorm.ErrRecordNotFound}
	s := &MatchingService{matchRepo: repo}

	err := s.ToggleFavorite(999, 1)
	if !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("got %v want shared.ErrNotFound", err)
	}
	if repo.toggleCalled {
		t.Fatal("ToggleFavorite should not be called when the match is not found")
	}
}

func TestToggleFavorite_ForbiddenOnOtherUsersMatch(t *testing.T) {
	repo := &stubToggleFavoriteMatchRepo{findByIDResult: &models.UserCompanyMatch{ID: 1, UserID: 2}}
	s := &MatchingService{matchRepo: repo}

	err := s.ToggleFavorite(1, 1)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want shared.ErrForbidden", err)
	}
}

func TestToggleFavorite_SucceedsForOwner(t *testing.T) {
	repo := &stubToggleFavoriteMatchRepo{findByIDResult: &models.UserCompanyMatch{ID: 1, UserID: 1}}
	s := &MatchingService{matchRepo: repo}

	if err := s.ToggleFavorite(1, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.toggleCalled {
		t.Fatal("expected ToggleFavorite to be called")
	}
}

func TestUnfavoriteCompany_ClearsAcrossSessions(t *testing.T) {
	repo := &stubToggleFavoriteMatchRepo{}
	s := &MatchingService{matchRepo: repo}

	if err := s.UnfavoriteCompany(7, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.unfavUser != 7 || repo.unfavCompany != 3 {
		t.Fatalf("unfavorite user=%d company=%d, want 7 / 3", repo.unfavUser, repo.unfavCompany)
	}
}

func TestListFavoriteCompanies_DedupesRepoRows(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &stubToggleFavoriteMatchRepo{favorited: []*models.UserCompanyMatch{
		{ID: 1, CompanyID: 10, MatchScore: 90, UpdatedAt: older, Company: models.Company{ID: 10}},
		{ID: 2, CompanyID: 10, MatchScore: 50, UpdatedAt: older.Add(time.Hour), Company: models.Company{ID: 10}},
	}}
	s := &MatchingService{matchRepo: repo}

	got, err := s.ListFavoriteCompanies(1, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("got %+v, want the newer row only", got)
	}
}
