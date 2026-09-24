package schoolapproval

import (
	"errors"

	"Backend/internal/models"
)

// ErrAlreadySuppressed は既に個別停止済みの求人を再度停止しようとしたとき。
var ErrAlreadySuppressed = errors.New("この求人は既に停止済みです")

// suppressionRepo は個別求人停止の永続化。
type suppressionRepo interface {
	ListBySchool(schoolID uint) ([]models.SchoolJobSuppression, error)
	Create(s *models.SchoolJobSuppression) error
	Delete(schoolID, jobPositionID uint) error
}

// SuppressionService は学校ごとの個別求人停止（#1508）。
type SuppressionService struct {
	repo suppressionRepo
}

func NewSuppressionService(repo suppressionRepo) *SuppressionService {
	return &SuppressionService{repo: repo}
}

// List は学校の個別停止一覧を返す。
func (s *SuppressionService) List(schoolID uint) ([]models.SchoolJobSuppression, error) {
	return s.repo.ListBySchool(schoolID)
}

// Suppress は求人を学校向けに個別停止する。
func (s *SuppressionService) Suppress(schoolID, jobPositionID, actorID uint, reason string) (*models.SchoolJobSuppression, error) {
	sup := &models.SchoolJobSuppression{
		SchoolID:      schoolID,
		JobPositionID: jobPositionID,
		SuppressedBy:  actorID,
		Reason:        reason,
	}
	if err := s.repo.Create(sup); err != nil {
		if isDuplicateEntryErr(err) {
			return nil, ErrAlreadySuppressed
		}
		return nil, err
	}
	return sup, nil
}

// Unsuppress は個別停止を解除する（べき等）。
func (s *SuppressionService) Unsuppress(schoolID, jobPositionID uint) error {
	return s.repo.Delete(schoolID, jobPositionID)
}
