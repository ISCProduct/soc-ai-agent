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
	if err := fillOrganizationID(r.db, v.UserID, &v.OrganizationID); err != nil {
		return err
	}
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

// RetentionDueVideo は保持期間を過ぎた録画。ObjectKey は S3 のキー（空ならファイルが無い）。
type RetentionDueVideo struct {
	ID        uint   `gorm:"column:id"`
	ObjectKey string `gorm:"column:object_key"`
}

// RetentionGraceUntil は、公開から 30 日の猶予の終わり。行が無ければゼロ値。
func (r *InterviewVideoRepository) RetentionGraceUntil(ctx context.Context) (time.Time, error) {
	var row struct {
		GraceUntil time.Time `gorm:"column:grace_until"`
	}
	err := r.db.WithContext(ctx).Raw("SELECT `grace_until` FROM `interview_video_retention_grace` WHERE `id` = 1").Scan(&row).Error
	return row.GraceUntil, err
}

// ListRetentionDue は、学校の保持日数を過ぎた在籍者の録画を返す。
// 期限なしの学校は対象外。学校が無い学生は 90 日。退会者は退会の削除に任せる。
//
// 判断(SOCAIAGENT-671): users.school_id が NULL の学生(学校に未所属、または学校が削除済み)は、
// 既定の 90 日で削除する。保持期間を選べるのは学校だけで、学校に紐づかない録画を「期限なし」で
// 残す根拠が無いため、意図して 90 日にしている。school_id の一括補完(backfill)は行わない。
// 後から学校に所属した場合は、その学校の設定が次回の実行から適用される。
func (r *InterviewVideoRepository) ListRetentionDue(ctx context.Context, now time.Time, limit int) ([]RetentionDueVideo, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []RetentionDueVideo
	err := r.db.WithContext(ctx).Raw(`
SELECT v.id AS id, v.drive_file_id AS object_key
FROM interview_videos AS v
INNER JOIN users AS u ON u.id = v.user_id AND u.withdrawn_at IS NULL
LEFT JOIN schools AS s ON s.id = u.school_id
WHERE (u.school_id IS NULL OR s.id IS NULL OR s.interview_video_retention_days IS NOT NULL)
  AND COALESCE(v.uploaded_at, v.created_at) < DATE_SUB(?, INTERVAL IF(u.school_id IS NULL OR s.id IS NULL, 90, s.interview_video_retention_days) DAY)
ORDER BY v.id ASC
LIMIT ?`, now, limit).Scan(&rows).Error
	return rows, err
}

func (r *InterviewVideoRepository) DeleteInterviewVideo(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.InterviewVideo{}, id).Error
}
