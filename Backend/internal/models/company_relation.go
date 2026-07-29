package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// CompanyRelation 企業間の関係（資本関係・ビジネス関係）
type CompanyRelation struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	ParentID     *uint          `gorm:"index" json:"parent_id"`                         // 親会社ID（資本関係の場合）
	ChildID      *uint          `gorm:"index" json:"child_id"`                          // 子会社ID（資本関係の場合）
	FromID       *uint          `gorm:"index" json:"from_id"`                           // 供給元企業ID（ビジネス関係の場合）
	ToID         *uint          `gorm:"index" json:"to_id"`                             // 供給先企業ID（ビジネス関係の場合）
	RelationType string         `gorm:"type:varchar(50);not null" json:"relation_type"` // 関係タイプ: capital_subsidiary, capital_affiliate, business
	Ratio        *float64       `json:"ratio,omitempty"`                                // 出資比率（資本関係の場合）
	Description  string         `gorm:"type:text" json:"description"`                   // 関係の説明
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// リレーション
	Parent *Company `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Child  *Company `gorm:"foreignKey:ChildID" json:"child,omitempty"`
	From   *Company `gorm:"foreignKey:FromID" json:"from,omitempty"`
	To     *Company `gorm:"foreignKey:ToID" json:"to,omitempty"`
}

// CompanyMarketInfo 企業の市場情報
type CompanyMarketInfo struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CompanyID   uint           `gorm:"uniqueIndex;not null" json:"company_id"`
	MarketType  string         `gorm:"type:varchar(20)" json:"market_type"` // prime, standard, growth, unlisted
	IsListed    bool           `gorm:"default:false" json:"is_listed"`
	StockCode   string         `gorm:"type:varchar(10)" json:"stock_code,omitempty"`
	MarketCap   *float64       `json:"market_cap,omitempty"`                           // 時価総額（百万円）
	ListingDate *string        `gorm:"type:varchar(20)" json:"listing_date,omitempty"` // 上場日（YYYY-MM-DD形式）
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// リレーション
	Company *Company `gorm:"foreignKey:CompanyID" json:"company,omitempty"`
}

// TableName テーブル名を指定
func (CompanyRelation) TableName() string {
	return "company_relations"
}

func (CompanyMarketInfo) TableName() string {
	return "company_market_info"
}

// CompanyIDs は関係に含まれる全企業IDを返す（nil は除外）。
func (r CompanyRelation) CompanyIDs() []uint {
	ids := make([]uint, 0, 4)
	if r.ParentID != nil {
		ids = append(ids, *r.ParentID)
	}
	if r.ChildID != nil {
		ids = append(ids, *r.ChildID)
	}
	if r.FromID != nil {
		ids = append(ids, *r.FromID)
	}
	if r.ToID != nil {
		ids = append(ids, *r.ToID)
	}
	return ids
}

// InvolvesCompany は関係が指定企業IDに関わるかどうかを返す。
func (r CompanyRelation) InvolvesCompany(id uint) bool {
	return (r.ParentID != nil && *r.ParentID == id) ||
		(r.ChildID != nil && *r.ChildID == id) ||
		(r.FromID != nil && *r.FromID == id) ||
		(r.ToID != nil && *r.ToID == id)
}

// IsCapitalRelationType は資本関係（parent/child で保存・表示する）かどうかを返す。
func IsCapitalRelationType(relationType string) bool {
	return strings.HasPrefix(relationType, "capital_")
}

