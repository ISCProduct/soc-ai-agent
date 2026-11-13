package repositories

import (
	"Backend/internal/models"
	"context"

	"gorm.io/gorm"
)

// ReleaseNoteRepository は repository.ReleaseNoteRepository の GORM 実装。
type ReleaseNoteRepository struct {
	db *gorm.DB
}

func NewReleaseNoteRepository(db *gorm.DB) *ReleaseNoteRepository {
	return &ReleaseNoteRepository{db: db}
}

func (r *ReleaseNoteRepository) DeleteByPRNumber(ctx context.Context, prNumber uint) error {
	return r.db.WithContext(ctx).Where("pr_number = ?", prNumber).Delete(&models.ReleaseNote{}).Error
}

func (r *ReleaseNoteRepository) ListAll(ctx context.Context) ([]models.ReleaseNote, error) {
	var notes []models.ReleaseNote
	if err := r.db.WithContext(ctx).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *ReleaseNoteRepository) Delete(ctx context.Context, note *models.ReleaseNote) error {
	return r.db.WithContext(ctx).Delete(note).Error
}

func (r *ReleaseNoteRepository) ExistsByPRNumber(ctx context.Context, prNumber uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.ReleaseNote{}).Where("pr_number = ?", prNumber).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ReleaseNoteRepository) Create(ctx context.Context, note *models.ReleaseNote) error {
	return r.db.WithContext(ctx).Create(note).Error
}

func (r *ReleaseNoteRepository) ListByAudiences(ctx context.Context, audiences []string, limit int) ([]models.ReleaseNote, error) {
	var notes []models.ReleaseNote
	if err := r.db.WithContext(ctx).
		Where("audience IN ?", audiences).
		Order("merged_at DESC").Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}
