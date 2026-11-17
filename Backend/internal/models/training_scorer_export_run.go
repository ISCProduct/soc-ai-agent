package models

import "time"

// 採点モデルの学習データの書き出し（#2168）の結果。
const (
	TrainingScorerExportRunning   = "running"
	TrainingScorerExportSucceeded = "succeeded"
	TrainingScorerExportFailed    = "failed"
)

// TrainingScorerExportRun は採点モデルの学習データの書き出し1回ぶんの監査ログ（#2168）。
//
// 個人を特定するもの（利用者 ID・氏名・HMAC 化した ID も含む）は持たない。件数と出力先だけを残すので、
// 退会時のパージ（hardDeleteUserData）の対象にしなくてよい。
type TrainingScorerExportRun struct {
	ID uint `gorm:"primaryKey"`
	// Status は running / succeeded / failed。開始時に running で作り、終わったら更新する。
	// 途中でプロセスが落ちると running のまま残り、終わらなかった回だと分かる。
	Status       string     `gorm:"size:16;not null"`
	StartedAt    time.Time  `gorm:"not null"`
	FinishedAt   *time.Time `gorm:"column:finished_at"`
	StudentCount int        `gorm:"not null;default:0"`
	SessionCount int        `gorm:"not null;default:0"`
	// Destination は出力先（s3://bucket/prefix/runs/<実行ID>/）。
	Destination string `gorm:"size:1024;not null"`
	// ErrorMessage は失敗したときのエラー。成功なら空。
	ErrorMessage string `gorm:"type:text"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TableName は監査ログのテーブル名。
func (TrainingScorerExportRun) TableName() string {
	return "training_scorer_export_runs"
}
