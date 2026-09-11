package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

// fillOrganizationID は未設定の organization_id を users から解決する。
// DB エラー時はフォールバックせずエラーを返す（誤ってデフォルト組織へ書き込まない）。
func fillOrganizationID(db *gorm.DB, userID uint, orgID *uint) error {
	if orgID == nil || *orgID > 0 {
		return nil
	}
	if userID == 0 {
		*orgID = models.DefaultOrganizationID
		return nil
	}
	var id uint
	if err := db.Model(&models.User{}).Select("organization_id").Where("id = ?", userID).Scan(&id).Error; err != nil {
		return err
	}
	if id == 0 {
		id = models.DefaultOrganizationID
	}
	*orgID = id
	return nil
}
