package repositories

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/internal/models"

	"gorm.io/gorm"
)

type UserWeightScoreRepository struct {
	db *gorm.DB
}

func NewUserWeightScoreRepository(db *gorm.DB) *UserWeightScoreRepository {
	return &UserWeightScoreRepository{db: db}
}

// SetScore スコアを絶対値で新規作成する。
// 呼び出し前に対象レコードが存在しないことを確認すること。
func (r *UserWeightScoreRepository) SetScore(userID uint, sessionID, category string, absoluteScore int) error {
	score := models.UserWeightScore{
		UserID:         userID,
		SessionID:      sessionID,
		WeightCategory: category,
		Score:          absoluteScore,
	}
	return r.db.Create(&score).Error
}

// AddScore 既存スコアに差分を加算する。
// レコードが存在しない場合はエラーを返す。
func (r *UserWeightScoreRepository) AddScore(userID uint, sessionID, category string, delta int) error {
	var score models.UserWeightScore
	err := r.db.Where("user_id = ? AND session_id = ? AND weight_category = ?",
		userID, sessionID, category).First(&score).Error
	if err != nil {
		return err
	}
	return r.db.Model(&score).Update("score", gorm.Expr("score + ?", delta)).Error
}

// FindByUserAndSession ユーザーとセッションに紐づく全スコアを取得
func (r *UserWeightScoreRepository) FindByUserAndSession(userID uint, sessionID string) ([]entity.UserWeightScore, error) {
	var ms []models.UserWeightScore
	err := r.db.Where("user_id = ? AND session_id = ?", userID, sessionID).
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return mapper.UserWeightScoresToEntities(ms), nil
}

// FindTopCategories トップNのカテゴリを取得
func (r *UserWeightScoreRepository) FindTopCategories(userID uint, sessionID string, limit int) ([]entity.UserWeightScore, error) {
	var ms []models.UserWeightScore
	err := r.db.Where("user_id = ? AND session_id = ?", userID, sessionID).
		Order("score DESC").
		Limit(limit).
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return mapper.UserWeightScoresToEntities(ms), nil
}

// FindByUserSessionAndCategory ユーザー、セッション、カテゴリで検索
func (r *UserWeightScoreRepository) FindByUserSessionAndCategory(userID uint, sessionID, category string) (*entity.UserWeightScore, error) {
	var m models.UserWeightScore
	err := r.db.Where("user_id = ? AND session_id = ? AND weight_category = ?", userID, sessionID, category).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return mapper.UserWeightScoreToEntity(&m), nil
}

// CountByUserAndSession ユーザーとセッションに紐づくスコア数を取得
func (r *UserWeightScoreRepository) CountByUserAndSession(userID uint, sessionID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.UserWeightScore{}).
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Count(&count).Error
	return count, err
}

// FindLatestByUser ユーザーの最新セッションのスコアを取得する
func (r *UserWeightScoreRepository) FindLatestByUser(userID uint) ([]entity.UserWeightScore, error) {
	// 最新の session_id を特定
	var latest models.UserWeightScore
	err := r.db.Where("user_id = ?", userID).
		Order("updated_at DESC").
		First(&latest).Error
	if err != nil {
		return nil, err
	}
	return r.FindByUserAndSession(userID, latest.SessionID)
}
