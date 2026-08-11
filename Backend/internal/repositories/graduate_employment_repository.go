package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

type GraduateEmploymentRepository struct {
	db *gorm.DB
}

func NewGraduateEmploymentRepository(db *gorm.DB) *GraduateEmploymentRepository {
	return &GraduateEmploymentRepository{db: db}
}

func (r *GraduateEmploymentRepository) Create(entry *models.GraduateEmployment) error {
	return r.db.Create(entry).Error
}

func (r *GraduateEmploymentRepository) FindByID(id uint) (*models.GraduateEmployment, error) {
	var entry models.GraduateEmployment
	err := r.db.Preload("Company").Preload("JobPosition").First(&entry, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &entry, err
}

func (r *GraduateEmploymentRepository) Update(entry *models.GraduateEmployment) error {
	return r.db.Save(entry).Error
}

// List は卒業生の就職情報一覧を返す。schoolID が非nilの場合は個別校で絞り込む。
func (r *GraduateEmploymentRepository) List(companyID, schoolID *uint, limit int) ([]models.GraduateEmployment, error) {
	if limit <= 0 {
		limit = 50
	}
	var entries []models.GraduateEmployment
	query := r.db.Preload("Company").Preload("JobPosition")
	if companyID != nil {
		query = query.Where("company_id = ?", *companyID)
	}
	if schoolID != nil {
		query = query.Where("school_id = ?", *schoolID)
	}
	err := query.Order("created_at desc").Limit(limit).Find(&entries).Error
	return entries, err
}
