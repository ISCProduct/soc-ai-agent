package models

import "time"

// IndustryWeightProfile 業界の適性プロファイル（10カテゴリの重視度）#1027。
//
// カラム構成は CompanyWeightProfile と揃えてある。
// 生徒のスコアとの突き合わせに matching.CalculateCategoryMatch を
// そのまま使えるようにするためで、企業マッチングとの一貫性を保つ狙い。
//
// 行が無い業界は「未設定」であり、アプリ側で中立値50として扱う。
// スキーマは migrations/000021_industry_weight_profiles.up.sql で管理。
type IndustryWeightProfile struct {
	ID         uint     `gorm:"primaryKey" json:"id"`
	IndustryID uint     `gorm:"not null;uniqueIndex" json:"industry_id"`
	Industry   Industry `gorm:"foreignKey:IndustryID" json:"-"`

	TechnicalOrientation  int `gorm:"default:50" json:"technical_orientation"`
	TeamworkOrientation   int `gorm:"default:50" json:"teamwork_orientation"`
	LeadershipOrientation int `gorm:"default:50" json:"leadership_orientation"`
	CreativityOrientation int `gorm:"default:50" json:"creativity_orientation"`
	StabilityOrientation  int `gorm:"default:50" json:"stability_orientation"`
	GrowthOrientation     int `gorm:"default:50" json:"growth_orientation"`
	WorkLifeBalance       int `gorm:"default:50" json:"work_life_balance"`
	ChallengeSeeking      int `gorm:"default:50" json:"challenge_seeking"`
	DetailOrientation     int `gorm:"default:50" json:"detail_orientation"`
	CommunicationSkill    int `gorm:"default:50" json:"communication_skill"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (IndustryWeightProfile) TableName() string { return "industry_weight_profiles" }

// NeutralIndustryWeightProfile は未設定業界に使う中立プロファイル。
// 行が無いことをエラーにせず、全カテゴリ50で評価を続ける（PRD 非機能要件）。
func NeutralIndustryWeightProfile(industryID uint) *IndustryWeightProfile {
	return &IndustryWeightProfile{
		IndustryID:            industryID,
		TechnicalOrientation:  50,
		TeamworkOrientation:   50,
		LeadershipOrientation: 50,
		CreativityOrientation: 50,
		StabilityOrientation:  50,
		GrowthOrientation:     50,
		WorkLifeBalance:       50,
		ChallengeSeeking:      50,
		DetailOrientation:     50,
		CommunicationSkill:    50,
	}
}

// WeightByCategory は正典カテゴリ名から重視度を引く。
// キーは domain/valueobject/match.go の10種。
func (p *IndustryWeightProfile) WeightByCategory() map[string]float64 {
	return map[string]float64{
		"技術志向":       float64(p.TechnicalOrientation),
		"チームワーク志向":   float64(p.TeamworkOrientation),
		"リーダーシップ志向":  float64(p.LeadershipOrientation),
		"創造性志向":      float64(p.CreativityOrientation),
		"安定志向":       float64(p.StabilityOrientation),
		"成長志向":       float64(p.GrowthOrientation),
		"ワークライフバランス": float64(p.WorkLifeBalance),
		"チャレンジ志向":    float64(p.ChallengeSeeking),
		"細部志向":       float64(p.DetailOrientation),
		"コミュニケーション力": float64(p.CommunicationSkill),
	}
}
