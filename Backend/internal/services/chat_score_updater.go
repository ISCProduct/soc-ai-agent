package services

import (
	"Backend/internal/models"
	"context"
	"fmt"
	"log"
	"math"
	"strings"
)

// analyzeAndUpdateWeights ユーザーの回答を分析し重み係数を更新
func (s *ChatService) analyzeAndUpdateWeights(ctx context.Context, userID uint, sessionID, message string, jobCategoryID uint) error {
	// 会話履歴から直近の質問を取得
	history, err := s.chatMessageRepo.FindRecentBySessionID(sessionID, 5)
	if err != nil {
		log.Printf("Warning: failed to get history for analysis: %v\n", err)
		history = []models.ChatMessage{}
	}

	lastQuestion := ""
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			lastQuestion = history[i].Content
			break
		}
	}

	if strings.TrimSpace(lastQuestion) == "" {
		log.Printf("Warning: no previous question found for scoring\n")
		return nil
	}
	if s.isJobSelectionQuestion(lastQuestion) {
		log.Printf("Skipping analysis for job selection question\n")
		return nil
	}

	targetCategory := s.inferCategoryFromQuestion(lastQuestion)
	isChoice := !isTextBasedQuestion(lastQuestion)

	result := s.answerEvaluator.EvaluateHumanScoring(lastQuestion, message, isChoice, jobCategoryID != 0, nil)
	if result.Action != PrecheckScore {
		log.Printf("Skipping scoring due to precheck: %s\n", result.Reason)
		return nil
	}
	if result.Score <= 0 {
		log.Printf("No human score applied (score=%d)\n", result.Score)
		return nil
	}

	return s.updateCategoryScore(userID, sessionID, targetCategory, result.Score)
}

// processChoiceAnswer 選択肢回答を処理してスコアを更新
func (s *ChatService) processChoiceAnswer(ctx context.Context, userID uint, sessionID, answer string, history []models.ChatMessage, jobCategoryID uint) error {
	// 最後のAIの質問を取得
	var lastQuestion string
	var targetCategory string

	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			lastQuestion = history[i].Content
			break
		}
	}

	if lastQuestion == "" {
		return fmt.Errorf("no previous question found")
	}
	if s.isJobSelectionQuestion(lastQuestion) {
		log.Printf("[Choice Answer] Skipping score update for job selection question\n")
		return nil
	}

	// AIが生成した質問から対象カテゴリを特定
	aiQuestions, err := s.aiGeneratedQuestionRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get AI questions: %w", err)
	}

	for i := len(aiQuestions) - 1; i >= 0; i-- {
		if strings.Contains(lastQuestion, aiQuestions[i].QuestionText) ||
			strings.Contains(aiQuestions[i].QuestionText, strings.Split(lastQuestion, "\n")[0]) {
			if aiQuestions[i].Template != nil {
				targetCategory = aiQuestions[i].Template.Category
			}
			break
		}
	}

	if targetCategory == "" {
		// カテゴリが特定できない場合は、質問文から推測
		targetCategory = s.inferCategoryFromQuestion(lastQuestion)
	}

	log.Printf("[Choice Answer] Processing choice '%s' for category: %s\n", answer, targetCategory)

	result := s.answerEvaluator.EvaluateHumanScoring(lastQuestion, answer, true, jobCategoryID != 0, nil)
	if result.Action != PrecheckScore {
		log.Printf("Skipping choice scoring due to precheck: %s\n", result.Reason)
		return nil
	}
	score := result.Score

	// スコアを保存または更新
	return s.updateCategoryScore(userID, sessionID, targetCategory, score)
}

// convertChoiceToScore 選択肢をスコアに変換
func (s *ChatService) convertChoiceToScore(choice string) int {
	choice = strings.ToUpper(strings.TrimSpace(choice))
	switch choice {
	case "A", "1":
		return 100 // 非常に高い/強く同意
	case "B", "2":
		return 75 // やや高い/やや同意
	case "C", "3":
		return 50 // 中立/どちらでもない
	case "D", "4":
		return 25 // やや低い/やや不同意
	case "E", "5":
		return 0 // 低い/不同意
	default:
		return 50 // デフォルト
	}
}

// inferCategoryFromQuestion 質問文からカテゴリを推測
func (s *ChatService) inferCategoryFromQuestion(question string) string {
	categoryKeywords := map[string][]string{
		"技術志向":       {"技術", "プログラミング", "コーディング", "アルゴリズム", "システム設計", "新しい技術", "技術的"},
		"チームワーク":     {"チーム", "協力", "協働", "連携", "メンバー", "共同"},
		"リーダーシップ":    {"リーダー", "指導", "率いる", "マネジメント", "方向性", "意思決定"},
		"創造性":        {"創造", "アイデア", "発想", "革新", "イノベーション", "新しい"},
		"安定志向":       {"安定", "確実", "堅実", "リスク回避", "慎重"},
		"成長志向":       {"成長", "キャリア", "昇進", "スキルアップ", "学習"},
		"ワークライフバランス": {"ワークライフ", "残業", "休日", "プライベート", "働き方"},
		"チャレンジ志向":    {"チャレンジ", "挑戦", "困難", "新しいこと", "未経験"},
		"細部志向":       {"細部", "詳細", "正確", "精密", "丁寧"},
		"コミュニケーション力": {"コミュニケーション", "説明", "伝える", "対話", "話す", "プレゼン"},
	}

	questionLower := strings.ToLower(question)
	for category, keywords := range categoryKeywords {
		for _, keyword := range keywords {
			if strings.Contains(questionLower, strings.ToLower(keyword)) {
				return category
			}
		}
	}

	return "技術志向" // デフォルト
}

// updateCategoryScore カテゴリスコアを更新
func (s *ChatService) updateCategoryScore(userID uint, sessionID, category string, score int) error {
	// 既存のスコアを取得
	existingScore, err := s.userWeightScoreRepo.FindByUserSessionAndCategory(userID, sessionID, category)

	if err != nil || existingScore == nil {
		// 新規作成: 絶対値でセット
		if err := s.userWeightScoreRepo.SetScore(userID, sessionID, category, score); err != nil {
			return fmt.Errorf("failed to create score: %w", err)
		}
		log.Printf("[Choice Answer] Created new score: %s = %d\n", category, score)
	} else {
		// 移動平均で更新（直近回答の影響を反映）: 差分を加算
		newScore := int(math.Round(float64(existingScore.Score)*0.7 + float64(score)*0.3))
		delta := newScore - existingScore.Score
		if delta == 0 {
			log.Printf("[Choice Answer] Score unchanged: %s = %d\n", category, existingScore.Score)
			return nil
		}
		if err := s.userWeightScoreRepo.AddScore(userID, sessionID, category, delta); err != nil {
			return fmt.Errorf("failed to update score: %w", err)
		}
		log.Printf("[Choice Answer] Updated score: %s = %d (average)\n", category, newScore)
	}

	return nil
}

// isChoiceAnswer 選択肢回答かどうかを判定
func (s *ChatService) isChoiceAnswer(answer string) bool {
	answer = strings.ToUpper(strings.TrimSpace(answer))
	// A-E または 1-5 の形式
	return answer == "A" || answer == "B" || answer == "C" || answer == "D" || answer == "E" ||
		answer == "1" || answer == "2" || answer == "3" || answer == "4" || answer == "5"
}

func buildChoiceFallback(questionText, phaseName string) string {
	choices := []string{}
	switch phaseName {
	case "job_analysis":
		choices = []string{
			"1) ものづくり・開発系（Web/アプリ/設計）",
			"2) データ・分析系（分析/企画/改善）",
			"3) インフラ・運用系（基盤/安定稼働）",
			"4) 対人・調整系（営業/人事/サポート）",
			"5) その他（自由記述）",
		}
	case "interest_analysis":
		choices = []string{
			"1) 新しい技術やツールに触れる",
			"2) 仕組みを考えたり設計する",
			"3) 人と関わりながら進める",
			"4) コツコツ改善・整理する",
			"5) その他（自由記述）",
		}
	case "aptitude_analysis":
		choices = []string{
			"1) 自分から主導して進める",
			"2) みんなで協力して進める",
			"3) 支える・サポート役に回る",
			"4) 一人で集中して進める",
			"5) その他（自由記述）",
		}
	case "future_analysis":
		choices = []string{
			"1) 安定や福利厚生を重視",
			"2) 成長や挑戦を重視",
			"3) ワークライフバランス重視",
			"4) 裁量や自由度重視",
			"5) その他（自由記述）",
		}
	default:
		choices = []string{
			"1) とても当てはまる",
			"2) まあ当てはまる",
			"3) あまり当てはまらない",
			"4) まったく当てはまらない",
			"5) その他（自由記述）",
		}
	}
	return fmt.Sprintf("%s\n\n%s", strings.TrimSpace(questionText), strings.Join(choices, "\n"))
}
