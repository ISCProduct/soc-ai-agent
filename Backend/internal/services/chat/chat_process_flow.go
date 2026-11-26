package chat

import (
	"Backend/internal/models"
	"Backend/internal/services/judgment"
	"context"
	"log"
)

func (s *ChatService) handleInvalidAnswerFlow(ctx context.Context, req ChatRequest, history []models.ChatMessage, response string) (*ChatResponse, error) {
	currentPhase, phaseErr := s.getCurrentOrNextPhase(ctx, req.UserID, req.SessionID)
	if phaseErr == nil {
		if err := s.updatePhaseProgress(currentPhase, false); err != nil {
			log.Printf("Warning: failed to update phase progress for invalid answer: %v\n", err)
		}
	}

	validation, err := s.sessionValidationRepo.GetOrCreate(req.SessionID)
	if err != nil {
		log.Printf("Warning: failed to get validation: %v\n", err)
	}

	allPhases, currentPhaseInfo, _ := s.buildPhaseProgressResponse(req.UserID, req.SessionID)

	chatResponse := &ChatResponse{
		Response:          response,
		IsComplete:        false,
		TotalQuestions:    defaultTargetQuestions,
		AnsweredQuestions: len(history) / 2,
		AllPhases:         allPhases,
		CurrentPhase:      currentPhaseInfo,
	}
	j := s.judgmentFor(req.UserID, req.SessionID)
	chatResponse.JudgmentPercent = j.Percent
	chatResponse.SuggestProvisional = judgment.SuggestProvisional(countUserAnswers(history), j.Ready)

	if validation != nil {
		chatResponse.InvalidAnswerCount = validation.InvalidAnswerCount
		chatResponse.IsTerminated = validation.IsTerminated

		// 3回目の無効回答の場合は完了フラグを立て、やり直し/スキップを提案する（#522）
		if validation.IsTerminated {
			chatResponse.IsComplete = true
			chatResponse.SuggestRestart = true
		}
	}

	return chatResponse, nil
}

func (s *ChatService) handleAllPhasesCompletedFromPhaseError(ctx context.Context, req ChatRequest, history []models.ChatMessage) (*ChatResponse, error) {
	completionMsg := "分析が完了しました！あなたに最適な企業をマッチングしました。「結果を見る」ボタンから詳細をご確認ください。"

	assistantMsg := &models.ChatMessage{
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Role:      "assistant",
		Content:   completionMsg,
	}
	if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
		log.Printf("Warning: failed to save completion message: %v\n", err)
	}

	allPhases, currentPhaseInfo, _ := s.buildPhaseProgressResponse(req.UserID, req.SessionID)
	summary := s.sessionSummaryForResponse(ctx, req.UserID, req.SessionID)
	return &ChatResponse{
		Response:            completionMsg,
		IsComplete:          true,
		TotalQuestions:      defaultTargetQuestions,
		AnsweredQuestions:   countUserAnswers(history),
		EvaluatedCategories: 10,
		TotalCategories:     10,
		JudgmentPercent:     100,
		AllPhases:           allPhases,
		CurrentPhase:        currentPhaseInfo,
		Summary:             summary,
	}, nil
}

func (s *ChatService) handleAllPhasesCompletedFromCount(
	ctx context.Context,
	req ChatRequest,
	history []models.ChatMessage,
	allPhases []models.AnalysisPhase,
) (*ChatResponse, bool) {
	// 完了は、フェーズを全部終えたかではなく、判定できたかで決める（SOCAIAGENT-668）。
	// 15 問に満たなくても判定できていれば完了、全フェーズを終えても判定できなければ続ける。
	if len(allPhases) == 0 || !s.judgmentFor(req.UserID, req.SessionID).Ready {
		return nil, false
	}

	completionMsg := "分析が完了しました！あなたに最適な企業をマッチングしました。「結果を見る」ボタンから詳細をご確認ください。"
	assistantMsg := &models.ChatMessage{
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Role:      "assistant",
		Content:   completionMsg,
	}
	if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
		log.Printf("Warning: failed to save completion message: %v\n", err)
	}
	allPhasesInfo, currentPhaseInfo, _ := s.buildPhaseProgressResponse(req.UserID, req.SessionID)
	evaluatedCategoriesCount := 0
	scores, err := s.userWeightScoreRepo.FindByUserAndSession(req.UserID, req.SessionID)
	if err != nil {
		log.Printf("Warning: failed to get scores for completion response: %v\n", err)
	} else {
		seenCategories := make(map[string]bool)
		for _, score := range scores {
			if score.Score != 0 {
				seenCategories[score.WeightCategory] = true
			}
		}
		evaluatedCategoriesCount = len(seenCategories)
	}
	totalMinQuestions := 0
	for _, phase := range allPhases {
		if phase.MaxQuestions > 0 {
			totalMinQuestions += phase.MaxQuestions
		} else {
			totalMinQuestions += phase.MinQuestions
		}
	}
	summary := s.sessionSummaryForResponse(ctx, req.UserID, req.SessionID)
	return &ChatResponse{
		Response:            completionMsg,
		IsComplete:          true,
		TotalQuestions:      totalMinQuestions,
		AnsweredQuestions:   countUserAnswers(history),
		EvaluatedCategories: evaluatedCategoriesCount,
		TotalCategories:     10,
		JudgmentPercent:     100,
		AllPhases:           allPhasesInfo,
		CurrentPhase:        currentPhaseInfo,
		Summary:             summary,
	}, true
}
