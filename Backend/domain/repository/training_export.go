package repository

import "context"

// OutcomeSession は、選考が結論まで進んだ応募に紐づく面接セッション。
type OutcomeSession struct {
	SessionID uint
	Status    string // 教師ラベル(選考ステータス)
}

// SessionUserUtterance は面接セッションのユーザー発話1件。
type SessionUserUtterance struct {
	SessionID uint
	Text      string
}

// TrainingExportRepository は学習データのエクスポートに使う、集計と取得を担う。
// statuses は教師ラベルになりうる選考ステータス(呼び出し側が決める)。
// 発話は、AI の発話を混ぜると蒸留にあたるため、実装は role=user だけを返すこと。
type TrainingExportRepository interface {
	// CountOutcomeApplications は結論が出た応募の数。
	CountOutcomeApplications(ctx context.Context, statuses []string) (int, error)
	// CountSessionsByStatus は、結論が出た応募に紐づく面接セッション数をステータス別に返す。
	CountSessionsByStatus(ctx context.Context, statuses []string) (map[string]int, error)
	// CountExportableSessions は、ユーザー発話が1つ以上あるセッション数。
	CountExportableSessions(ctx context.Context, statuses []string) (int, error)
	// CountUserUtterances は、対象セッションのユーザー発話の総数。
	CountUserUtterances(ctx context.Context, statuses []string) (int, error)
	// ListOutcomeSessions は対象セッションを id 順に返す。limit が 0 以下なら全件。
	ListOutcomeSessions(ctx context.Context, statuses []string, limit int) ([]OutcomeSession, error)
	// ListUserUtterances は sessionIDs のユーザー発話を、セッション・発話順に返す。
	ListUserUtterances(ctx context.Context, sessionIDs []uint) ([]SessionUserUtterance, error)
}
