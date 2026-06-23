package models

import "time"

// ScheduleStage 選考ステージ
type ScheduleStage string

const (
	StageDocument    ScheduleStage = "書類選考"
	StageFirst       ScheduleStage = "1次面接"
	StageSecond      ScheduleStage = "2次面接"
	StageFinal       ScheduleStage = "最終面接"
	StageOffer       ScheduleStage = "内定"
	StageOther       ScheduleStage = "その他"
)

// ScheduleEvent 選考スケジュールイベント
type ScheduleEvent struct {
	ID                   uint          `gorm:"primaryKey"                  json:"id"`
	UserID               uint          `gorm:"not null;index"              json:"user_id"`
	CompanyName          string        `gorm:"size:255;not null"           json:"company_name"`
	Stage                ScheduleStage `gorm:"size:50;not null"            json:"stage"`
	Title                string        `gorm:"size:255"                    json:"title"`
	ScheduledAt          time.Time     `gorm:"not null;index"              json:"scheduled_at"`
	Notes                string        `gorm:"type:text"                   json:"notes"`
	GoogleCalendarEventID string       `gorm:"size:255"                    json:"google_calendar_event_id,omitempty"`
	CreatedAt            time.Time     `                                   json:"created_at"`
	UpdatedAt            time.Time     `                                   json:"updated_at"`
}
