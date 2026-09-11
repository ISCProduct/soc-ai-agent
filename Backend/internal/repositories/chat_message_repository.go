package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

type ChatMessageRepository struct {
	db *gorm.DB
}

func NewChatMessageRepository(db *gorm.DB) *ChatMessageRepository {
	return &ChatMessageRepository{db: db}
}

// Create チャットメッセージを保存
func (r *ChatMessageRepository) Create(msg *models.ChatMessage) error {
	if err := fillOrganizationID(r.db, msg.UserID, &msg.OrganizationID); err != nil {
		return err
	}
	return r.db.Create(msg).Error
}

// FindBySessionID セッションIDでメッセージ履歴を取得
func (r *ChatMessageRepository) FindBySessionID(sessionID string) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := r.db.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}

// FindByUserID ユーザーIDで全てのチャット履歴を取得
func (r *ChatMessageRepository) FindByUserID(userID uint) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := r.db.Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}

// FindRecentBySessionID セッションIDで最新N件を取得
func (r *ChatMessageRepository) FindRecentBySessionID(sessionID string, limit int) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := r.db.Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error

	// 時系列順に並び替え
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, err
}

// GetUsedQuestionIDs セッションで既に使用した質問IDを取得
func (r *ChatMessageRepository) GetUsedQuestionIDs(sessionID string) ([]uint, error) {
	var questionIDs []uint
	err := r.db.Model(&models.ChatMessage{}).
		Where("session_id = ? AND role = ? AND question_weight_id > 0", sessionID, "assistant").
		Pluck("question_weight_id", &questionIDs).Error
	return questionIDs, err
}

// GetUserSessions ユーザーのチャットセッション一覧を取得
func (r *ChatMessageRepository) GetUserSessions(userID uint) ([]models.ChatSession, error) {
	var sessions []models.ChatSession
	err := r.db.Raw(`
		SELECT 
			session_id,
			user_id,
			MIN(created_at) as started_at,
			MAX(created_at) as last_message_at,
			COUNT(*) as message_count
		FROM chat_messages
		WHERE user_id = ?
		GROUP BY session_id, user_id
		ORDER BY last_message_at DESC
	`, userID).Scan(&sessions).Error
	return sessions, err
}
