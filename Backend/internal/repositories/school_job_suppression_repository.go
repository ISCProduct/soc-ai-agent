package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

// SchoolJobSuppressionRepository は学校ごとの個別求人停止を扱う（#1508）。
type SchoolJobSuppressionRepository struct {
	db *gorm.DB
}

func NewSchoolJobSuppressionRepository(db *gorm.DB) *SchoolJobSuppressionRepository {
	return &SchoolJobSuppressionRepository{db: db}
}

// ListBySchool は学校の個別停止一覧を新しい順で返す。
func (r *SchoolJobSuppressionRepository) ListBySchool(schoolID uint) ([]models.SchoolJobSuppression, error) {
	var list []models.SchoolJobSuppression
	err := r.db.Where("school_id = ?", schoolID).
		Order("created_at DESC, id DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// Create は個別停止を追加する。既に停止済みなら一意制約違反を返す。
func (r *SchoolJobSuppressionRepository) Create(s *models.SchoolJobSuppression) error {
	return r.db.Create(s).Error
}

// Delete は個別停止を解除する。停止していなくてもエラーにしない（べき等）。
func (r *SchoolJobSuppressionRepository) Delete(schoolID, jobPositionID uint) error {
	return r.db.Where("school_id = ? AND job_position_id = ?", schoolID, jobPositionID).
		Delete(&models.SchoolJobSuppression{}).Error
}
