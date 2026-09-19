// Package training は学習データのエクスポートを担う。
//
// RAG 側には受け取ったセッション配列を学習用 JSONL へ変換する経路
// (rag/training_api.py の /training/export) が既にあるが、DB から
// セッションを取り出す側が無く、パイプラインが繋がっていなかった。
// ここがその欠けていた一本になる。
//
// 教師ラベルは選考結果(user_application_statuses.status)のみを使う。
// AI の発話や自動採点は一切含めない。これは rag/export_training_data.py の
// 方針と揃えている(蒸留の回避)。
package training

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// OutcomeStatuses は教師ラベルになりうる選考ステータス。
// rag/export_training_data.py の OUTCOME_LABELS と一致させること。
// 片方だけ増やすと、Backend が出したデータを RAG 側が黙って捨てる。
var OutcomeStatuses = []string{"rejected", "offered", "accepted"}

// Utterance は面接の1発話。RAG 側が期待する形に合わせる。
type Utterance struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// Session は RAG の /training/export へ渡す1件分。
type Session struct {
	ID                uint        `json:"id"`
	ApplicationStatus string      `json:"application_status"`
	Utterances        []Utterance `json:"utterances"`
}

// Stats はエクスポート対象の件数。学習に足りる量があるかの判断に使う。
type Stats struct {
	// 選考が結論まで進んだ応募の数。
	OutcomeApplications int `json:"outcome_applications"`
	// うち面接セッションが紐づくもの。
	SessionsWithOutcome int `json:"sessions_with_outcome"`
	// 実際に学習データになる数（ユーザー発話が1つ以上ある）。
	ExportableSessions int `json:"exportable_sessions"`
	// ステータス別の内訳。偏りがあると学習が傾く。
	ByStatus map[string]int `json:"by_status"`
	// ユーザー発話の総数。1セッションあたりの厚みを見る。
	TotalUserUtterances int `json:"total_user_utterances"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Count は学習データになりうる件数を数える。エクスポート前の実態把握に使う。
func (s *Service) Count(ctx context.Context) (*Stats, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("training: db is nil")
	}
	db := s.db.WithContext(ctx)

	stats := &Stats{ByStatus: map[string]int{}}

	var outcomeApps int64
	if err := db.Table("user_application_statuses").
		Where("status IN ?", OutcomeStatuses).
		Count(&outcomeApps).Error; err != nil {
		return nil, fmt.Errorf("training: 応募の集計に失敗しました: %w", err)
	}
	stats.OutcomeApplications = int(outcomeApps)

	type statusRow struct {
		Status string
		N      int
	}
	var rows []statusRow
	if err := db.Table("user_application_statuses AS a").
		Select("a.status AS status, COUNT(DISTINCT s.id) AS n").
		Joins("JOIN interview_sessions AS s ON s.user_id = a.user_id AND s.company_id = a.company_id").
		Where("a.status IN ?", OutcomeStatuses).
		Group("a.status").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("training: ステータス別集計に失敗しました: %w", err)
	}
	for _, r := range rows {
		stats.ByStatus[r.Status] = r.N
		stats.SessionsWithOutcome += r.N
	}

	// ユーザー発話が無いセッションは prompt を作れないので学習対象にならない。
	var exportable int64
	if err := db.Table("interview_sessions AS s").
		Joins("JOIN user_application_statuses AS a ON s.user_id = a.user_id AND s.company_id = a.company_id").
		Joins("JOIN interview_utterances AS u ON u.session_id = s.id AND u.role = ?", "user").
		Where("a.status IN ?", OutcomeStatuses).
		Distinct("s.id").
		Count(&exportable).Error; err != nil {
		return nil, fmt.Errorf("training: 出力可能件数の集計に失敗しました: %w", err)
	}
	stats.ExportableSessions = int(exportable)

	var utterances int64
	if err := db.Table("interview_utterances AS u").
		Joins("JOIN interview_sessions AS s ON s.id = u.session_id").
		Joins("JOIN user_application_statuses AS a ON s.user_id = a.user_id AND s.company_id = a.company_id").
		Where("a.status IN ? AND u.role = ?", OutcomeStatuses, "user").
		Count(&utterances).Error; err != nil {
		return nil, fmt.Errorf("training: 発話数の集計に失敗しました: %w", err)
	}
	stats.TotalUserUtterances = int(utterances)

	return stats, nil
}

// Export は学習対象のセッションを取り出す。
//
// ユーザー発話のみを含める。AI発話を prompt に混ぜると、モデルの出力を
// 教師信号にすることになり蒸留にあたる。
func (s *Service) Export(ctx context.Context, limit int) ([]Session, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("training: db is nil")
	}
	db := s.db.WithContext(ctx)

	type sessionRow struct {
		SessionID uint
		Status    string
	}
	q := db.Table("interview_sessions AS s").
		Select("s.id AS session_id, a.status AS status").
		Joins("JOIN user_application_statuses AS a ON s.user_id = a.user_id AND s.company_id = a.company_id").
		Where("a.status IN ?", OutcomeStatuses).
		Order("s.id")
	if limit > 0 {
		q = q.Limit(limit)
	}

	var rows []sessionRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("training: セッションの取得に失敗しました: %w", err)
	}
	if len(rows) == 0 {
		return []Session{}, nil
	}

	ids := make([]uint, 0, len(rows))
	statusByID := make(map[uint]string, len(rows))
	for _, r := range rows {
		ids = append(ids, r.SessionID)
		statusByID[r.SessionID] = r.Status
	}

	type uttRow struct {
		SessionID uint
		Text      string
	}
	var utts []uttRow
	// created_at は並び順にだけ使う。取り出す必要はない。
	if err := db.Table("interview_utterances").
		Select("session_id, text").
		Where("session_id IN ? AND role = ?", ids, "user").
		Order("session_id, created_at, id").
		Scan(&utts).Error; err != nil {
		return nil, fmt.Errorf("training: 発話の取得に失敗しました: %w", err)
	}

	bySession := make(map[uint][]Utterance, len(ids))
	for _, u := range utts {
		bySession[u.SessionID] = append(bySession[u.SessionID], Utterance{Role: "user", Text: u.Text})
	}

	out := make([]Session, 0, len(ids))
	for _, id := range ids {
		us := bySession[id]
		if len(us) == 0 {
			// 発話が無いと prompt を作れない。RAG 側でも捨てられるのでここで落とす。
			continue
		}
		out = append(out, Session{
			ID:                id,
			ApplicationStatus: statusByID[id],
			Utterances:        us,
		})
	}
	return out, nil
}
