package schoolapproval

import (
	"errors"
	"testing"

	"Backend/internal/models"

	"github.com/go-sql-driver/mysql"
)

type fakeSuppressionRepo struct {
	listed    []models.SchoolJobSuppression
	created   *models.SchoolJobSuppression
	createErr error
	deletedS  uint
	deletedJ  uint
}

func (f *fakeSuppressionRepo) ListBySchool(schoolID uint) ([]models.SchoolJobSuppression, error) {
	return f.listed, nil
}
func (f *fakeSuppressionRepo) Create(s *models.SchoolJobSuppression) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = s
	return nil
}
func (f *fakeSuppressionRepo) Delete(schoolID, jobPositionID uint) error {
	f.deletedS, f.deletedJ = schoolID, jobPositionID
	return nil
}

func TestSuppress_Success(t *testing.T) {
	repo := &fakeSuppressionRepo{}
	svc := NewSuppressionService(repo)
	sup, err := svc.Suppress(3, 55, 99, "不適切な内容")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if sup.SchoolID != 3 || sup.JobPositionID != 55 || sup.SuppressedBy != 99 || sup.Reason != "不適切な内容" {
		t.Errorf("fields = %+v", sup)
	}
}

// TestSuppress_Duplicate は重複停止(1062)を分かりやすいエラーに変換することを検証する。
func TestSuppress_Duplicate(t *testing.T) {
	repo := &fakeSuppressionRepo{createErr: &mysql.MySQLError{Number: 1062}}
	svc := NewSuppressionService(repo)
	_, err := svc.Suppress(3, 55, 99, "")
	if !errors.Is(err, ErrAlreadySuppressed) {
		t.Fatalf("err = %v, want ErrAlreadySuppressed", err)
	}
}

func TestUnsuppress_Passthrough(t *testing.T) {
	repo := &fakeSuppressionRepo{}
	svc := NewSuppressionService(repo)
	if err := svc.Unsuppress(3, 55); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if repo.deletedS != 3 || repo.deletedJ != 55 {
		t.Errorf("deleted (%d,%d), want (3,55)", repo.deletedS, repo.deletedJ)
	}
}
