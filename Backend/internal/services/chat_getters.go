package services

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"context"
	"errors"
	"log"
	"strings"
	"time"
)

// GetChatHistory チャット履歴を取得
func (s *ChatService) GetChatHistory(sessionID string) ([]models.ChatMessage, error) {
	return s.chatMessageRepo.FindBySessionID(sessionID)
}

// GetUserScores ユーザーのスコアを取得
func (s *ChatService) GetUserScores(userID uint, sessionID string) ([]entity.UserWeightScore, error) {
	return s.userWeightScoreRepo.FindByUserAndSession(userID, sessionID)
}

// GetTopRecommendations トップNの適性カテゴリを取得
func (s *ChatService) GetTopRecommendations(userID uint, sessionID string, limit int) ([]entity.UserWeightScore, error) {
	return s.userWeightScoreRepo.FindTopCategories(userID, sessionID, limit)
}

// GetUserChatSessions ユーザーのチャットセッション一覧を取得
func (s *ChatService) GetUserChatSessions(userID uint) ([]models.ChatSession, error) {
	return s.chatMessageRepo.GetUserSessions(userID)
}

// aiCallWithRetries AI呼び出しをリトライして安定化させる（最大3回）
func (s *ChatService) aiCallWithRetries(ctx context.Context, prompt string) (string, error) {
	var resp string
	var err error
	backoffs := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
	for i := 0; i < len(backoffs); i++ {
		resp, err = s.aiClient.Responses(ctx, prompt)
		if err == nil && strings.TrimSpace(resp) != "" {
			return resp, nil
		}
		if err == nil {
			err = errors.New("empty response")
		}
		// log and wait before retry
		log.Printf("Warning: AI call failed or empty response (attempt %d): %v\n", i+1, err)
		if i == len(backoffs)-1 {
			break
		}
		select {
		case <-time.After(backoffs[i]):
			// continue
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	// last attempt with final call (no extra wait)
	resp, err = s.aiClient.Responses(ctx, prompt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resp) == "" {
		return "", errors.New("empty response")
	}
	return resp, nil
}
