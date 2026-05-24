package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services"
)

type CollectiveInsightService interface {
	GetCollectiveRecommendations(userID uint, sessionID string, excludeCompanyIDs []uint) ([]services.CollectiveRecommendItem, error)
	GetTopPassRateCompanies(limit int) ([]models.AnonymizedBehaviorSummary, error)
	UpdateConsent(userID uint, allow bool) error
	RecordAction(userID uint, sessionID string, companyID uint, actionType string) error
	RebuildSummaries() error
}
