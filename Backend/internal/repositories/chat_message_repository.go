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

// FindBySessionIDForUser は指定ユーザーのメッセージだけを返す（#1156）。
//
// session_id だけで引くと、他ユーザーのセッションIDを渡されたときに
// 他人のメッセージが返る。防御を呼び出し側の比較に委ねず、
// クエリ自体をスコープする（多層防御）。
func (r *ChatMessageRepository) FindBySessionIDForUser(sessionID string, userID uint) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := r.db.Where("session_id = ? AND user_id = ?", sessionID, userID).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}

// ExistsBySessionIDForOtherUser は session_id に「自分以外」のメッセージが
// あるかを返す（#1156）。このリポジトリで意図的に user_id でスコープしない唯一のメソッド。
//
// session_id はクライアント採番のため衝突・推測があり得る。他人のメッセージが
// 混在したセッションは双方に対して拒否する（フェイルクローズ）。
// 「自分のメッセージが1件でもあれば許可」にすると、混在セッションで両者が通り、
// 下流の要約・埋め込み・分析に相手の自由記述が混ざる。
func (r *ChatMessageRepository) ExistsBySessionIDForOtherUser(sessionID string, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.ChatMessage{}).
		Where("session_id = ? AND user_id <> ?", sessionID, userID).
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
