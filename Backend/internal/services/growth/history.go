package growth

import (
	"cmp"
	"encoding/json"
	"math"
	"slices"
	"time"

	"Backend/domain/valueobject"
	"Backend/internal/models"
)

const (
	SourceChat       = "chat"
	SourceInterview  = "interview"
	SourceResume     = "resume"
	SourceES         = "es"
	SourceAssessment = "assessment"

	ChangeImproved    = "伸びた"
	ChangeDeclined    = "落ちた"
	ChangeUnchanged   = "変化なし"
	ChangeNew         = "新しく出た"
	ChangeMissing     = "今回は出ていない"
	PhaseAdvanced     = "進んだ"
	PhaseRegressed    = "戻った"
	PhaseUnchanged    = "変化なし"
	PhaseIncomparable = "比較できない"
)

// Recorder はスコアを書いたあとに、その時点の写しを1行残す。
// nil のときは何もしない。失敗してもスコアの保存は成功のままにする（実装側でログだけ残す）。
type Recorder interface {
	Record(userID uint, source, sessionID string)
}

// PhaseSnapshot は記録時点の診断フェーズ1件。
type PhaseSnapshot struct {
	Name            string  `json:"name"`
	DisplayName     string  `json:"display_name"`
	ValidAnswers    int     `json:"valid_answers"`
	IsCompleted     bool    `json:"is_completed"`
	CompletionScore float64 `json:"completion_score"`
}

// ScoresFromRows はそのセッションに実際にあるカテゴリだけを返す。無い軸は入れない。
func ScoresFromRows(rows []models.UserWeightScore) map[string]float64 {
	out := make(map[string]float64, len(rows))
	for _, row := range rows {
		if row.WeightCategory == "" {
			continue
		}
		out[row.WeightCategory] = row.Score
	}
	return out
}

// PhasesFromProgress は診断フェーズの進みを、画面に出せる形へ写す。
func PhasesFromProgress(rows []models.UserAnalysisProgress) []PhaseSnapshot {
	out := make([]PhaseSnapshot, 0, len(rows))
	for _, row := range rows {
		name := ""
		display := ""
		if row.Phase != nil {
			name = row.Phase.PhaseName
			display = row.Phase.DisplayName
		}
		if name == "" {
			name = "フェーズ"
		}
		if display == "" {
			display = name
		}
		out = append(out, PhaseSnapshot{
			Name:            name,
			DisplayName:     display,
			ValidAnswers:    row.ValidAnswers,
			IsCompleted:     row.IsCompleted,
			CompletionScore: row.CompletionScore,
		})
	}
	return out
}

func SourceLabel(source string) string {
	switch source {
	case SourceChat:
		return "チャット診断"
	case SourceInterview:
		return "面接"
	case SourceResume:
		return "職務経歴書"
	case SourceES:
		return "ES添削"
	case SourceAssessment:
		return "適性検査"
	default:
		return source
	}
}

// Point は履歴の1時点。
type Point struct {
	ID          uint               `json:"id"`
	Source      string             `json:"source"`
	SourceLabel string             `json:"source_label"`
	CreatedAt   time.Time          `json:"created_at"`
	Scores      map[string]float64 `json:"scores"`
	Phases      []PhaseSnapshot    `json:"phases"`
}

// CategoryChange は直近の記録と、その1つ前との差。
type CategoryChange struct {
	Category    string   `json:"category"`
	Previous    *float64 `json:"previous"`
	Current     *float64 `json:"current"`
	Delta       *float64 `json:"delta"`
	ChangeLabel string   `json:"change_label"`
}

// PhaseChange は完了した診断フェーズの数の差。
type PhaseChange struct {
	CompletedPrevious int      `json:"completed_previous"`
	CompletedCurrent  int      `json:"completed_current"`
	ChangeLabel       string   `json:"change_label"`
	NewlyCompleted    []string `json:"newly_completed"`
}

// History は学生に見せる成長の記録。Points は古い順。比較は新しい2件。
type History struct {
	Points      []Point          `json:"points"`
	HasPrevious bool             `json:"has_previous"`
	Categories  []CategoryChange `json:"categories"`
	Phase       PhaseChange      `json:"phase"`
}

// BuildHistory は保存済みの写しから、以前と比べて伸びたかを作る。
// 壊れたJSONの行は飛ばす。2件未満なら比較は出さない。
func BuildHistory(rows []models.UserGrowthSnapshot) History {
	points := make([]Point, 0, len(rows))
	for _, row := range rows {
		var scores map[string]float64
		if err := json.Unmarshal([]byte(row.ScoresJSON), &scores); err != nil {
			continue
		}
		var phases []PhaseSnapshot
		if err := json.Unmarshal([]byte(row.PhaseProgressJSON), &phases); err != nil {
			phases = []PhaseSnapshot{}
		}
		if phases == nil {
			phases = []PhaseSnapshot{}
		}
		if scores == nil {
			scores = map[string]float64{}
		}
		points = append(points, Point{
			ID:          row.ID,
			Source:      row.Source,
			SourceLabel: SourceLabel(row.Source),
			CreatedAt:   row.CreatedAt,
			Scores:      scores,
			Phases:      phases,
		})
	}
	slices.SortFunc(points, func(a, b Point) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	history := History{
		Points: points,
		Phase:  PhaseChange{ChangeLabel: PhaseIncomparable, NewlyCompleted: []string{}},
	}
	if len(points) < 2 {
		return history
	}
	history.HasPrevious = true
	prev := points[len(points)-2]
	cur := points[len(points)-1]
	history.Categories = compareCategories(prev.Scores, cur.Scores)
	history.Phase = comparePhases(prev.Phases, cur.Phases)
	return history
}

// scoreDelta は小数2桁（user_weight_scores.score は decimal(5,2)）で差を返す。
// 浮動小数の引き算のまま返すと 2.1999999999999886 のような値が画面に出る。
func scoreDelta(current, previous float64) float64 {
	return math.Round((current-previous)*100) / 100
}

func compareCategories(prev, cur map[string]float64) []CategoryChange {
	seen := map[string]bool{}
	names := make([]string, 0, len(valueobject.AllWeightCategories()))
	for _, category := range valueobject.AllWeightCategories() {
		name := string(category)
		names = append(names, name)
		seen[name] = true
	}
	extras := make([]string, 0)
	for name := range prev {
		if !seen[name] {
			extras = append(extras, name)
			seen[name] = true
		}
	}
	for name := range cur {
		if !seen[name] {
			extras = append(extras, name)
		}
	}
	slices.Sort(extras)
	names = append(names, extras...)

	out := make([]CategoryChange, 0, len(names))
	for _, name := range names {
		previous, hasPrev := prev[name]
		current, hasCur := cur[name]
		if !hasPrev && !hasCur {
			continue
		}
		change := CategoryChange{Category: name}
		if hasPrev {
			value := previous
			change.Previous = &value
		}
		if hasCur {
			value := current
			change.Current = &value
		}
		switch {
		case hasPrev && hasCur && current > previous:
			change.ChangeLabel = ChangeImproved
			delta := scoreDelta(current, previous)
			change.Delta = &delta
		case hasPrev && hasCur && current < previous:
			change.ChangeLabel = ChangeDeclined
			delta := scoreDelta(current, previous)
			change.Delta = &delta
		case hasPrev && hasCur:
			change.ChangeLabel = ChangeUnchanged
			delta := 0.0
			change.Delta = &delta
		case hasCur:
			change.ChangeLabel = ChangeNew
		default:
			change.ChangeLabel = ChangeMissing
		}
		out = append(out, change)
	}
	return out
}

func comparePhases(prev, cur []PhaseSnapshot) PhaseChange {
	if len(prev) == 0 && len(cur) == 0 {
		return PhaseChange{ChangeLabel: PhaseIncomparable, NewlyCompleted: []string{}}
	}
	prevDone := completedSet(prev)
	curDone := completedSet(cur)
	newly := make([]string, 0)
	for _, phase := range cur {
		if !phase.IsCompleted {
			continue
		}
		if _, ok := prevDone[phase.Name]; ok {
			continue
		}
		newly = append(newly, phase.DisplayName)
	}
	label := PhaseUnchanged
	switch {
	case len(curDone) > len(prevDone):
		label = PhaseAdvanced
	case len(curDone) < len(prevDone):
		label = PhaseRegressed
	}
	return PhaseChange{
		CompletedPrevious: len(prevDone),
		CompletedCurrent:  len(curDone),
		ChangeLabel:       label,
		NewlyCompleted:    newly,
	}
}

func completedSet(phases []PhaseSnapshot) map[string]struct{} {
	out := map[string]struct{}{}
	for _, phase := range phases {
		if phase.IsCompleted && phase.Name != "" {
			out[phase.Name] = struct{}{}
		}
	}
	return out
}
