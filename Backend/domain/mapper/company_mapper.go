package mapper

import (
	"Backend/domain/entity"
	"Backend/internal/models"
)

// UserCompanyMatchToEntity models.UserCompanyMatch を entity.UserCompanyMatch に変換
func UserCompanyMatchToEntity(m *models.UserCompanyMatch) *entity.UserCompanyMatch {
	if m == nil {
		return nil
	}
	e := &entity.UserCompanyMatch{
		ID:                       m.ID,
		UserID:                   m.UserID,
		SessionID:                m.SessionID,
		CompanyID:                m.CompanyID,
		MatchScore:               m.MatchScore,
		TechnicalMatch:           m.TechnicalMatch,
		TeamworkMatch:            m.TeamworkMatch,
		LeadershipMatch:          m.LeadershipMatch,
		CreativityMatch:          m.CreativityMatch,
		StabilityMatch:           m.StabilityMatch,
		GrowthMatch:              m.GrowthMatch,
		WorkLifeMatch:            m.WorkLifeMatch,
		ChallengeMatch:           m.ChallengeMatch,
		DetailMatch:              m.DetailMatch,
		CommunicationMatch:       m.CommunicationMatch,
		MatchedAxisCount:         m.MatchedAxisCount,
		MatchReason:              m.MatchReason,
		MatchReasonPromptVersion: m.MatchReasonPromptVersion,
		IsViewed:                 m.IsViewed,
		IsFavorited:              m.IsFavorited,
		IsApplied:                m.IsApplied,
		CreatedAt:                m.CreatedAt,
		UpdatedAt:                m.UpdatedAt,
	}
	e.Company = m.Company
	return e
}

// UserCompanyMatchFromEntity entity.UserCompanyMatch を models.UserCompanyMatch に変換
func UserCompanyMatchFromEntity(e *entity.UserCompanyMatch) *models.UserCompanyMatch {
	if e == nil {
		return nil
	}
	m := &models.UserCompanyMatch{
		ID:                       e.ID,
		UserID:                   e.UserID,
		SessionID:                e.SessionID,
		CompanyID:                e.CompanyID,
		MatchScore:               e.MatchScore,
		TechnicalMatch:           e.TechnicalMatch,
		TeamworkMatch:            e.TeamworkMatch,
		LeadershipMatch:          e.LeadershipMatch,
		CreativityMatch:          e.CreativityMatch,
		StabilityMatch:           e.StabilityMatch,
		GrowthMatch:              e.GrowthMatch,
		WorkLifeMatch:            e.WorkLifeMatch,
		ChallengeMatch:           e.ChallengeMatch,
		DetailMatch:              e.DetailMatch,
		CommunicationMatch:       e.CommunicationMatch,
		MatchedAxisCount:         e.MatchedAxisCount,
		MatchReason:              e.MatchReason,
		MatchReasonPromptVersion: e.MatchReasonPromptVersion,
		IsViewed:                 e.IsViewed,
		IsFavorited:              e.IsFavorited,
		IsApplied:                e.IsApplied,
		CreatedAt:                e.CreatedAt,
		UpdatedAt:                e.UpdatedAt,
	}
	return m
}
