package repositories

import (
	"context"
	"time"

	"Backend/internal/models"
	"gorm.io/gorm"
)

type InterviewVideoRepository struct {
	db *gorm.DB
}

func NewInterviewVideoRepository(db *gorm.DB) *InterviewVideoRepository {
	return &InterviewVideoRepository{db: db}
}

func (r *InterviewVideoRepository) Create(ctx context.Context, v *models.InterviewVideo) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *InterviewVideoRepository) UpdateStatus(ctx context.Context, id uint, status, errorMessage string, driveFileID, driveFileURL string, uploadedAt *time.Time) error {
	updates := map[string]any{
		"status":         status,
		"error_message":  errorMessage,
		"drive_file_id":  driveFileID,
		"drive_file_url": driveFileURL,
		"uploaded_at":    uploadedAt,
	}
	return r.db.WithContext(ctx).Model(&models.InterviewVideo{}).Where("id = ?", id).Updates(updates).Error
}

func (r *InterviewVideoRepository) FindBySessionID(ctx context.Context, sessionID uint) ([]models.InterviewVideo, error) {
	var videos []models.InterviewVideo
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Find(&videos).Error
	return videos, err
}

func (r *InterviewVideoRepository) FindByID(ctx context.Context, id uint) (*models.InterviewVideo, error) {
	var video models.InterviewVideo
	err := r.db.WithContext(ctx).First(&video, id).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}
