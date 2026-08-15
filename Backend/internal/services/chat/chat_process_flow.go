package chat

import (
	"Backend/domain/entity"
	"Backend/internal/models"
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
		TotalQuestions:    15,
		AnsweredQuestions: len(history) / 2,
		AllPhases:         allPhases,
		CurrentPhase:      currentPhaseInfo,
	}

	if validation != nil {
		chatResponse.InvalidAnswerCount = validation.InvalidAnswerCount
		chatResponse.IsTerminated = validation.IsTerminated

		// 3回目の無効回答の場合は完了フラグを立てる
		if validation.IsTerminated {
			chatResponse.IsComplete = true
		}
	}

	return chatResponse, nil
}

func (s *ChatService) handleAllPhasesCompletedFromPhaseError(req ChatRequest, history []models.ChatMessage) (*ChatResponse, error) {
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
	// ここで要約を生成して保存（failure を許容して応答は続行）
	summary, _ := s.buildAndSaveSessionSummary(context.Background(), req.UserID, req.SessionID)
	return &ChatResponse{
		Response:            completionMsg,
		IsComplete:          true,
		TotalQuestions:      15,
		AnsweredQuestions:   countUserAnswers(history),
		EvaluatedCategories: 10,
		TotalCategories:     10,
		AllPhases:           allPhases,
		CurrentPhase:        currentPhaseInfo,
		Summary:             summary,
	}, nil
}

func (s *ChatService) handleAllPhasesCompletedFromCount(
	req ChatRequest,
	history []models.ChatMessage,
	allPhases []entity.AnalysisPhase,
	completedProgresses []entity.UserAnalysisProgress,
	phaseByID map[uint]*entity.AnalysisPhase,
) (*ChatResponse, bool) {
	completedPhaseCount := 0
	for _, p := range completedProgresses {
		phase := p.Phase
		if phase == nil {
			phase = phaseByID[p.PhaseID]
		}
		if isPhaseComplete(p.ValidAnswers, phase) {
			completedPhaseCount++
		}
	}
	if completedPhaseCount != len(allPhases) || len(allPhases) == 0 {
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
	// ここで要約を生成して保存
	summary, _ := s.buildAndSaveSessionSummary(context.Background(), req.UserID, req.SessionID)
	return &ChatResponse{
		Response:            completionMsg,
		IsComplete:          true,
		TotalQuestions:      totalMinQuestions,
		AnsweredQuestions:   countUserAnswers(history),
		EvaluatedCategories: evaluatedCategoriesCount,
		TotalCategories:     10,
		AllPhases:           allPhasesInfo,
		CurrentPhase:        currentPhaseInfo,
		Summary:             summary,
	}, true
}
