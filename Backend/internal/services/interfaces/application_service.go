package interfaces

import "Backend/internal/models"

type ApplicationService interface {
	Apply(userID, companyID, matchID uint) (*models.UserApplicationStatus, error)
	UpdateStatus(applicationID uint, userID uint, status string, notes *string, isAdmin bool) (*models.UserApplicationStatus, error)
	Withdraw(applicationID, userID uint, isAdmin bool) (*models.UserApplicationStatus, error)
	Accept(applicationID, userID uint, isAdmin bool) (*models.UserApplicationStatus, error)
	GetApplicationsByUser(userID uint) ([]*models.UserApplicationStatus, error)
	ListForAdmin(userID, companyID uint, status string, schoolID *uint) ([]*models.UserApplicationStatus, error)
	OwnerSchoolID(applicationID uint) (*uint, error)
	ListForOwner(userID, companyID uint, status string) ([]*models.UserApplicationStatus, error)
	UpdateStatusAsOwner(applicationID, userID uint, status string, notes *string) (*models.UserApplicationStatus, error)
	GetCorrelation(userID, companyID uint) ([]map[string]any, error)
}
