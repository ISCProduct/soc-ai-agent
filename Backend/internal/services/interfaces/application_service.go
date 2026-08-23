package interfaces

import "Backend/domain/entity"

type ApplicationService interface {
	Apply(userID, companyID, matchID uint) (*entity.UserApplicationStatus, error)
	UpdateStatus(applicationID uint, userID uint, status, notes string, isAdmin bool) (*entity.UserApplicationStatus, error)
	Withdraw(applicationID, userID uint, isAdmin bool) (*entity.UserApplicationStatus, error)
	Accept(applicationID, userID uint, isAdmin bool) (*entity.UserApplicationStatus, error)
	GetApplicationsByUser(userID uint) ([]*entity.UserApplicationStatus, error)
	ListForAdmin(userID, companyID uint, status string) ([]*entity.UserApplicationStatus, error)
	GetCorrelation(companyID uint) ([]map[string]any, error)
}
