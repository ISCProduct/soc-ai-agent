package repositories

import (
	"context"

	"Backend/internal/models"

	"gorm.io/gorm"
)

const growthHistoryLimit = 30

// UserGrowthSnapshotRepository はスコアとフェーズの写しを追記する。
// 何を残すかの判断は services/growth 側。ここは読み書きだけ。
type UserGrowthSnapshotRepository struct {
	db *gorm.DB
}

func NewUserGrowthSnapshotRepository(db *gorm.DB) *UserGrowthSnapshotRepository {
	return &UserGrowthSnapshotRepository{db: db}
}

func (r *UserGrowthSnapshotRepository) Scores(ctx context.Context, userID uint, sessionID string) ([]models.UserWeightScore, error) {
	var rows []models.UserWeightScore
	err := r.db.WithContext(ctx).Where("user_id = ? AND session_id = ?", userID, sessionID).Find(&rows).Error
	return rows, err
}

func (r *UserGrowthSnapshotRepository) Progress(ctx context.Context, userID uint, sessionID string) ([]models.UserAnalysisProgress, error) {
	var rows []models.UserAnalysisProgress
	err := r.db.WithContext(ctx).Preload("Phase").Where("user_id = ? AND session_id = ?", userID, sessionID).Find(&rows).Error
	return rows, err
}

// LatestProgressSession は、そのユーザーの診断フェーズが最後に進んだセッションを返す。
// 進捗が無ければ空文字。
func (r *UserGrowthSnapshotRepository) LatestProgressSession(ctx context.Context, userID uint) (string, error) {
	var row models.UserAnalysisProgress
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at DESC, id DESC").Limit(1).Find(&row).Error
	if err != nil {
		return "", err
	}
	return row.SessionID, nil
}

func (r *UserGrowthSnapshotRepository) Insert(ctx context.Context, row *models.UserGrowthSnapshot) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// ListRecent は新しい順に最大30件返す。
func (r *UserGrowthSnapshotRepository) ListRecent(ctx context.Context, userID uint, limit int) ([]models.UserGrowthSnapshot, error) {
	if limit <= 0 || limit > growthHistoryLimit {
		limit = growthHistoryLimit
	}
	var rows []models.UserGrowthSnapshot
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
