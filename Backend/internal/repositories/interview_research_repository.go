package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
)

type InterviewResearchRepository struct {
	db *gorm.DB
}

func NewInterviewResearchRepository(db *gorm.DB) *InterviewResearchRepository {
	return &InterviewResearchRepository{db: db}
}

func (r *InterviewResearchRepository) FindFresh(ctx context.Context, companyKey, position string, now time.Time) ([]string, bool, error) {
	var row models.InterviewResearchSnapshot
	err := r.db.WithContext(ctx).
		Where("company_key = ? AND position = ? AND expires_at > ?", companyKey, position, now).
		Order("id DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var questions []string
	if err := json.Unmarshal([]byte(row.QuestionsJSON), &questions); err != nil {
		return nil, false, err
	}
	return questions, true, nil
}

func (r *InterviewResearchRepository) Save(ctx context.Context, companyKey, position string, questions []string, fetchedAt, expiresAt time.Time) error {
	raw, err := json.Marshal(questions)
	if err != nil {
		return err
	}
	var row models.InterviewResearchSnapshot
	err = r.db.WithContext(ctx).
		Where("company_key = ? AND position = ?", companyKey, position).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.InterviewResearchSnapshot{
			CompanyKey:    companyKey,
			Position:      position,
			QuestionsJSON: string(raw),
			FetchedAt:     fetchedAt,
			ExpiresAt:     expiresAt,
		}
		return r.db.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	row.QuestionsJSON = string(raw)
	row.FetchedAt = fetchedAt
	row.ExpiresAt = expiresAt
	return r.db.WithContext(ctx).Save(&row).Error
}
