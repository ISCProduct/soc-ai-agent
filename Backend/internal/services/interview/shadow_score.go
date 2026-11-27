package interview

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/interview/shadowscorer"
)

// shadowScoreStore はシャドー採点の保存と参照。学生向けの経路からは呼ばない。
type shadowScoreStore interface {
	Upsert(ctx context.Context, row *models.InterviewShadowScore) error
	FindBySessionID(ctx context.Context, sessionID uint) (*models.InterviewShadowScore, error)
}

// ShadowComparison は管理者向けの、LLM 採点とローカル採点の並び。
type ShadowComparison struct {
	SessionID    uint                              `json:"session_id"`
	ModelVersion string                            `json:"model_version"`
	LatencyMs    int                               `json:"latency_ms"`
	Shadow       map[string]shadowscorer.AxisScore `json:"shadow"`
	LLM          map[string]int                    `json:"llm_scores"`
}

// SetShadowStore は保存先を入れる。採点を止めたあとも、保存済みの結果はここから読む。
func (s *InterviewService) SetShadowStore(store shadowScoreStore) {
	if s == nil {
		return
	}
	s.shadowStore = store
}

// SetShadowMask は学習書き出しと同じ伏字を入れる。未設定なら採点しない。
func (s *InterviewService) SetShadowMask(load func(ctx context.Context, userID uint) (func(string) string, error)) {
	if s == nil {
		return
	}
	s.shadowMaskFor = load
}

// SetShadowScorer はフラグがオンのときだけ呼ぶ。未設定なら採点しない。
func (s *InterviewService) SetShadowScorer(scorer shadowscorer.InterviewScorer, store shadowScoreStore) {
	s.shadowScorer = scorer
	s.shadowStore = store
}

// recordShadowScore はレポート保存の後に走る。失敗してもレポート作成は成功のままにする。
// scores_json や user_weight_scores はここでは触らない。
func (s *InterviewService) recordShadowScore(ctx context.Context, userID, sessionID uint, utterances []models.InterviewUtterance) {
	if s == nil || s.shadowScorer == nil || s.shadowStore == nil || s.shadowMaskFor == nil {
		return
	}
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[Interview] shadow score panic session=%d: %v", sessionID, rec)
		}
	}()
	mask, err := s.shadowMaskFor(ctx, userID)
	if err != nil || mask == nil {
		log.Printf("[Interview] shadow score mask failed session=%d: %v", sessionID, err)
		return
	}
	turns := shadowTurns(utterances, mask)
	if len(turns) == 0 {
		return
	}
	start := time.Now()
	out, err := s.shadowScorer.Score(ctx, shadowscorer.ScoreInput{Turns: turns})
	if err != nil {
		log.Printf("[Interview] shadow score failed session=%d: %v", sessionID, err)
		return
	}
	body, err := json.Marshal(out.Scores)
	if err != nil {
		log.Printf("[Interview] shadow score encode failed session=%d: %v", sessionID, err)
		return
	}
	row := &models.InterviewShadowScore{
		SessionID:    sessionID,
		ModelVersion: out.ModelVersion,
		ScoresJSON:   string(body),
		LatencyMs:    int(time.Since(start).Milliseconds()),
	}
	if err := s.shadowStore.Upsert(ctx, row); err != nil {
		log.Printf("[Interview] shadow score save failed session=%d: %v", sessionID, err)
	}
}

// ShadowComparison は保存済みのシャドー採点を返す。無ければ ErrNotFound。
func (s *InterviewService) ShadowComparison(ctx context.Context, sessionID uint) (ShadowComparison, error) {
	if s == nil || s.shadowStore == nil {
		return ShadowComparison{}, repository.ErrNotFound
	}
	row, err := s.shadowStore.FindBySessionID(ctx, sessionID)
	if err != nil {
		return ShadowComparison{}, err
	}
	view := ShadowComparison{
		SessionID:    row.SessionID,
		ModelVersion: row.ModelVersion,
		LatencyMs:    row.LatencyMs,
		Shadow:       map[string]shadowscorer.AxisScore{},
		LLM:          map[string]int{},
	}
	if err := json.Unmarshal([]byte(row.ScoresJSON), &view.Shadow); err != nil {
		return ShadowComparison{}, err
	}
	if s.reportRepo != nil {
		report, err := s.reportRepo.FindBySessionID(sessionID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return ShadowComparison{}, err
		}
		if report != nil && strings.TrimSpace(report.ScoresJSON) != "" {
			if err := json.Unmarshal([]byte(report.ScoresJSON), &view.LLM); err != nil {
				return ShadowComparison{}, err
			}
		}
	}
	return view, nil
}

func shadowTurns(utterances []models.InterviewUtterance, mask func(string) string) []shadowscorer.Turn {
	var turns []shadowscorer.Turn
	var q []string
	answering := false
	for _, u := range utterances {
		text := strings.TrimSpace(mask(u.Text))
		if text == "" {
			continue
		}
		switch u.Role {
		case "ai":
			if answering {
				q, answering = nil, false
			}
			q = append(q, text)
		case "user":
			if answering {
				turns[len(turns)-1].A += "\n" + text
				continue
			}
			turns = append(turns, shadowscorer.Turn{Q: strings.Join(q, "\n"), A: text})
			answering = true
		}
	}
	return turns
}
