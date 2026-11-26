package repositories

import (
	"context"
	"errors"

	"Backend/domain/repository"
	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InterviewShadowScoreRepository はシャドー採点の保存先。
type InterviewShadowScoreRepository struct {
	db *gorm.DB
}

func NewInterviewShadowScoreRepository(db *gorm.DB) *InterviewShadowScoreRepository {
	return &InterviewShadowScoreRepository{db: db}
}

// Upsert は同じ面接の再採点で行を上書きする。
func (r *InterviewShadowScoreRepository) Upsert(ctx context.Context, row *models.InterviewShadowScore) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"model_version", "scores_json", "latency_ms", "updated_at"}),
	}).Create(row).Error
}

// FindBySessionID は無ければ repository.ErrNotFound を返す。
func (r *InterviewShadowScoreRepository) FindBySessionID(ctx context.Context, sessionID uint) (*models.InterviewShadowScore, error) {
	var row models.InterviewShadowScore
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
