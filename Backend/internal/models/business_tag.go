package models

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// BusinessTag は企業が提供する事業・サービスの分類（SOCAIAGENT-717）。
// 全企業で共通の一覧を持ち、表記ゆれを防ぐ。業種(Industry)とは別物。
// スキーマは migrations/20261124184459_create_business_tags.up.sql で管理。
type BusinessTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(64);not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (BusinessTag) TableName() string { return "business_tags" }

// CompanyBusinessTag は企業と事業タグの紐付け。
type CompanyBusinessTag struct {
	CompanyID     uint `gorm:"primaryKey"`
	BusinessTagID uint `gorm:"primaryKey"`
	CreatedAt     time.Time
}

func (CompanyBusinessTag) TableName() string { return "company_business_tags" }

// MaxBusinessTagsPerCompany は1社に付けられる事業タグの上限。
const MaxBusinessTagsPerCompany = 20

// NormalizeBusinessTagNames は登録用にタグ名をそろえる。
// NFKC で全角・半角を統一し、前後の空白を除き、連続する空白を1つにする。
// 空は捨て、同じ名前は1つにまとめる（順序は最初の出現順）。
func NormalizeBusinessTagNames(names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, raw := range names {
		name := strings.Join(strings.Fields(norm.NFKC.String(raw)), " ")
		if name == "" {
			continue
		}
		if utf8.RuneCountInString(name) > MaxTagNameLength {
			return nil, fmt.Errorf("事業タグは%d文字以内にしてください: %q", MaxTagNameLength, name)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) > MaxBusinessTagsPerCompany {
		return nil, fmt.Errorf("事業タグは1社%d件までです", MaxBusinessTagsPerCompany)
	}
	return out, nil
}
