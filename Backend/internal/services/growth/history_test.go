package growth

import (
	"testing"
	"time"

	"Backend/internal/models"
)

func snapshot(id uint, at time.Time, scores, phases string) models.UserGrowthSnapshot {
	return models.UserGrowthSnapshot{
		ID:                id,
		UserID:            7,
		Source:            SourceChat,
		SessionID:         "s",
		ScoresJSON:        scores,
		PhaseProgressJSON: phases,
		CreatedAt:         at,
	}
}

func TestBuildHistory_ShowsImprovementAgainstThePreviousSnapshot(t *testing.T) {
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	rows := []models.UserGrowthSnapshot{
		snapshot(2, base.Add(time.Hour), `{"技術志向":70,"チームワーク志向":40}`, `[{"name":"興味分析","display_name":"興味","valid_answers":3,"is_completed":true,"completion_score":80},{"name":"適性分析","display_name":"適性","valid_answers":1,"is_completed":false,"completion_score":20}]`),
		snapshot(1, base, `{"技術志向":55,"安定志向":60}`, `[{"name":"興味分析","display_name":"興味","valid_answers":1,"is_completed":false,"completion_score":10}]`),
	}

	history := BuildHistory(rows)
	if !history.HasPrevious {
		t.Fatal("2件あるので比較できる")
	}
	if history.Points[0].ID != 1 || history.Points[1].ID != 2 {
		t.Fatalf("points are not oldest-first: %+v", history.Points)
	}
	byName := map[string]CategoryChange{}
	for _, change := range history.Categories {
		byName[change.Category] = change
	}
	if byName["技術志向"].ChangeLabel != ChangeImproved || byName["技術志向"].Delta == nil || *byName["技術志向"].Delta != 15 {
		t.Fatalf("技術志向: %+v", byName["技術志向"])
	}
	if byName["チームワーク志向"].ChangeLabel != ChangeNew {
		t.Fatalf("新しく出た軸: %+v", byName["チームワーク志向"])
	}
	if byName["安定志向"].ChangeLabel != ChangeMissing {
		t.Fatalf("消えた軸: %+v", byName["安定志向"])
	}
	if _, ok := byName["リーダーシップ志向"]; ok {
		t.Fatal("どちらの記録にも無い軸は出さない")
	}
	if history.Phase.ChangeLabel != PhaseAdvanced {
		t.Fatalf("phase: %+v", history.Phase)
	}
	if len(history.Phase.NewlyCompleted) != 1 || history.Phase.NewlyCompleted[0] != "興味" {
		t.Fatalf("newly completed: %+v", history.Phase.NewlyCompleted)
	}
}

func TestBuildHistory_OneSnapshotCannotBeCompared(t *testing.T) {
	history := BuildHistory([]models.UserGrowthSnapshot{
		snapshot(1, time.Now(), `{"技術志向":50}`, `[]`),
	})
	if history.HasPrevious || len(history.Categories) != 0 {
		t.Fatalf("1件では比較しない: %+v", history)
	}
	if history.Phase.ChangeLabel != PhaseIncomparable {
		t.Fatalf("phase label: %s", history.Phase.ChangeLabel)
	}
}

func TestBuildHistory_DeclineAndUnchanged(t *testing.T) {
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	history := BuildHistory([]models.UserGrowthSnapshot{
		snapshot(1, base, `{"技術志向":80,"チームワーク志向":40}`, `[{"name":"a","display_name":"A","is_completed":true}]`),
		snapshot(2, base.Add(time.Minute), `{"技術志向":70,"チームワーク志向":40}`, `[{"name":"a","display_name":"A","is_completed":true}]`),
	})
	byName := map[string]CategoryChange{}
	for _, change := range history.Categories {
		byName[change.Category] = change
	}
	if byName["技術志向"].ChangeLabel != ChangeDeclined || *byName["技術志向"].Delta != -10 {
		t.Fatalf("落ちた: %+v", byName["技術志向"])
	}
	if byName["チームワーク志向"].ChangeLabel != ChangeUnchanged {
		t.Fatalf("変化なし: %+v", byName["チームワーク志向"])
	}
	if history.Phase.ChangeLabel != PhaseUnchanged {
		t.Fatalf("phase: %+v", history.Phase)
	}
}

func TestScoresFromRows_DoesNotFillMissingCategories(t *testing.T) {
	scores := ScoresFromRows([]models.UserWeightScore{{WeightCategory: "技術志向", Score: 63}})
	if len(scores) != 1 || scores["技術志向"] != 63 {
		t.Fatalf("scores: %+v", scores)
	}
	if _, ok := scores["安定志向"]; ok {
		t.Fatal("無いカテゴリを埋めてはいけない")
	}
}

func TestBuildHistory_DecimalScoreDeltaHasNoFloatNoise(t *testing.T) {
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	rows := []models.UserGrowthSnapshot{
		snapshot(2, base.Add(time.Hour), `{"技術志向":72.3}`, `[]`),
		snapshot(1, base, `{"技術志向":70.1}`, `[]`),
	}

	history := BuildHistory(rows)
	var got *float64
	for _, change := range history.Categories {
		if change.Category == "技術志向" {
			got = change.Delta
		}
	}
	if got == nil || *got != 2.2 {
		t.Fatalf("delta should be 2.2 without float noise: %v", got)
	}
}
