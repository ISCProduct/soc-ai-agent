package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DiagnosisQualityRepository struct {
	db *gorm.DB
}

func NewDiagnosisQualityRepository(db *gorm.DB) *DiagnosisQualityRepository {
	return &DiagnosisQualityRepository{db: db}
}

func (r *DiagnosisQualityRepository) Upsert(report *models.DiagnosisQualityReport) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"confidence", "flags_json", "summary", "raw_json", "updated_at",
		}),
	}).Create(report).Error
}

func (r *DiagnosisQualityRepository) FindByUserAndSession(userID uint, sessionID string) (*models.DiagnosisQualityReport, error) {
	var report models.DiagnosisQualityReport
	err := r.db.Where("user_id = ? AND session_id = ?", userID, sessionID).First(&report).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *DiagnosisQualityRepository) ListRecent(limit int) ([]models.DiagnosisQualityReport, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []models.DiagnosisQualityReport
	err := r.db.Order("updated_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
