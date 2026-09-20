package chat

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

	// 職種固有の質問を優先しつつ、汎用質問(job_category_id IS NULL)も使う。
	//
	// 以前はここで汎用質問を捨てていた（「汎用はAIに任せる」という方針）。
	// しかし事前定義質問11問のうち職種に紐づくのは技術志向の2問だけで、
	// 残り9問（安定志向・ワークライフバランスを含む）が丸ごと使われず、
	// 採点ルール付きの質問があるのに毎回AI生成へ落ちていた(#1333)。
	//
	// 汎用でも「その軸を測る」目的は果たせる。職種固有があればそちらを選ぶ、
	// という優先順位だけ残して捨てるのはやめる。
	betterThan := func(a, b *models.PredefinedQuestion) bool {
		if b == nil {
			return true
		}
		aSpecific := a.JobCategoryID != nil && *a.JobCategoryID == jobCategoryID
		bSpecific := b.JobCategoryID != nil && *b.JobCategoryID == jobCategoryID
		if aSpecific != bSpecific {
			return aSpecific // 職種固有が勝つ
		}
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		return a.ID < b.ID
	}

	var selected *models.PredefinedQuestion
	for _, q := range allQuestions {
		// 他職種向けの質問は対象外。汎用(nil)は残す。
		if q.JobCategoryID != nil && *q.JobCategoryID != jobCategoryID {
			continue
		}
		if _, asked := askedTexts[q.QuestionText]; asked {
			continue
		}
		if prioritizeCategory != "" && q.Category != prioritizeCategory {
			continue
		}
		if betterThan(q, selected) {
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
	// 警告メッセージを飛ばし、実質の直前質問で判定する
	lastAssistant := findLastAssistantQuestion(history)
	if strings.TrimSpace(lastAssistant) == "" {
		// 質問がまだ無い初回のみ職種判定へ
		if s.getLastAssistantMessage(history) == "" {
			return true
		}
		return false
	}
	return s.isJobSelectionQuestion(lastAssistant)
}
