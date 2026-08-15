package interfaces

import "Backend/internal/services/flywheel"

type CrossFeatureIntegrationService interface {
	BuildIntegratedProfile(userID uint, chatSessionID string, interviewCount int, resumeReviewDone bool) (*flywheel.UserIntegratedProfile, error)
}
