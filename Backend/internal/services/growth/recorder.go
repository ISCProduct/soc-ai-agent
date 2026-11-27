package growth

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"Backend/internal/models"
)

// Store は写しを残すために必要な読み書き。
type Store interface {
	Scores(ctx context.Context, userID uint, sessionID string) ([]models.UserWeightScore, error)
	Progress(ctx context.Context, userID uint, sessionID string) ([]models.UserAnalysisProgress, error)
	LatestProgressSession(ctx context.Context, userID uint) (string, error)
	Insert(ctx context.Context, row *models.UserGrowthSnapshot) error
}

// recordTimeout は履歴1行の書き込みに使える時間。呼び出し元のリクエストが終わっても続けたいので、リクエストの context は使わない。
const recordTimeout = 5 * time.Second

// Snapshotter はスコアを書いたあとに、その時点の10カテゴリと診断フェーズを1行残す。
type Snapshotter struct {
	store Store
}

func NewSnapshotter(store Store) *Snapshotter {
	return &Snapshotter{store: store}
}

// Record は失敗しても呼び出し元へ返さない。マッチングに使うスコアの保存を、履歴のために巻き戻さない。
// そのセッションにスコアが1件も無ければ残さない（空の記録は「伸びた」の比較を壊す）。
func (s *Snapshotter) Record(userID uint, source, sessionID string) {
	if s == nil || s.store == nil || userID == 0 || sessionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), recordTimeout)
	defer cancel()
	if err := s.append(ctx, userID, source, sessionID); err != nil {
		log.Printf("[Growth] snapshot failed user=%d source=%s: %v", userID, source, err)
	}
}

func (s *Snapshotter) append(ctx context.Context, userID uint, source, sessionID string) error {
	scores, err := s.store.Scores(ctx, userID, sessionID)
	if err != nil {
		return err
	}
	if len(scores) == 0 {
		return nil
	}
	phases, err := s.phases(ctx, userID, sessionID)
	if err != nil {
		return err
	}
	scoreJSON, err := json.Marshal(ScoresFromRows(scores))
	if err != nil {
		return err
	}
	phaseJSON, err := json.Marshal(PhasesFromProgress(phases))
	if err != nil {
		return err
	}
	return s.store.Insert(ctx, &models.UserGrowthSnapshot{
		UserID:            userID,
		Source:            source,
		SessionID:         sessionID,
		ScoresJSON:        string(scoreJSON),
		PhaseProgressJSON: string(phaseJSON),
	})
}

func (s *Snapshotter) phases(ctx context.Context, userID uint, sessionID string) ([]models.UserAnalysisProgress, error) {
	rows, err := s.store.Progress(ctx, userID, sessionID)
	if err != nil || len(rows) > 0 {
		return rows, err
	}
	// 適性検査のセッションには診断フェーズが無い。空のまま残すと、
	// 診断のあとに検査しただけで「フェーズが戻った」ように見える。
	latest, err := s.store.LatestProgressSession(ctx, userID)
	if err != nil || latest == "" || latest == sessionID {
		return rows, err
	}
	return s.store.Progress(ctx, userID, latest)
}
