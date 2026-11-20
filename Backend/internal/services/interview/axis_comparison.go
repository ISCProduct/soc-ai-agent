package interview

import (
	"Backend/domain/repository"
	"encoding/json"
	"errors"
	"math"
)

// 面接スコアは 0〜5。高いほど良い、という向きは interview_report の採点と同じ。
const axisUnchangedEpsilon = 1e-6

// ErrInvalidComparison は同じセッション同士、または ID が空の比較要求。
var ErrInvalidComparison = errors.New("interview: invalid axis comparison")

// ErrReportNotReady は比較に必要なレポートが無い、またはスコアが読めない。
var ErrReportNotReady = errors.New("interview: report not yet available")

type interviewAxis struct {
	key   string
	label string
}

var interviewAxisOrder = []interviewAxis{
	{key: "logic", label: "論理性"},
	{key: "specificity", label: "具体性"},
	{key: "ownership", label: "主体性"},
	{key: "communication", label: "コミュニケーション"},
	{key: "enthusiasm", label: "熱意"},
}

// AxisScoreDelta は1つの評価軸について、比較元から比較先への差分。
type AxisScoreDelta struct {
	Axis        string   `json:"axis"`
	Label       string   `json:"label"`
	Base        *float64 `json:"base"`
	Target      *float64 `json:"target"`
	Delta       *float64 `json:"delta"`
	Direction   string   `json:"direction"`
	ChangeLabel string   `json:"change_label"`
}

// AxisComparison は2回の面接の軸別差分。
type AxisComparison struct {
	BaseSessionID   uint             `json:"base_session_id"`
	TargetSessionID uint             `json:"target_session_id"`
	Axes            []AxisScoreDelta `json:"axes"`
}

// AxisDeltas は比較先のスコアから比較元を引いた差分を、固定の軸順で返す。
// どちらかの軸が無いときは差分を出さない。
func AxisDeltas(base, target map[string]float64) []AxisScoreDelta {
	out := make([]AxisScoreDelta, 0, len(interviewAxisOrder))
	for _, axis := range interviewAxisOrder {
		item := AxisScoreDelta{
			Axis:        axis.key,
			Label:       axis.label,
			Direction:   "unavailable",
			ChangeLabel: "比較できない",
		}
		if base != nil {
			if v, ok := base[axis.key]; ok {
				copied := v
				item.Base = &copied
			}
		}
		if target != nil {
			if v, ok := target[axis.key]; ok {
				copied := v
				item.Target = &copied
			}
		}
		if item.Base != nil && item.Target != nil {
			delta := *item.Target - *item.Base
			item.Delta = &delta
			switch {
			case delta > axisUnchangedEpsilon:
				item.Direction = "improved"
				item.ChangeLabel = "伸びた"
			case delta < -axisUnchangedEpsilon:
				item.Direction = "declined"
				item.ChangeLabel = "落ちた"
			default:
				item.Direction = "unchanged"
				item.ChangeLabel = "変化なし"
			}
		}
		out = append(out, item)
	}
	return out
}

func parseAxisScores(raw string) (map[string]float64, error) {
	var scores map[string]float64
	if err := json.Unmarshal([]byte(raw), &scores); err != nil {
		return nil, err
	}
	return scores, nil
}

// CompareAxes は本人（または管理者）の2セッションについて、軸ごとの差分を返す。
// 差分は比較先 − 比較元。正ならその軸は伸びている。
func (s *InterviewService) CompareAxes(userID, baseSessionID, targetSessionID uint) (*AxisComparison, error) {
	if baseSessionID == 0 || targetSessionID == 0 || baseSessionID == targetSessionID {
		return nil, ErrInvalidComparison
	}
	if err := s.EnsureSessionOwnership(userID, baseSessionID); err != nil {
		return nil, err
	}
	if err := s.EnsureSessionOwnership(userID, targetSessionID); err != nil {
		return nil, err
	}
	baseScores, err := s.axisScores(baseSessionID)
	if err != nil {
		return nil, err
	}
	targetScores, err := s.axisScores(targetSessionID)
	if err != nil {
		return nil, err
	}
	return &AxisComparison{
		BaseSessionID:   baseSessionID,
		TargetSessionID: targetSessionID,
		Axes:            AxisDeltas(baseScores, targetScores),
	}, nil
}

func (s *InterviewService) axisScores(sessionID uint) (map[string]float64, error) {
	if s.reportRepo == nil {
		return nil, ErrReportNotReady
	}
	report, err := s.reportRepo.FindBySessionID(sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrReportNotReady
		}
		return nil, err
	}
	if report == nil || report.ScoresJSON == "" {
		return nil, ErrReportNotReady
	}
	scores, err := parseAxisScores(report.ScoresJSON)
	if err != nil || len(scores) == 0 {
		return nil, ErrReportNotReady
	}
	for k, v := range scores {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			delete(scores, k)
		}
	}
	return scores, nil
}
