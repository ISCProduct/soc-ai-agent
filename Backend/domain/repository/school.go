package repository

import "Backend/internal/models"

// SchoolRepository は個別校・担当管理者・企業掲載承認リストの永続化インターフェース。
type SchoolRepository interface {
	Create(school *models.School) error
	Update(school *models.School) error
	FindByID(id uint) (*models.School, error)
	FindByName(name string) (*models.School, error)
	List(limit, offset int) ([]models.School, int64, error)

	AddMember(m *models.AdminSchoolMembership) error
	RemoveMember(userID, schoolID uint) error
	ListSchoolsForAdmin(userID uint) ([]models.School, error)

	AddCompanyApproval(a *models.SchoolCompanyApproval) error
	RemoveCompanyApproval(schoolID, companyID uint) error
	IsCompanyApproved(schoolID, companyID uint) (bool, error)
	ListApprovedCompanyIDs(schoolID uint) ([]uint, error)
}
