package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

type InterviewQuestionStateRepository struct {
	db *gorm.DB
}

func NewInterviewQuestionStateRepository(db *gorm.DB) *InterviewQuestionStateRepository {
	return &InterviewQuestionStateRepository{db: db}
}

func (r *InterviewQuestionStateRepository) CountBySessionID(sessionID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.InterviewQuestionState{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}

func (r *InterviewQuestionStateRepository) CreateBatch(states []models.InterviewQuestionState) error {
	if len(states) == 0 {
		return nil
	}
	return r.db.Create(&states).Error
}

func (r *InterviewQuestionStateRepository) Create(state *models.InterviewQuestionState) error {
	return r.db.Create(state).Error
}

func (r *InterviewQuestionStateRepository) Update(state *models.InterviewQuestionState) error {
	return r.db.Save(state).Error
}

func (r *InterviewQuestionStateRepository) FindBySessionID(sessionID uint) ([]models.InterviewQuestionState, error) {
	var states []models.InterviewQuestionState
	err := r.db.Where("session_id = ?", sessionID).Order("sort_order ASC, id ASC").Find(&states).Error
	return states, err
}

func (r *InterviewQuestionStateRepository) FindLatestAsked(sessionID uint) (*models.InterviewQuestionState, error) {
	var state models.InterviewQuestionState
	err := r.db.Where("session_id = ? AND status = ?", sessionID, "asked").
		Order("id DESC").
		First(&state).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}
