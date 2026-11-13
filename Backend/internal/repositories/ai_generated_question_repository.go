package repositories

import (
	"Backend/internal/models"
	"gorm.io/gorm"
)

type AIGeneratedQuestionRepository struct {
	db *gorm.DB
}

func NewAIGeneratedQuestionRepository(db *gorm.DB) *AIGeneratedQuestionRepository {
	return &AIGeneratedQuestionRepository{db: db}
}

// Create AI生成質問を保存
func (r *AIGeneratedQuestionRepository) Create(q *models.AIGeneratedQuestion) error {
	return r.db.Create(q).Error
}

// FindBySessionID セッションIDで質問を取得
func (r *AIGeneratedQuestionRepository) FindBySessionID(sessionID string) ([]models.AIGeneratedQuestion, error) {
	var questions []models.AIGeneratedQuestion
	err := r.db.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&questions).Error
	return questions, err
}

// FindByUserAndSession ユーザーとセッションで質問を取得
func (r *AIGeneratedQuestionRepository) FindByUserAndSession(userID uint, sessionID string) ([]models.AIGeneratedQuestion, error) {
	var questions []models.AIGeneratedQuestion
	err := r.db.Where("user_id = ? AND session_id = ?", userID, sessionID).
		Order("created_at ASC").
		Find(&questions).Error
	return questions, err
}

// UpdateAnswer 回答を更新
func (r *AIGeneratedQuestionRepository) UpdateAnswer(id uint, answerText string, answerScore int) error {
	return r.db.Model(&models.AIGeneratedQuestion{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"answer_text":  answerText,
			"answer_score": answerScore,
			"is_answered":  true,
		}).Error
}
