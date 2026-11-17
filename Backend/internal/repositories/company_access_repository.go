package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

// CompanyAccessRepository は「このユーザーは管理者か」「この企業の所有者か」を返す。
// shared.CompanyAccessReader を満たす。
type CompanyAccessRepository struct {
	db *gorm.DB
}

func NewCompanyAccessRepository(db *gorm.DB) *CompanyAccessRepository {
	return &CompanyAccessRepository{db: db}
}

// IsAdmin は users.is_admin を返す。db が無ければ false（fail-closed）。
func (r *CompanyAccessRepository) IsAdmin(userID uint) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	var isAdmin bool
	err := r.db.Model(&models.User{}).Select("is_admin").Where("id = ?", userID).Scan(&isAdmin).Error
	return isAdmin, err
}

// OwnsCompany は company_ownerships に (user_id, company_id) があるか返す。
// db が無い、または companyID が 0 なら false（fail-closed）。
func (r *CompanyAccessRepository) OwnsCompany(userID, companyID uint) (bool, error) {
	if r == nil || r.db == nil || companyID == 0 {
		return false, nil
	}
	var n int64
	err := r.db.Model(&models.CompanyOwnership{}).
		Where("user_id = ? AND company_id = ?", userID, companyID).
		Count(&n).Error
	return n > 0, err
}
