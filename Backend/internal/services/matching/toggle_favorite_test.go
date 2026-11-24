package matching

import (
	"Backend/internal/models"
	"context"
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
func (s *stubToggleFavoriteMatchRepo) FindFavoritedByUser(context.Context, uint) ([]*models.UserCompanyMatch, error) {
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
func (s *stubToggleFavoriteMatchRepo) UnfavoriteCompany(_ context.Context, userID uint, companyID uint) error {
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

	if err := s.UnfavoriteCompany(context.Background(), 7, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.unfavUser != 7 || repo.unfavCompany != 3 {
		t.Fatalf("unfavorite user=%d company=%d, want 7 / 3", repo.unfavUser, repo.unfavCompany)
	}
}

func TestListFavoriteCompanies_DedupesRepoRows(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &stubToggleFavoriteMatchRepo{favorited: []*models.UserCompanyMatch{
		{ID: 1, CompanyID: 10, MatchScore: 90, CreatedAt: older, UpdatedAt: older.Add(2 * time.Hour), Company: models.Company{ID: 10}},
		{ID: 2, CompanyID: 10, MatchScore: 50, CreatedAt: older.Add(time.Hour), UpdatedAt: older, Company: models.Company{ID: 10}},
	}}
	s := &MatchingService{matchRepo: repo}

	got, total, ids, err := s.ListFavoriteCompanies(context.Background(), 1, 0, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("got %+v, want the newer row only", got)
	}
	if total != 1 || len(ids) != 1 || ids[0] != 10 {
		t.Fatalf("total=%d ids=%v, want 1 / [10]", total, ids)
	}
}

func TestListFavoriteCompanies_OffsetReturnsLaterCompanies(t *testing.T) {
	at := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	rows := make([]*models.UserCompanyMatch, 0, 3)
	for i, companyID := range []uint{30, 10, 20} {
		rows = append(rows, &models.UserCompanyMatch{
			ID:         uint(i + 1),
			CompanyID:  companyID,
			MatchScore: float64(100 - i*10),
			CreatedAt:  at,
			Company:    models.Company{ID: companyID},
		})
	}
	// スコア順は 30, 10, 20。2社目から1社だけ返す。
	repo := &stubToggleFavoriteMatchRepo{favorited: rows}
	s := &MatchingService{matchRepo: repo}

	got, total, ids, err := s.ListFavoriteCompanies(context.Background(), 1, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 || len(ids) != 3 || ids[0] != 30 || ids[2] != 20 {
		t.Fatalf("total=%d ids=%v, want 3社すべて", total, ids)
	}
	if len(got) != 1 || got[0].CompanyID != 10 {
		t.Fatalf("got %+v, want company 10 only", got)
	}

	past, pastTotal, _, err := s.ListFavoriteCompanies(context.Background(), 1, 100, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pastTotal != 3 || len(past) != 0 {
		t.Fatalf("末尾を超えたページは空。total=%d len=%d", pastTotal, len(past))
	}
}
