package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

// fillOrganizationID は未設定の organization_id を users から解決する。
func fillOrganizationID(db *gorm.DB, userID uint, orgID *uint) {
	if orgID == nil || *orgID > 0 {
		return
	}
	var id uint
	if userID > 0 {
		_ = db.Model(&models.User{}).Select("organization_id").Where("id = ?", userID).Scan(&id).Error
	}
	if id == 0 {
		id = models.DefaultOrganizationID
	}
	*orgID = id
}
