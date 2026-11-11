package repositories

import (
	"time"

	"Backend/internal/models"
	"Backend/internal/services/funnel"

	"gorm.io/gorm"
)

// ConversionFunnelRepository は機能横断のコンバージョンファネル（#2039）の、
// 対象の学生と段階ごとの到達者を引く。
//
// 対象の学生は「退会していない学生（role=student、管理者を除く）」で、schoolID が非nilなら
// その学校（同じ組織）に限る。担当校以外を数えないよう、どの段階もこの学生の集合で絞る。
type ConversionFunnelRepository struct {
	db *gorm.DB
}

func NewConversionFunnelRepository(db *gorm.DB) *ConversionFunnelRepository {
	return &ConversionFunnelRepository{db: db}
}

// FunnelPeriod は集計期間。From は含み、To は含まない。nil は制限なし。
type FunnelPeriod struct {
	From *time.Time
	To   *time.Time
}

// studentsQuery は対象の学生の ID を返すサブクエリ。
func (r *ConversionFunnelRepository) studentsQuery(schoolID *uint) *gorm.DB {
	q := r.db.Model(&models.User{}).Select("users.id").
		Where("users.withdrawn_at IS NULL").
		Where("users.role = ?", models.UserRoleStudent).
		Where("users.is_admin = ?", false)
	return scopeUsersBySchool(q, schoolID)
}

// Students は対象の学生の ID を返す。
func (r *ConversionFunnelRepository) Students(schoolID *uint) ([]uint, error) {
	var ids []uint
	err := r.studentsQuery(schoolID).Pluck("users.id", &ids).Error
	return ids, err
}

// withPeriod は column が期間に入る行に絞る。
func withPeriod(q *gorm.DB, column string, p FunnelPeriod) *gorm.DB {
	if p.From != nil {
		q = q.Where(column+" >= ?", *p.From)
	}
	if p.To != nil {
		q = q.Where(column+" < ?", *p.To)
	}
	return q
}

// StageUserIDs は段階ごとに、期間内にその段階に到達した対象の学生の ID を返す。
//
// 段階の定義（いつ到達したとみなすか）:
//   - 診断: AI 診断チャットで回答した（chat_messages の role=user の発言）、または学校の診断を提出した（assessment_sessions.submitted_at）
//   - 面接: AI 面接を最後まで終えた（interview_sessions.status=finished、開始日時で期間を見る）
//   - ES: ES の添削を受けた（es_reviews）
//   - 応募: 選考ステータスが応募済み以降（not_applied 以外）。応募日（無ければ登録日時）で期間を見る
//   - 内定: 選考ステータスが内定・内定承諾。ステータスの最終更新日（無ければ更新日時）で期間を見る
//
// 選考ステータスは履歴を持たないので、内定のあと辞退した学生（withdrawn）は内定に数えられない。
func (r *ConversionFunnelRepository) StageUserIDs(schoolID *uint, p FunnelPeriod) (map[string][]uint, error) {
	students := r.studentsQuery(schoolID)
	out := make(map[string][]uint, len(funnel.Stages))

	pluck := func(stage string, q *gorm.DB) error {
		var ids []uint
		if err := q.Distinct().Pluck("user_id", &ids).Error; err != nil {
			return err
		}
		out[stage] = append(out[stage], ids...)
		return nil
	}

	chat := withPeriod(r.db.Table("chat_messages").Where("role = ?", "user").Where("user_id IN (?)", students), "created_at", p)
	if err := pluck(funnel.StageDiagnosis, chat); err != nil {
		return nil, err
	}
	assessment := withPeriod(r.db.Table("assessment_sessions").Where("submitted_at IS NOT NULL").Where("user_id IN (?)", students), "submitted_at", p)
	if err := pluck(funnel.StageDiagnosis, assessment); err != nil {
		return nil, err
	}
	interview := withPeriod(r.db.Table("interview_sessions").Where("status = ?", "finished").Where("user_id IN (?)", students), "created_at", p)
	if err := pluck(funnel.StageInterview, interview); err != nil {
		return nil, err
	}
	es := withPeriod(r.db.Table("es_reviews").Where("user_id IN (?)", students), "created_at", p)
	if err := pluck(funnel.StageES, es); err != nil {
		return nil, err
	}
	applied := withPeriod(r.db.Table("user_application_statuses").Where("status <> ?", "not_applied").Where("user_id IN (?)", students),
		"COALESCE(applied_at, created_at)", p)
	if err := pluck(funnel.StageApplied, applied); err != nil {
		return nil, err
	}
	offered := withPeriod(r.db.Table("user_application_statuses").Where("status IN ?", []string{"offered", "accepted"}).Where("user_id IN (?)", students),
		"COALESCE(status_updated_at, updated_at)", p)
	if err := pluck(funnel.StageOffered, offered); err != nil {
		return nil, err
	}
	return out, nil
}
