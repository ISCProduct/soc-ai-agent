package matching

import (
	"errors"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/services/shared"

	"gorm.io/gorm"
)

// stubToggleFavoriteMatchRepo は ToggleFavorite のテスト用最小モック。
type stubToggleFavoriteMatchRepo struct {
	findByIDResult *entity.UserCompanyMatch
	findByIDErr    error
	toggleCalled   bool
}

func (s *stubToggleFavoriteMatchRepo) CreateOrUpdate(*entity.UserCompanyMatch) error { return nil }
func (s *stubToggleFavoriteMatchRepo) CreateOrUpdateBatch([]*entity.UserCompanyMatch) (int, error) {
	return 0, nil
}
func (s *stubToggleFavoriteMatchRepo) FindTopMatchesByUserAndSession(uint, string, int) ([]*entity.UserCompanyMatch, error) {
	return nil, nil
}
func (s *stubToggleFavoriteMatchRepo) FindByID(uint) (*entity.UserCompanyMatch, error) {
	return s.findByIDResult, s.findByIDErr
}
func (s *stubToggleFavoriteMatchRepo) MarkAsViewed(uint) error { return nil }
func (s *stubToggleFavoriteMatchRepo) ToggleFavorite(uint) error {
	s.toggleCalled = true
	return nil
}
func (s *stubToggleFavoriteMatchRepo) MarkAsApplied(uint) error { return nil }
func (s *stubToggleFavoriteMatchRepo) FindFavoritesByUser(uint, string) ([]*entity.UserCompanyMatch, error) {
	return nil, nil
}
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
	repo := &stubToggleFavoriteMatchRepo{findByIDResult: &entity.UserCompanyMatch{ID: 1, UserID: 2}}
	s := &MatchingService{matchRepo: repo}

	err := s.ToggleFavorite(1, 1)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("got %v want shared.ErrForbidden", err)
	}
}

func TestToggleFavorite_SucceedsForOwner(t *testing.T) {
	repo := &stubToggleFavoriteMatchRepo{findByIDResult: &entity.UserCompanyMatch{ID: 1, UserID: 1}}
	s := &MatchingService{matchRepo: repo}

	if err := s.ToggleFavorite(1, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.toggleCalled {
		t.Fatal("expected ToggleFavorite to be called")
	}
}
