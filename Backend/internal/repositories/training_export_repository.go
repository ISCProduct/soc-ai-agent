package repositories

import (
	"Backend/domain/repository"
	"context"

	"gorm.io/gorm"
)

// TrainingExportRepository は repository.TrainingExportRepository の GORM 実装。
type TrainingExportRepository struct {
	db *gorm.DB
}

func NewTrainingExportRepository(db *gorm.DB) *TrainingExportRepository {
	return &TrainingExportRepository{db: db}
}

func (r *TrainingExportRepository) CountOutcomeApplications(ctx context.Context, statuses []string) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("user_application_statuses").
		Where("status IN ?", statuses).
		Count(&n).Error
	return int(n), err
}

func (r *TrainingExportRepository) CountSessionsByStatus(ctx context.Context, statuses []string) (map[string]int, error) {
	type statusRow struct {
		Status string
		N      int
	}
	var rows []statusRow
	if err := r.db.WithContext(ctx).Table("user_application_statuses AS a").
		Select("a.status AS status, COUNT(DISTINCT s.id) AS n").
		Joins("JOIN interview_sessions AS s ON s.user_id = a.user_id AND s.company_id = a.company_id").
		Where("a.status IN ?", statuses).
		Group("a.status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.Status] = row.N
	}
	return out, nil
}

// CountExportableSessions: ユーザー発話が無いセッションは prompt を作れないので学習対象にならない。
func (r *TrainingExportRepository) CountExportableSessions(ctx context.Context, statuses []string) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("interview_sessions AS s").
		Joins("JOIN user_application_statuses AS a ON s.user_id = a.user_id AND s.company_id = a.company_id").
		Joins("JOIN interview_utterances AS u ON u.session_id = s.id AND u.role = ?", "user").
		Where("a.status IN ?", statuses).
		Distinct("s.id").
		Count(&n).Error
	return int(n), err
}

func (r *TrainingExportRepository) CountUserUtterances(ctx context.Context, statuses []string) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("interview_utterances AS u").
		Joins("JOIN interview_sessions AS s ON s.id = u.session_id").
		Joins("JOIN user_application_statuses AS a ON s.user_id = a.user_id AND s.company_id = a.company_id").
		Where("a.status IN ? AND u.role = ?", statuses, "user").
		Count(&n).Error
	return int(n), err
}

func (r *TrainingExportRepository) ListOutcomeSessions(ctx context.Context, statuses []string, limit int) ([]repository.OutcomeSession, error) {
	type sessionRow struct {
		SessionID uint
		Status    string
	}
	q := r.db.WithContext(ctx).Table("interview_sessions AS s").
		Select("s.id AS session_id, a.status AS status").
		Joins("JOIN user_application_statuses AS a ON s.user_id = a.user_id AND s.company_id = a.company_id").
		Where("a.status IN ?", statuses).
		Order("s.id")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var rows []sessionRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]repository.OutcomeSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, repository.OutcomeSession{SessionID: row.SessionID, Status: row.Status})
	}
	return out, nil
}

// ListUserUtterances は role=user の発話だけを返す。AI 発話を prompt に混ぜると、
// モデルの出力を教師信号にすることになり蒸留にあたる。
func (r *TrainingExportRepository) ListUserUtterances(ctx context.Context, sessionIDs []uint) ([]repository.SessionUserUtterance, error) {
	type uttRow struct {
		SessionID uint
		Text      string
	}
	var rows []uttRow
	// created_at は並び順にだけ使う。取り出す必要はない。
	if err := r.db.WithContext(ctx).Table("interview_utterances").
		Select("session_id, text").
		Where("session_id IN ? AND role = ?", sessionIDs, "user").
		Order("session_id, created_at, id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]repository.SessionUserUtterance, 0, len(rows))
	for _, row := range rows {
		out = append(out, repository.SessionUserUtterance{SessionID: row.SessionID, Text: row.Text})
	}
	return out, nil
}
