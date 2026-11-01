package mapper

import (
	"Backend/domain/entity"
	"Backend/internal/models"
)

// UserWeightScoreToEntity models.UserWeightScore を entity.UserWeightScore に変換
func UserWeightScoreToEntity(m *models.UserWeightScore) *entity.UserWeightScore {
	if m == nil {
		return nil
	}
	return &entity.UserWeightScore{
		ID:             m.ID,
		UserID:         m.UserID,
		SessionID:      m.SessionID,
		WeightCategory: m.WeightCategory,
		Score:          m.Score,
		PromptVersion:  m.PromptVersion,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

// UserWeightScoresToEntities []models.UserWeightScore を []entity.UserWeightScore に変換
func UserWeightScoresToEntities(ms []models.UserWeightScore) []entity.UserWeightScore {
	result := make([]entity.UserWeightScore, len(ms))
	for i, m := range ms {
		e := UserWeightScoreToEntity(&m)
		result[i] = *e
	}
	return result
}

// AnalysisPhaseToEntity models.AnalysisPhase を entity.AnalysisPhase に変換
func AnalysisPhaseToEntity(m *models.AnalysisPhase) *entity.AnalysisPhase {
	if m == nil {
		return nil
	}
	return &entity.AnalysisPhase{
		ID:           m.ID,
		PhaseName:    m.PhaseName,
		DisplayName:  m.DisplayName,
		PhaseOrder:   m.PhaseOrder,
		Description:  m.Description,
		MinQuestions: m.MinQuestions,
		MaxQuestions: m.MaxQuestions,
	}
}

// UserAnalysisProgressToEntity models.UserAnalysisProgress を entity.UserAnalysisProgress に変換
func UserAnalysisProgressToEntity(m *models.UserAnalysisProgress) *entity.UserAnalysisProgress {
	if m == nil {
		return nil
	}
	e := &entity.UserAnalysisProgress{
		ID:              m.ID,
		UserID:          m.UserID,
		SessionID:       m.SessionID,
		PhaseID:         m.PhaseID,
		QuestionsAsked:  m.QuestionsAsked,
		ValidAnswers:    m.ValidAnswers,
		InvalidAnswers:  m.InvalidAnswers,
		CompletionScore: m.CompletionScore,
		IsCompleted:     m.IsCompleted,
		CompletedAt:     m.CompletedAt,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if m.Phase != nil {
		e.Phase = AnalysisPhaseToEntity(m.Phase)
	}
	return e
}

// UserAnalysisProgressFromEntity entity.UserAnalysisProgress を models.UserAnalysisProgress に変換
func UserAnalysisProgressFromEntity(e *entity.UserAnalysisProgress) *models.UserAnalysisProgress {
	if e == nil {
		return nil
	}
	return &models.UserAnalysisProgress{
		ID:              e.ID,
		UserID:          e.UserID,
		SessionID:       e.SessionID,
		PhaseID:         e.PhaseID,
		QuestionsAsked:  e.QuestionsAsked,
		ValidAnswers:    e.ValidAnswers,
		InvalidAnswers:  e.InvalidAnswers,
		CompletionScore: e.CompletionScore,
		IsCompleted:     e.IsCompleted,
		CompletedAt:     e.CompletedAt,
		CreatedAt:       e.CreatedAt,
		UpdatedAt:       e.UpdatedAt,
	}
}
