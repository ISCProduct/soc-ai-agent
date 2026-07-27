package services

import (
	"Backend/internal/models"
	"strings"
)

func (s *ChatService) tryGetPredefinedQuestion(userID uint, sessionID string, prioritizeCategory string, industryID, jobCategoryID uint, targetLevel string, askedTexts map[string]bool, currentPhase string) (*models.PredefinedQuestion, error) {
	if jobCategoryID == 0 {
		// 職種未決定の場合はAI質問に任せる
		return nil, nil
	}
	if strings.TrimSpace(targetLevel) == "" {
		targetLevel = "新卒"
	}

	// すべての事前定義質問を取得して、質問文でフィルタ
	allQuestions, err := s.predefinedQuestionRepo.FindActiveQuestions(targetLevel, &industryID, &jobCategoryID, currentPhase)
	if err != nil {
		return nil, err
	}

	// 職種に合う質問のみ残す（汎用質問はAIに任せる）
	jobSpecificQuestions := make([]*models.PredefinedQuestion, 0, len(allQuestions))
	for _, q := range allQuestions {
		if q.JobCategoryID == nil || *q.JobCategoryID != jobCategoryID {
			continue
		}
		jobSpecificQuestions = append(jobSpecificQuestions, q)
	}

	if len(jobSpecificQuestions) == 0 {
		return nil, nil
	}

	// 優先カテゴリで質問を検索（該当がなければAIに任せる）
	var selected *models.PredefinedQuestion
	for _, q := range jobSpecificQuestions {
		if _, asked := askedTexts[q.QuestionText]; asked {
			continue
		}
		if prioritizeCategory != "" && q.Category != prioritizeCategory {
			continue
		}
		if selected == nil || q.Priority > selected.Priority || (q.Priority == selected.Priority && q.ID < selected.ID) {
			selected = q
		}
	}

	if selected == nil {
		return nil, nil
	}

	return selected, nil
}

func (s *ChatService) isJobSelectionQuestion(text string) bool {
	return isJobSelectionQuestionText(text)
}

func (s *ChatService) shouldValidateJobCategory(history []models.ChatMessage) bool {
	lastAssistant := s.getLastAssistantMessage(history)
	if strings.TrimSpace(lastAssistant) == "" {
		return true
	}
	return s.isJobSelectionQuestion(lastAssistant)
}
