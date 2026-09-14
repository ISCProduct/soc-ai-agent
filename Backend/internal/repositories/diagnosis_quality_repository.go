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

// ListRecent は直近のレポートを返す。schoolID が非nilならその学校の生徒に限定する。
func (r *DiagnosisQualityRepository) ListRecent(limit int, schoolID *uint) ([]models.DiagnosisQualityReport, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := r.db.Model(&models.DiagnosisQualityReport{}).
		Joins("JOIN users ON users.id = diagnosis_quality_reports.user_id").
		Order("diagnosis_quality_reports.updated_at DESC").
		Limit(limit)
	if schoolID != nil {
		q = q.Where("users.school_id = ?", *schoolID)
	}
	var rows []models.DiagnosisQualityReport
	err := q.Find(&rows).Error
	return rows, err
}
