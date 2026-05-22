package interfaces

import "Backend/domain/entity"

// ApplicationService 応募・選考ステータス管理サービスのインターフェース
type ApplicationService interface {
	Apply(userID, companyID, matchID uint) (*entity.UserApplicationStatus, error)
	UpdateStatus(applicationID uint, userID uint, status, notes string) (*entity.UserApplicationStatus, error)
	GetApplicationsByUser(userID uint) ([]*entity.UserApplicationStatus, error)
	GetCorrelation(companyID uint) ([]map[string]interface{}, error)
}
