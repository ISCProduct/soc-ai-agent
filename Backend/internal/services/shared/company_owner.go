package shared

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

// UserIsAdmin は users.is_admin を返す。db が nil なら false（fail-closed）。
func UserIsAdmin(db *gorm.DB, userID uint) (bool, error) {
	if db == nil {
		return false, nil
	}
	var isAdmin bool
	err := db.Model(&models.User{}).Select("is_admin").Where("id = ?", userID).Scan(&isAdmin).Error
	return isAdmin, err
}

// UserOwnsCompany は company_ownerships に (user_id, company_id) があるか返す。
// db が nil、または companyID が 0 なら false（fail-closed）。
func UserOwnsCompany(db *gorm.DB, userID, companyID uint) (bool, error) {
	if db == nil || companyID == 0 {
		return false, nil
	}
	var n int64
	err := db.Model(&models.CompanyOwnership{}).
		Where("user_id = ? AND company_id = ?", userID, companyID).
		Count(&n).Error
	return n > 0, err
}
