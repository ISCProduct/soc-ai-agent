package repositories

import (
	"Backend/domain/repository"
	"Backend/internal/models"
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

// trainingConsentedUserIDs は「学習に使ってよい学生」のIDの副問い合わせ。
// 学習利用に同意済み・未退会・学生ロール・非ゲスト。退会の猶予期間中は論理削除で allow_training_use が
// 1 のまま残るため、フラグだけでは判定しない。同意の画面は学生にしか出さないので、学生以外は対象にしない。
// 学習データへ書き出すすべての問い合わせは、この条件を通す。同意していない人の面接を、学習に使わないため。
func (r *TrainingExportRepository) trainingConsentedUserIDs(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("users AS tu").Select("tu.id").
		Where("tu.allow_training_use = ?", true).
		Where("tu.withdrawn_at IS NULL").
		Where("tu.role = ?", models.UserRoleStudent).
		Where("tu.is_guest = ?", false)
}

func (r *TrainingExportRepository) CountOutcomeApplications(ctx context.Context, statuses []string) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("user_application_statuses").
		Where("status IN ?", statuses).
		Where("user_id IN (?)", r.trainingConsentedUserIDs(ctx)).
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
		Where("a.user_id IN (?)", r.trainingConsentedUserIDs(ctx)).
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
		Where("a.user_id IN (?)", r.trainingConsentedUserIDs(ctx)).
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
		Where("a.user_id IN (?)", r.trainingConsentedUserIDs(ctx)).
		Count(&n).Error
	return int(n), err
}

func (r *TrainingExportRepository) ListOutcomeSessions(ctx context.Context, statuses []string, limit, offset int) ([]repository.OutcomeSession, error) {
	type sessionRow struct {
		SessionID uint
		Status    string
	}
	q := r.db.WithContext(ctx).Table("interview_sessions AS s").
		Select("s.id AS session_id, a.status AS status").
		Joins("JOIN user_application_statuses AS a ON s.user_id = a.user_id AND s.company_id = a.company_id").
		Where("a.status IN ?", statuses).
		Where("a.user_id IN (?)", r.trainingConsentedUserIDs(ctx)).
		Order("s.id")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
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
		// 一覧を取ったあとに同意を取り消した人の発話を、ここで取り出さない（セッション単位で条件を付け直す）
		Where("session_id IN (?)", r.db.WithContext(ctx).Table("interview_sessions AS s").Select("s.id").
			Where("s.user_id IN (?)", r.trainingConsentedUserIDs(ctx))).
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
