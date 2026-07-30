package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// buildAndSaveSessionSummary チャットセッションの要約（強み・注意点・おすすめの働き方）を生成し、DBに保存します。
func (s *ChatService) buildAndSaveSessionSummary(ctx context.Context, userID uint, sessionID string) (*SessionSummary, error) {
	if s.aiClient == nil || s.chatMessageRepo == nil || s.userWeightScoreRepo == nil || s.conversationContextRepo == nil {
		return nil, fmt.Errorf("dependencies missing for summary generation")
	}

	// 直近のユーザーメッセージを収集
	msgs, err := s.chatMessageRepo.FindRecentBySessionID(sessionID, 30)
	if err != nil {
		return nil, err
	}
	var lastUserTexts []string
	for _, m := range msgs {
		if m.Role == "user" && strings.TrimSpace(m.Content) != "" {
			lastUserTexts = append(lastUserTexts, m.Content)
		}
	}
	userContext := strings.Join(lastUserTexts, "\n---\n")

	// スコア要約
	scores, _ := s.userWeightScoreRepo.FindByUserAndSession(userID, sessionID)
	// 簡易的に上位3カテゴリを列挙
	topCats := ""
	if len(scores) > 0 {
		for i, sc := range scores {
			if i >= 3 {
				break
			}
			if i > 0 {
				topCats += "\n"
			}
			topCats += fmt.Sprintf("- %s: %d", sc.WeightCategory, sc.Score)
		}
	}

	contextBytes, _ := json.Marshal(map[string]any{
		"top_scores": topCats,
	})

	systemPrompt := "あなたは就職適性診断の専門家です。以下の情報をもとに、ユーザー向けに短く親しみやすい日本語で要約を生成してください。出力は必ずJSONのみを返してください。フォーマット: {\"strengths\": \"...\", \"weaknesses\": \"...\", \"recommended_working_style\": \"...\"}。各項目は2〜3文、合計で200文字程度を目安にしてください。"
	userPrompt := "解析情報: " + string(contextBytes) + "\n\n直近のユーザーメッセージ:\n" + userContext

	raw, err := s.aiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.2, 400)
	if err != nil {
		log.Printf("LLM session summary generation failed: %v", err)
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("empty summary from LLM")
	}

	var parsed SessionSummary
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// JSONパースに失敗した場合は生テキストとして保存して終了
		_ = s.conversationContextRepo.SetSessionSummary(userID, sessionID, raw)
		return nil, fmt.Errorf("failed to parse summary json: %w", err)
	}

	// 永続化（生JSONを保存）
	if err := s.conversationContextRepo.SetSessionSummary(userID, sessionID, raw); err != nil {
		log.Printf("failed to persist session summary: %v", err)
	}

	return &parsed, nil
}
