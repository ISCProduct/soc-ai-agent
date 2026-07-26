package services

import (
	"Backend/internal/models"
	"Backend/internal/services/prompts"
	"context"
	"fmt"
	"log"
	"strings"
)

func (s *ChatService) generateQuestionWithAI(ctx context.Context, history []models.ChatMessage, userID uint, sessionID string, industryID, jobCategoryID uint) (string, uint, error) {
	// 会話履歴を構築
	historyText := ""
	hasLowConfidenceAnswer := false
	lastQuestion := ""

	for i, msg := range history {
		historyText += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)

		if msg.Role == "assistant" {
			lastQuestion = msg.Content
		}

		// 最後のユーザー回答が「わからない」系かチェック
		if i == len(history)-1 && msg.Role == "user" {
			lowConfidencePatterns := []string{
				"わからない", "わからない", "わかりません", "分かりません",
				"よくわからない", "特にない", "思いつかない", "ありません",
			}
			for _, pattern := range lowConfidencePatterns {
				if strings.Contains(strings.ToLower(msg.Content), pattern) {
					hasLowConfidenceAnswer = true
					break
				}
			}
		}
	}

	// 現在のスコアを取得して、まだ評価が不十分な領域を特定
	scores, err := s.userWeightScoreRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		log.Printf("Warning: failed to get scores for question generation: %v\n", err)
	}

	// スコア分布を分析
	scoreMap := make(map[string]int)
	for _, score := range scores {
		scoreMap[score.WeightCategory] = score.Score
	}

	// まだ評価されていないカテゴリを特定（職種に応じて並び順を調整）
	allCategories := s.getCategoryOrder(jobCategoryID)

	unevaluatedCategories := []string{}
	for _, cat := range allCategories {
		if _, exists := scoreMap[cat]; !exists {
			unevaluatedCategories = append(unevaluatedCategories, cat)
		}
	}

	var prompt string
	if hasLowConfidenceAnswer {
		// わからない回答の場合は、同じカテゴリで別の角度から質問
		prompt = prompts.BuildLowConfidenceQuestionPrompt(historyText, lastQuestion, industryID, jobCategoryID)
	} else if len(unevaluatedCategories) > 0 {
		// 未評価のカテゴリがある場合は、それを重点的に評価
		targetCategory := unevaluatedCategories[0]

		categoryDescriptions := map[string]string{
			"技術志向":       "技術やデジタル活用への興味（授業、趣味、独学）",
			"コミュニケーション力": "人と話すこと、説明すること、協力すること",
			"リーダーシップ志向":  "自分から提案、まとめ役、メンバーのサポート",
			"チームワーク志向":   "グループでの協力、役割分担、助け合い",
			"創造性志向":      "アイデア発想、工夫、新しいアプローチ",
			"安定志向":       "長期的キャリア観、安定性への考え方",
			"成長志向":       "学習意欲、自己成長、新しい挑戦",
			"チャレンジ志向":    "困難への挑戦、失敗を恐れない姿勢",
			"細部志向":       "丁寧さ、正確性、品質へのこだわり",
			"ワークライフバランス": "仕事と私生活のバランス観",
		}
		description := categoryDescriptions[targetCategory]
		prompt = prompts.BuildUnevaluatedCategoryQuestionPrompt(historyText, targetCategory, description, industryID, jobCategoryID)
	} else {
		// 全カテゴリ評価済みの場合は、深掘り質問
		var highestCategory string
		highestScore := -100
		for cat, score := range scoreMap {
			if score > highestScore {
				highestScore = score
				highestCategory = cat
			}
		}
		prompt = prompts.BuildDeepeningQuestionPrompt(historyText, highestCategory, highestScore, industryID, jobCategoryID)
	}

	questionText, err := s.aiCallWithRetries(ctx, prompt)
	if err != nil {
		return "", 0, err
	}

	// 質問文をクリーンアップ
	questionText = strings.TrimSpace(questionText)
	questionText = strings.Trim(questionText, `"「」`)

	// AI生成質問をデータベースに保存
	aiGenQuestion := &models.AIGeneratedQuestion{
		UserID:       userID,
		SessionID:    sessionID,
		TemplateID:   nil, // AI生成の場合はNULL
		QuestionText: questionText,
		Weight:       5, // デフォルト重み
		IsAnswered:   false,
	}

	if err := s.aiGeneratedQuestionRepo.Create(aiGenQuestion); err != nil {
		return "", 0, fmt.Errorf("failed to save AI generated question: %w", err)
	}

	return questionText, aiGenQuestion.ID, nil
}

func (s *ChatService) simplifyQuestionWithAI(ctx context.Context, question string) (string, error) {
	prompt := prompts.BuildSimplifyQuestionPrompt(question)
	return s.aiCallWithRetries(ctx, prompt)
}
