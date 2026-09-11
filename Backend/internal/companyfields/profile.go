// Package companyfields は業界ごとに入力可能な企業フィールドを定義する。
// Frontend の frontend/lib/admin-company-field-profile.ts と対応を揃えること。
package companyfields

import (
	"strings"
)

// ProfileID は業界プロファイル識別子。
type ProfileID string

const (
	ProfileIT            ProfileID = "it"
	ProfileManufacturing ProfileID = "manufacturing"
	ProfileFinance       ProfileID = "finance"
	ProfileConsulting    ProfileID = "consulting"
	ProfileEducation     ProfileID = "education"
	ProfileHealthcare    ProfileID = "healthcare"
	ProfileGeneral       ProfileID = "general"
)

// Profile は業界ごとの入力・取得方針。
type Profile struct {
	ID                    ProfileID
	Label                 string
	MatchKeywords         []string
	TechAspectEnabled     bool
	RequireTechForPublish bool
}

// profiles はマッチ優先順（先に当たったものが採用される）。
var profiles = []Profile{
	{
		ID:                    ProfileIT,
		Label:                 "IT・ソフトウェア",
		MatchKeywords:         []string{"it", "ｉｔ", "情報", "ソフト", "software", "web", "ウェブ", "saas", "システム開発", "通信", "インターネット", "ゲーム"},
		TechAspectEnabled:     true,
		RequireTechForPublish: true,
	},
	{
		ID:                    ProfileManufacturing,
		Label:                 "製造業",
		MatchKeywords:         []string{"製造", "メーカー", "自動車", "機械", "電機", "電子", "ものづくり", "工場"},
		TechAspectEnabled:     true,
		RequireTechForPublish: false,
	},
	{
		ID:                    ProfileFinance,
		Label:                 "金融・保険",
		MatchKeywords:         []string{"金融", "銀行", "保険", "証券", "クレジット"},
		TechAspectEnabled:     false,
		RequireTechForPublish: false,
	},
	{
		ID:                    ProfileConsulting,
		Label:                 "コンサルティング",
		MatchKeywords:         []string{"コンサル"},
		TechAspectEnabled:     false,
		RequireTechForPublish: false,
	},
	{
		ID:                    ProfileEducation,
		Label:                 "教育",
		MatchKeywords:         []string{"教育", "学校", "学習", "大学", "塾"},
		TechAspectEnabled:     false,
		RequireTechForPublish: false,
	},
	{
		ID:                    ProfileHealthcare,
		Label:                 "医療・福祉",
		MatchKeywords:         []string{"医療", "福祉", "病院", "ヘルスケア", "介護"},
		TechAspectEnabled:     false,
		RequireTechForPublish: false,
	},
	{
		ID:                    ProfileGeneral,
		Label:                 "その他",
		MatchKeywords:         nil,
		TechAspectEnabled:     false,
		RequireTechForPublish: false,
	},
}

// Resolve は industry 文字列からプロファイルを返す。
func Resolve(industry string) Profile {
	normalized := strings.ToLower(strings.TrimSpace(industry))
	if normalized == "" {
		return profiles[len(profiles)-1] // general
	}
	for _, p := range profiles {
		if p.ID == ProfileGeneral {
			continue
		}
		for _, kw := range p.MatchKeywords {
			if strings.Contains(normalized, strings.ToLower(kw)) {
				return p
			}
		}
	}
	return profiles[len(profiles)-1]
}

// RequiresTech は技術情報の取得・公開必須判定。
func RequiresTech(industry string) bool {
	return Resolve(industry).RequireTechForPublish
}

// TechAspectEnabled は技術情報タブを出すか。
func TechAspectEnabled(industry string) bool {
	return Resolve(industry).TechAspectEnabled
}

// TechEmptyStepStatus は技術取得後にデータが空だったときのステップ結果を返す。
// 公開必須でない業種は empty ではなく skipped(optional_empty) にする。
func TechEmptyStepStatus(industry string) (status, detail string, countAsError bool) {
	if !RequiresTech(industry) {
		return "skipped", "optional_empty", false
	}
	return "empty", "no_tech_stack", true
}

// TechRequiredIndustrySQL は「技術情報が必須の業界」にマッチする SQL 断片と引数を返す。
// column は companies.industry など。
func TechRequiredIndustrySQL(column string) (cond string, args []any) {
	var parts []string
	for _, p := range profiles {
		if !p.RequireTechForPublish {
			continue
		}
		for _, kw := range p.MatchKeywords {
			parts = append(parts, column+" LIKE ?")
			args = append(args, "%"+kw+"%")
		}
	}
	if len(parts) == 0 {
		return "0=1", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}
