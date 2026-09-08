package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

type IndustryWeightProfileRepository struct {
	db *gorm.DB
}

func NewIndustryWeightProfileRepository(db *gorm.DB) *IndustryWeightProfileRepository {
	return &IndustryWeightProfileRepository{db: db}
}

// ListAll は業界プロファイルを全件返す（#1027）。
// 業界数は十数件のオーダーなのでページングしない。
// 未設定の業界は行が無いので、呼び出し側で中立値へフォールバックすること。
func (r *IndustryWeightProfileRepository) ListAll() ([]models.IndustryWeightProfile, error) {
	var profiles []models.IndustryWeightProfile
	err := r.db.Find(&profiles).Error
	return profiles, err
}
