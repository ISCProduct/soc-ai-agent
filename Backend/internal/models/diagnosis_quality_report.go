package models

import "time"

// DiagnosisQualityReport は診断妥当性のバックグラウンド評価結果。
// スコアやマッチ行は更新せず、信頼度とフラグのみを残す。
type DiagnosisQualityReport struct {
	ID         uint   `gorm:"primaryKey"`
	UserID     uint   `gorm:"not null;uniqueIndex:uniq_diagnosis_quality_user_session,priority:1"`
	SessionID  string `gorm:"type:varchar(255);not null;uniqueIndex:uniq_diagnosis_quality_user_session,priority:2"`
	Confidence int    `gorm:"not null;default:0"` // 0-100
	FlagsJSON  string `gorm:"type:json"`
	Summary    string `gorm:"type:text"`
	RawJSON    string `gorm:"type:json"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (DiagnosisQualityReport) TableName() string {
	return "diagnosis_quality_reports"
}
