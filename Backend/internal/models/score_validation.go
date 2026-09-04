package models

import "time"

// QuestionVariant A/Bテスト用の質問セットバリアント定義
type QuestionVariant struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ExperimentName string    `gorm:"type:varchar(100);not null;index:idx_exp_variant" json:"experiment_name"` // 実験名（例: "phase1_2024q1"）
	VariantName    string    `gorm:"type:varchar(50);not null;index:idx_exp_variant" json:"variant_name"`     // バリアント名（例: "control", "treatment_a"）
	Description    string    `gorm:"type:text" json:"description"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	TrafficRatio   float64   `gorm:"default:0.5" json:"traffic_ratio"` // 割り当て比率 0-1
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// VariantAssignment セッションへのバリアント割り当て記録
type VariantAssignment struct {
	ID              uint             `gorm:"primaryKey" json:"id"`
	UserID          uint             `gorm:"not null;index:idx_user_session_variant" json:"user_id"`
	SessionID       string           `gorm:"type:varchar(100);not null;index:idx_user_session_variant" json:"session_id"`
	VariantID       uint             `gorm:"not null;index" json:"variant_id"`
	Variant         *QuestionVariant `gorm:"foreignKey:VariantID" json:"variant,omitempty"`
	ExperimentName  string           `gorm:"type:varchar(100);not null" json:"experiment_name"`
	AssignedVariant string           `gorm:"type:varchar(50);not null" json:"assigned_variant"`
	CreatedAt       time.Time        `json:"created_at"`
}

// ScoreCalibrationWeight スコアキャリブレーション重み
// 実績データに基づいてカテゴリ別スコアの重要度を調整する
type ScoreCalibrationWeight struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Category    string    `gorm:"type:varchar(100);not null;uniqueIndex:idx_cat_version" json:"category"`
	Version     int       `gorm:"not null;uniqueIndex:idx_cat_version;default:1" json:"version"`
	Weight      float64   `gorm:"not null;default:1.0" json:"weight"` // 乗数（デフォルト1.0）
	SampleCount int       `gorm:"not null;default:0" json:"sample_count"`
	PassRate    float64   `gorm:"not null;default:0" json:"pass_rate"`   // キャリブレーション時の通過率
	Correlation float64   `gorm:"not null;default:0" json:"correlation"` // スコアと通過率の相関係数
	IsActive    bool      `gorm:"default:false" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
