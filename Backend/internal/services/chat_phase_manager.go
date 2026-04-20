package services

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"context"
	"fmt"
	"time"
)

// getCurrentOrNextPhase 現在のフェーズを取得または次のフェーズを開始
func (s *ChatService) getCurrentOrNextPhase(ctx context.Context, userID uint, sessionID string) (*entity.UserAnalysisProgress, error) {
	allPhases, err := s.phaseRepo.FindAll()
	if err != nil {
		return nil, err
	}

	progresses, err := s.progressRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return nil, err
	}
	progressMap := make(map[uint]*entity.UserAnalysisProgress, len(progresses))
	for i := range progresses {
		progressMap[progresses[i].PhaseID] = &progresses[i]
	}

	// 次の未完了フェーズを見つける
	for _, phase := range allPhases {
		if progress, exists := progressMap[phase.ID]; exists {
			if progress.Phase == nil {
				phaseCopy := phase
				progress.Phase = &phaseCopy
			}
			if isPhaseComplete(progress.ValidAnswers, progress.Phase) {
				continue
			}
			return progress, nil
		}
		return s.progressRepo.FindOrCreate(userID, sessionID, phase.ID)
	}

	// 全フェーズ完了
	return nil, fmt.Errorf("all phases completed")
}

// updatePhaseProgress フェーズの進捗を更新
func (s *ChatService) updatePhaseProgress(progress *entity.UserAnalysisProgress, isValidAnswer bool) error {
	progress.QuestionsAsked++
	if isValidAnswer {
		progress.ValidAnswers++
	} else {
		progress.InvalidAnswers++
	}

	progress.CompletionScore = phaseCompletionScore(progress.ValidAnswers, progress.Phase)
	newIsCompleted := isPhaseComplete(progress.ValidAnswers, progress.Phase)
	if newIsCompleted {
		if !progress.IsCompleted {
			now := time.Now()
			progress.CompletedAt = &now
			phaseLabel := "分析"
			if progress.Phase != nil {
				if progress.Phase.DisplayName != "" {
					phaseLabel = progress.Phase.DisplayName
				} else if progress.Phase.PhaseName != "" {
					phaseLabel = progress.Phase.PhaseName
				}
			}
			fmt.Printf("%sが完了しました。\n", phaseLabel)
		}
	} else {
		progress.CompletedAt = nil
	}
	progress.IsCompleted = newIsCompleted

	return s.progressRepo.Update(progress)
}

// buildPhaseProgressResponse フェーズ進捗レスポンスを構築
func (s *ChatService) buildPhaseProgressResponse(userID uint, sessionID string) ([]PhaseProgress, *PhaseProgress, error) {
	progresses, err := s.progressRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	allPhases, err := s.phaseRepo.FindAll()
	if err != nil {
		return nil, nil, err
	}

	progressMap := make(map[uint]*entity.UserAnalysisProgress)
	for i := range progresses {
		progressMap[progresses[i].PhaseID] = &progresses[i]
	}

	var result []PhaseProgress
	var current *PhaseProgress

	for _, phase := range allPhases {
		pp := PhaseProgress{
			PhaseID:      phase.ID,
			PhaseName:    phase.PhaseName,
			DisplayName:  phase.DisplayName,
			MinQuestions: phase.MinQuestions,
			MaxQuestions: phase.MaxQuestions,
		}

		if progress, exists := progressMap[phase.ID]; exists {
			completionScore := phaseCompletionScore(progress.ValidAnswers, &phase)
			pp.QuestionsAsked = progress.QuestionsAsked
			pp.ValidAnswers = progress.ValidAnswers
			pp.CompletionScore = completionScore
			pp.IsCompleted = isPhaseComplete(progress.ValidAnswers, &phase)

			if !pp.IsCompleted && current == nil {
				current = &pp
			}
		}

		result = append(result, pp)
	}

	return result, current, nil
}

func phaseCompletionScore(validAnswers int, phase *entity.AnalysisPhase) float64 {
	if phase == nil {
		return 0
	}
	required := phase.MaxQuestions
	if required <= 0 {
		required = phase.MinQuestions
	}
	if required <= 0 {
		return 0
	}
	score := (float64(validAnswers) / float64(required)) * 100
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}

func isPhaseComplete(validAnswers int, phase *entity.AnalysisPhase) bool {
	if phase == nil {
		return false
	}
	required := phase.MaxQuestions
	if required <= 0 {
		required = phase.MinQuestions
	}
	if required <= 0 {
		return false
	}
	return validAnswers >= required
}

func (s *ChatService) getLastPhaseProgress(userID uint, sessionID string) (*entity.UserAnalysisProgress, error) {
	progresses, err := s.progressRepo.FindByUserAndSession(userID, sessionID)
	if err != nil {
		return nil, err
	}
	if len(progresses) == 0 {
		return nil, fmt.Errorf("no phase progress found")
	}
	return &progresses[len(progresses)-1], nil
}

func allPhasesReachedMax(progresses []entity.UserAnalysisProgress, phases []entity.AnalysisPhase) bool {
	if len(phases) == 0 {
		return false
	}
	progressMap := make(map[uint]entity.UserAnalysisProgress, len(progresses))
	for _, p := range progresses {
		progressMap[p.PhaseID] = p
	}
	for _, phase := range phases {
		p, ok := progressMap[phase.ID]
		if !ok {
			return false
		}
		if phase.MaxQuestions > 0 && p.QuestionsAsked < phase.MaxQuestions {
			return false
		}
	}
	return true
}

func shouldForceTextQuestion(history []models.ChatMessage, currentPhase *entity.UserAnalysisProgress) bool {
	if currentPhase == nil || currentPhase.Phase == nil {
		return false
	}
	minText := minTextQuestionsForPhase(currentPhase.Phase.PhaseName)
	if minText == 0 {
		return false
	}
	if currentPhase.QuestionsAsked == 0 {
		return true
	}
	textCount := countTextQuestionsInPhase(history, currentPhase.QuestionsAsked)
	return textCount < minText
}

func minTextQuestionsForPhase(phaseName string) int {
	switch phaseName {
	case "job_analysis", "interest_analysis", "aptitude_analysis", "future_analysis":
		return 1
	default:
		return 0
	}
}

func countTextQuestionsInPhase(history []models.ChatMessage, phaseQuestionsAsked int) int {
	if phaseQuestionsAsked <= 0 {
		return 0
	}
	textCount := 0
	questionCount := 0
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role != "assistant" {
			continue
		}
		questionText := normalizeQuestionText(msg.Content)
		if questionText == "" || !isQuestion(questionText) {
			continue
		}
		questionCount++
		if isTextBasedQuestion(questionText) {
			textCount++
		}
		if questionCount >= phaseQuestionsAsked {
			break
		}
	}
	return textCount
}

func countUserAnswers(history []models.ChatMessage) int {
	count := 0
	for _, msg := range history {
		if msg.Role == "user" {
			count++
		}
	}
	return count
}
