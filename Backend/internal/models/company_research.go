package models

import "time"

// 企業研究AIの材料（SOCAIAGENT-722 / 724）。

// 情報源の種類。
const (
	ResearchSourceEDINET       = "edinet"
	ResearchSourceGBizINFO     = "gbizinfo"
	ResearchSourceOfficialSite = "official_site"
	ResearchSourceWebSearch    = "web_search"
)

// CompanyDocument は公的資料・公式サイトから取得した文章。RAG のベクトルはここから作り直せる。
type CompanyDocument struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	CompanyID   uint       `gorm:"not null" json:"company_id"`
	SourceType  string     `gorm:"type:varchar(32);not null" json:"source_type"`
	SourceRef   string     `gorm:"type:varchar(128);not null" json:"source_ref"`
	Title       string     `gorm:"type:varchar(255);not null;default:''" json:"title"`
	URL         string     `gorm:"type:varchar(500);not null;default:''" json:"url"`
	AsOf        *time.Time `gorm:"type:date" json:"as_of,omitempty"`
	FetchedAt   time.Time  `gorm:"not null" json:"fetched_at"`
	ContentHash string     `gorm:"type:char(64);not null" json:"content_hash"`
	Body        string     `gorm:"type:mediumtext;not null" json:"body"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CompanyFact は数字1項目。出典と対象年度を必ず持つ。
type CompanyFact struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	CompanyID  uint       `gorm:"not null" json:"company_id"`
	Field      string     `gorm:"type:varchar(64);not null" json:"field"`
	Value      string     `gorm:"type:varchar(255);not null" json:"value"`
	Unit       string     `gorm:"type:varchar(16);not null;default:''" json:"unit"`
	SourceType string     `gorm:"type:varchar(32);not null" json:"source_type"`
	SourceURL  string     `gorm:"type:varchar(500);not null;default:''" json:"source_url"`
	AsOf       *time.Time `gorm:"type:date" json:"as_of,omitempty"`
	FetchedAt  time.Time  `gorm:"not null" json:"fetched_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
