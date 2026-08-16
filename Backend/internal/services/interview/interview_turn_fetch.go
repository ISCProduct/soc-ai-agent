package interview

import (
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// fetchCustomQuestions は企業別カスタム質問を取得する。未登録時は空スライスを返す。
func (s *InterviewService) fetchCustomQuestions(companyID uint, position string) []models.InterviewCompanyQuestion {
	if s.companyQuestionRepo == nil || companyID == 0 {
		return nil
	}
	qs, err := s.companyQuestionRepo.FindByCompanyAndPosition(companyID, position)
	if err != nil {
		log.Printf("[Interview] fetchCustomQuestions error: %v", err)
		return nil
	}
	return qs
}

// fetchSkillScores はユーザーのGitHubスキルスコアを取得する。未登録時は空スライスを返す。
func (s *InterviewService) fetchSkillScores(userID uint) []models.SkillScore {
	if s.skillScoreRepo == nil {
		return nil
	}
	scores, err := s.skillScoreRepo.GetScores(userID)
	if err != nil {
		log.Printf("[Interview] fetchSkillScores error: %v", err)
		return nil
	}
	return scores
}

// lookupCompanyReading はAIモデルの知識から企業名の日本語読み（ふりがな）を取得します。
// 取得に失敗した場合は空文字を返します（エラーは無視）。
func (s *InterviewService) lookupCompanyReading(ctx context.Context, companyName string) string {
	systemPrompt := "あなたは日本企業の名称に詳しいアシスタントです。確実に知っている場合のみ答え、不明なら空文字を返してください。"
	query := fmt.Sprintf("「%s」の正しい日本語読み（ふりがな）をカタカナで1行だけ答えてください。", companyName)
	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := s.openaiClient.ResponsesWithTemperature(ctxTimeout, systemPrompt, query, 0.0, companyfetch.ExtractModel())
	if err != nil {
		return ""
	}
	// 最初の行だけ抽出し、余分な記号や空白を除去
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) == 0 {
		return ""
	}
	reading := strings.TrimSpace(lines[0])
	// 句読点・括弧・引用符等を除去
	reading = strings.Trim(reading, "「」『』（）()。、・ ")
	return reading
}
