package interfaces

import "Backend/domain/entity"

type ApplicationService interface {
	Apply(userID, companyID, matchID uint) (*entity.UserApplicationStatus, error)
	UpdateStatus(applicationID uint, userID uint, status, notes string) (*entity.UserApplicationStatus, error)
	GetApplicationsByUser(userID uint) ([]*entity.UserApplicationStatus, error)
	GetCorrelation(companyID uint) ([]map[string]any, error)
}
