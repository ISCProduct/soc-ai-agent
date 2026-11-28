package growth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"Backend/internal/models"
)

type fakeStore struct {
	scores   []models.UserWeightScore
	progress map[string][]models.UserAnalysisProgress
	latest   string
	inserted *models.UserGrowthSnapshot
	scoreErr error
}

func (f *fakeStore) Scores(context.Context, uint, string) ([]models.UserWeightScore, error) {
	return f.scores, f.scoreErr
}

func (f *fakeStore) Progress(_ context.Context, _ uint, sessionID string) ([]models.UserAnalysisProgress, error) {
	return f.progress[sessionID], nil
}

func (f *fakeStore) LatestProgressSession(context.Context, uint) (string, error) {
	return f.latest, nil
}

func (f *fakeStore) Insert(_ context.Context, row *models.UserGrowthSnapshot) error {
	f.inserted = row
	return nil
}

func TestRecord_SkipsWhenThereAreNoScores(t *testing.T) {
	store := &fakeStore{}
	NewSnapshotter(store).Record(7, SourceChat, "s")
	if store.inserted != nil {
		t.Fatal("スコアが無いのに記録した")
	}
}

func TestRecord_KeepsGoingWhenTheStoreFails(t *testing.T) {
	store := &fakeStore{scoreErr: errors.New("db down")}
	NewSnapshotter(store).Record(7, SourceChat, "s")
	if store.inserted != nil {
		t.Fatal("失敗なのに行を書いた")
	}
}

func TestRecord_AssessmentUsesTheDiagnosisPhase(t *testing.T) {
	store := &fakeStore{
		scores: []models.UserWeightScore{{WeightCategory: "技術志向", Score: 80}},
		latest: "diagnosis-1",
		progress: map[string][]models.UserAnalysisProgress{
			"diagnosis-1": {{
				IsCompleted: true,
				Phase:       &models.AnalysisPhase{PhaseName: "適性分析", DisplayName: "適性"},
			}},
		},
	}
	NewSnapshotter(store).Record(7, SourceAssessment, "assessment:abc")
	if store.inserted == nil {
		t.Fatal("記録されなかった")
	}
	if store.inserted.Source != SourceAssessment || store.inserted.SessionID != "assessment:abc" {
		t.Fatalf("row: %+v", store.inserted)
	}
	var phases []PhaseSnapshot
	if err := json.Unmarshal([]byte(store.inserted.PhaseProgressJSON), &phases); err != nil {
		t.Fatal(err)
	}
	if len(phases) != 1 || phases[0].Name != "適性分析" || !phases[0].IsCompleted {
		t.Fatalf("phases: %+v", phases)
	}
	var scores map[string]float64
	if err := json.Unmarshal([]byte(store.inserted.ScoresJSON), &scores); err != nil {
		t.Fatal(err)
	}
	if scores["技術志向"] != 80 || len(scores) != 1 {
		t.Fatalf("scores: %+v", scores)
	}
}
