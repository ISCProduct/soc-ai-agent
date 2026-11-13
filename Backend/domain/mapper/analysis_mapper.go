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
		ID:                 m.ID,
		UserID:             m.UserID,
		SessionID:          m.SessionID,
		WeightCategory:     m.WeightCategory,
		Score:              m.Score,
		CalculationVersion: m.CalculationVersion,
		PromptVersion:      m.PromptVersion,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
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
