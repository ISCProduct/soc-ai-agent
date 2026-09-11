package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

type InterviewCompanyQuestionRepository struct {
	db *gorm.DB
}

func NewInterviewCompanyQuestionRepository(db *gorm.DB) *InterviewCompanyQuestionRepository {
	return &InterviewCompanyQuestionRepository{db: db}
}

func (r *InterviewCompanyQuestionRepository) FindByCompanyID(companyID uint) ([]models.InterviewCompanyQuestion, error) {
	var qs []models.InterviewCompanyQuestion
	err := r.db.Where("company_id = ?", companyID).Order("priority ASC, id ASC").Find(&qs).Error
	return qs, err
}

func (r *InterviewCompanyQuestionRepository) FindByCompanyAndPosition(companyID uint, position string) ([]models.InterviewCompanyQuestion, error) {
	var qs []models.InterviewCompanyQuestion
	err := r.db.Where("company_id = ? AND (position = '' OR position = ?)", companyID, position).
		Order("priority ASC, id ASC").Find(&qs).Error
	return qs, err
}

func (r *InterviewCompanyQuestionRepository) FindByID(id uint) (*models.InterviewCompanyQuestion, error) {
	var q models.InterviewCompanyQuestion
	err := r.db.First(&q, id).Error
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *InterviewCompanyQuestionRepository) Create(q *models.InterviewCompanyQuestion) error {
	return r.db.Create(q).Error
}

func (r *InterviewCompanyQuestionRepository) Update(q *models.InterviewCompanyQuestion) error {
	return r.db.Save(q).Error
}

func (r *InterviewCompanyQuestionRepository) Delete(id uint) error {
	return r.db.Delete(&models.InterviewCompanyQuestion{}, id).Error
}
