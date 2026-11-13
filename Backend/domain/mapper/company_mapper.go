package mapper

import (
	"Backend/domain/entity"
	"Backend/internal/models"
)

// CompanyToEntity models.Company を entity.Company に変換
func CompanyToEntity(m *models.Company) *entity.Company {
	if m == nil {
		return nil
	}
	return &entity.Company{
		ID:                 m.ID,
		Name:               m.Name,
		Description:        m.Description,
		Industry:           m.Industry,
		EmployeeCount:      m.EmployeeCount,
		EmployeeCountBasis: m.EmployeeCountBasis,
		FoundedYear:        m.FoundedYear,
		Location:           m.Location,
		WebsiteURL:         m.WebsiteURL,
		LogoURL:            m.LogoURL,
		CorporateNumber:    m.CorporateNumber,
		SourceType:         m.SourceType,
		SourceURL:          m.SourceURL,
		SourceFetchedAt:    m.SourceFetchedAt,
		IsProvisional:      m.IsProvisional,
		DataStatus:         m.DataStatus,
		Culture:            m.Culture,
		WorkStyle:          m.WorkStyle,
		WelfareDetails:     m.WelfareDetails,
		TechStack:          m.TechStack,
		DevelopmentStyle:   m.DevelopmentStyle,
		MainBusiness:       m.MainBusiness,
		AverageAge:         m.AverageAge,
		FemaleRatio:        m.FemaleRatio,
		IsActive:           m.IsActive,
		IsVerified:         m.IsVerified,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}

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
	e.Company = CompanyToEntity(&m.Company)
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
