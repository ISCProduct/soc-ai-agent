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
	"Backend/domain/repository"
	"context"
	"fmt"
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
	repo repository.TrainingExportRepository
}

func NewService(repo repository.TrainingExportRepository) *Service {
	return &Service{repo: repo}
}

// Count は学習データになりうる件数を数える。エクスポート前の実態把握に使う。
func (s *Service) Count(ctx context.Context) (*Stats, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("training: db is nil")
	}

	stats := &Stats{ByStatus: map[string]int{}}

	outcomeApps, err := s.repo.CountOutcomeApplications(ctx, OutcomeStatuses)
	if err != nil {
		return nil, fmt.Errorf("training: 応募の集計に失敗しました: %w", err)
	}
	stats.OutcomeApplications = outcomeApps

	byStatus, err := s.repo.CountSessionsByStatus(ctx, OutcomeStatuses)
	if err != nil {
		return nil, fmt.Errorf("training: ステータス別集計に失敗しました: %w", err)
	}
	for status, n := range byStatus {
		stats.ByStatus[status] = n
		stats.SessionsWithOutcome += n
	}

	exportable, err := s.repo.CountExportableSessions(ctx, OutcomeStatuses)
	if err != nil {
		return nil, fmt.Errorf("training: 出力可能件数の集計に失敗しました: %w", err)
	}
	stats.ExportableSessions = exportable

	utterances, err := s.repo.CountUserUtterances(ctx, OutcomeStatuses)
	if err != nil {
		return nil, fmt.Errorf("training: 発話数の集計に失敗しました: %w", err)
	}
	stats.TotalUserUtterances = utterances

	return stats, nil
}

// Export は学習対象のセッションを取り出す。
//
// ユーザー発話のみを含める。AI発話を prompt に混ぜると、モデルの出力を
// 教師信号にすることになり蒸留にあたる。
func (s *Service) Export(ctx context.Context, limit int) ([]Session, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("training: db is nil")
	}

	rows, err := s.repo.ListOutcomeSessions(ctx, OutcomeStatuses, limit)
	if err != nil {
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

	utts, err := s.repo.ListUserUtterances(ctx, ids)
	if err != nil {
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
