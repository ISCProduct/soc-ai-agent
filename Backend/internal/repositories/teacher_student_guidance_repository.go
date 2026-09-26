package repositories

import (
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
)

type TeacherStudentGuidanceRepository struct {
	db *gorm.DB
}

func NewTeacherStudentGuidanceRepository(db *gorm.DB) *TeacherStudentGuidanceRepository {
	return &TeacherStudentGuidanceRepository{db: db}
}

func (r *TeacherStudentGuidanceRepository) Create(g *models.TeacherStudentGuidance) error {
	return r.db.Create(g).Error
}

// ListActiveByStudent は未dismissの案内を新しい順で返す。
func (r *TeacherStudentGuidanceRepository) ListActiveByStudent(studentUserID uint, limit int) ([]models.TeacherStudentGuidance, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var rows []models.TeacherStudentGuidance
	err := r.db.Where("student_user_id = ? AND dismissed_at IS NULL", studentUserID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *TeacherStudentGuidanceRepository) FindByIDForStudent(id, studentUserID uint) (*models.TeacherStudentGuidance, error) {
	var g models.TeacherStudentGuidance
	err := r.db.Where("id = ? AND student_user_id = ?", id, studentUserID).First(&g).Error
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *TeacherStudentGuidanceRepository) Dismiss(id, studentUserID uint) error {
	now := time.Now()
	return r.db.Model(&models.TeacherStudentGuidance{}).
		Where("id = ? AND student_user_id = ? AND dismissed_at IS NULL", id, studentUserID).
		Updates(map[string]any{"dismissed_at": now, "updated_at": now}).Error
}
