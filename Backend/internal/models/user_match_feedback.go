package models

import "time"

// 推薦へのフィードバックの判定。
const (
	MatchFeedbackFit   = "fit"   // 合う
	MatchFeedbackUnfit = "unfit" // 合わない
)

// UserMatchFeedback は、推薦 1 件への「合う／合わない」（SOCAIAGENT-670）。本人×推薦で 1 行。
// 精度の評価に使うため、押した時点の適合度・ずれる軸・職種の加点を写しで持つ。自由記述は持たない。
type UserMatchFeedback struct {
	ID              uint      `gorm:"primaryKey"`
	UserID          uint      `gorm:"not null;uniqueIndex:uniq_user_match_feedback,priority:1"`
	MatchID         uint      `gorm:"not null;uniqueIndex:uniq_user_match_feedback,priority:2;index:idx_user_match_feedbacks_match"`
	Verdict         string    `gorm:"size:10;not null"`
	MatchScore      float64   `gorm:"not null;default:0"`
	FitMismatchAxes string    `gorm:"size:120;not null;default:''"`
	JobMatched      bool      `gorm:"not null;default:false"`
	CreatedAt       time.Time `gorm:"precision:3"`
	UpdatedAt       time.Time `gorm:"precision:3"`
}

func (UserMatchFeedback) TableName() string { return "user_match_feedbacks" }

// IsValidMatchFeedbackVerdict は、受け付ける判定か。
func IsValidMatchFeedbackVerdict(v string) bool {
	return v == MatchFeedbackFit || v == MatchFeedbackUnfit
}
