package interfaces

import "Backend/internal/services"

type CrossFeatureIntegrationService interface {
	BuildIntegratedProfile(userID uint, chatSessionID string, interviewCount int, resumeReviewDone bool) (*services.UserIntegratedProfile, error)
}
