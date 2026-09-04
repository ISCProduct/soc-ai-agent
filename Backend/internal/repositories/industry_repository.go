package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

// IndustryRepository は業界マスタの参照（#1094 希望業界フィルタ・希望条件入力で使用）。
type IndustryRepository struct {
	db *gorm.DB
}

func NewIndustryRepository(db *gorm.DB) *IndustryRepository {
	return &IndustryRepository{db: db}
}

// IndustryOption は選択肢として返す最小限の業界情報。
type IndustryOption struct {
	ID       uint   `gorm:"column:id" json:"id"`
	Name     string `gorm:"column:name" json:"name"`
	ParentID *uint  `gorm:"column:parent_id" json:"parent_id,omitempty"`
	Level    int    `gorm:"column:level" json:"level"`
}

// ListActive は有効な業界を表示順で返す。
func (r *IndustryRepository) ListActive() ([]IndustryOption, error) {
	options := []IndustryOption{}
	err := r.db.Model(&models.Industry{}).
		Where("is_active = ?", true).
		Order("level ASC, display_order ASC, id ASC").
		Select("id, name, parent_id, level").
		Scan(&options).Error
	return options, err
}
