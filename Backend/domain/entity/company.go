package entity

import "time"

// CompanyWeightProfile 企業の適性プロファイル（10カテゴリの重視度）
type CompanyWeightProfile struct {
	ID                    uint
	CompanyID             uint
	JobPositionID         *uint
	TechnicalOrientation  int // 技術志向 (0-100)
	TeamworkOrientation   int // チームワーク志向 (0-100)
	LeadershipOrientation int // リーダーシップ志向 (0-100)
	CreativityOrientation int // 創造性志向 (0-100)
	StabilityOrientation  int // 安定志向 (0-100)
	GrowthOrientation     int // 成長志向 (0-100)
	WorkLifeBalance       int // ワークライフバランス (0-100)
	ChallengeSeeking      int // チャレンジ志向 (0-100)
	DetailOrientation     int // 細部志向 (0-100)
	CommunicationSkill    int // コミュニケーション力 (0-100)
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
