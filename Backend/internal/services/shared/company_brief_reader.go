// Package shared は internal/services の複数サブパッケージから参照される
// 最小限の共有インターフェースを置く場所。特定クラスタへの依存は持たない。
package shared

import "Backend/internal/models"

// CompanyBriefReader は面接・レビュー用の共有企業キャッシュ参照の最小面。
type CompanyBriefReader interface {
	FindByID(id uint) (*models.Company, error)
	FindByName(name string) (*models.Company, error)
	GetWeightProfile(companyID uint, jobPositionID *uint) (*models.CompanyWeightProfile, error)
}
