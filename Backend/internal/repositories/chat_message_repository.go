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

// FindBySessionIDForUser は指定ユーザーのメッセージだけを返す（#1156）。
//
// FindBySessionID は session_id だけで引くため、他ユーザーのセッションIDを
// 渡されたときに他人のメッセージが返る。防御を呼び出し側の比較に委ねず、
// クエリ自体をスコープする（多層防御）。
func (r *ChatMessageRepository) FindBySessionIDForUser(sessionID string, userID uint) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := r.db.Where("session_id = ? AND user_id = ?", sessionID, userID).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}

// ExistsBySessionID は session_id のメッセージが1件でも存在するかを返す（#1156）。
//
// FindBySessionIDForUser が0件のとき「新規セッション」と「他人のセッション」を
// 区別するために使う。前者は開始を許可し、後者は拒否する。
func (r *ChatMessageRepository) ExistsBySessionID(sessionID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.ChatMessage{}).
		Where("session_id = ?", sessionID).
		Limit(1).
		Count(&count).Error
	return count > 0, err
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

// FindRecentBySessionIDForUser は指定ユーザーのメッセージだけを最新N件返す（#1156）。
// LLM のコンテキストへ投入する履歴なので、他人のメッセージが混ざらないようクエリをスコープする。
func (r *ChatMessageRepository) FindRecentBySessionIDForUser(
	sessionID string, userID uint, limit int,
) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := r.db.Where("session_id = ? AND user_id = ?", sessionID, userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error

	// 時系列順に並び替え
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, err
}

// GetUsedQuestionIDs セッションで既に使用した質問IDを取得。
// 本番の呼び出し元は現在無い（#1156 の調査で確認）。使うときは user_id でスコープすること。
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
