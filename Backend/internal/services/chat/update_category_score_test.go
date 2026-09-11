package chat

import (
	"errors"
	"testing"

	"Backend/domain/entity"

	"gorm.io/gorm"
)

// mockWeightScoreRepo は updateCategoryScore のテスト用最小モック。
type mockWeightScoreRepo struct {
	findResult *entity.UserWeightScore
	findErr    error
	setCalled  bool
	addCalled  bool
	addDelta   int
}

func (m *mockWeightScoreRepo) SetScore(userID uint, sessionID, category string, absoluteScore int) error {
	m.setCalled = true
	return nil
}
func (m *mockWeightScoreRepo) AddScore(userID uint, sessionID, category string, delta int) error {
	m.addCalled = true
	m.addDelta = delta
	return nil
}
func (m *mockWeightScoreRepo) FindByUserAndSession(userID uint, sessionID string) ([]entity.UserWeightScore, error) {
	return nil, nil
}
func (m *mockWeightScoreRepo) FindTopCategories(userID uint, sessionID string, limit int) ([]entity.UserWeightScore, error) {
	return nil, nil
}
func (m *mockWeightScoreRepo) FindByUserSessionAndCategory(userID uint, sessionID, category string) (*entity.UserWeightScore, error) {
	return m.findResult, m.findErr
}
func (m *mockWeightScoreRepo) CountByUserAndSession(userID uint, sessionID string) (int64, error) {
	return 0, nil
}

// #923: FindByUserSessionAndCategoryが一時的なDBエラー(レコード未存在以外)を返した場合、
// 新規作成にフォールバックせずエラーを伝播しなければならない(重複行の防止)。
func TestUpdateCategoryScore_PropagatesNonNotFoundError(t *testing.T) {
	repo := &mockWeightScoreRepo{findErr: errors.New("connection reset")}
	s := &ChatService{userWeightScoreRepo: repo}

	err := s.updateCategoryScore(1, "session-1", "技術志向", 80)
	if err == nil {
		t.Fatal("expected error to propagate, got nil")
	}
	if repo.setCalled {
		t.Fatal("SetScore should not be called when a non-not-found error occurs")
	}
}

func TestUpdateCategoryScore_CreatesOnRecordNotFound(t *testing.T) {
	repo := &mockWeightScoreRepo{findErr: gorm.ErrRecordNotFound}
	s := &ChatService{userWeightScoreRepo: repo}

	if err := s.updateCategoryScore(1, "session-1", "技術志向", 80); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.setCalled {
		t.Fatal("expected SetScore to be called on record-not-found")
	}
}

func TestUpdateCategoryScore_UpdatesExisting(t *testing.T) {
	repo := &mockWeightScoreRepo{findResult: &entity.UserWeightScore{Score: 50}}
	s := &ChatService{userWeightScoreRepo: repo}

	if err := s.updateCategoryScore(1, "session-1", "技術志向", 80); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.addCalled {
		t.Fatal("expected AddScore to be called for existing record")
	}
	// 移動平均 = 50*0.7 + 80*0.3 = 59、差分delta = 9
	if repo.addDelta != 9 {
		t.Fatalf("addDelta = %d, want 9", repo.addDelta)
	}
}
