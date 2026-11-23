package interview

import (
	"context"
	"errors"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/interview/shadowscorer"
)

type spyShadowStore struct {
	row    *models.InterviewShadowScore
	called bool
}

func (s *spyShadowStore) Upsert(_ context.Context, row *models.InterviewShadowScore) error {
	s.called = true
	s.row = row
	return nil
}

func (s *spyShadowStore) FindBySessionID(context.Context, uint) (*models.InterviewShadowScore, error) {
	if s.row == nil {
		return nil, repository.ErrNotFound
	}
	return s.row, nil
}

type fixedScorer struct {
	err error
}

func (f fixedScorer) Score(context.Context, shadowscorer.ScoreInput) (shadowscorer.ScoreOutput, error) {
	if f.err != nil {
		return shadowscorer.ScoreOutput{}, f.err
	}
	return shadowscorer.ScoreOutput{
		ModelVersion: "abc",
		Scores:       map[string]shadowscorer.AxisScore{"logic": {Score: 3, Confidence: 2.5}},
	}, nil
}

func TestRecordShadowScore_OffDoesNothing(t *testing.T) {
	store := &spyShadowStore{}
	svc := &InterviewService{shadowStore: store}
	svc.recordShadowScore(context.Background(), 1, []models.InterviewUtterance{{Role: "user", Text: "回答"}})
	if store.called {
		t.Fatal("フラグがオフなのに保存した")
	}
}

func TestRecordShadowScore_ErrorDoesNotSave(t *testing.T) {
	store := &spyShadowStore{}
	svc := &InterviewService{shadowScorer: fixedScorer{err: errors.New("壊れたモデル")}, shadowStore: store}
	svc.recordShadowScore(context.Background(), 9, []models.InterviewUtterance{{Role: "user", Text: "回答"}})
	if store.called {
		t.Fatal("採点が失敗したのに保存した")
	}
}

func TestRecordShadowScore_SavesShadowWithoutReportFields(t *testing.T) {
	store := &spyShadowStore{}
	svc := &InterviewService{shadowScorer: fixedScorer{}, shadowStore: store}
	svc.recordShadowScore(context.Background(), 9, []models.InterviewUtterance{
		{Role: "ai", Text: "質問"},
		{Role: "user", Text: "回答"},
	})
	if !store.called || store.row == nil {
		t.Fatal("保存していない")
	}
	if store.row.SessionID != 9 || store.row.ModelVersion != "abc" {
		t.Fatalf("row = %+v", store.row)
	}
	if store.row.ScoresJSON == "" {
		t.Fatal("スコアが空")
	}
}

func TestShadowComparison_MissingIsNotFound(t *testing.T) {
	svc := &InterviewService{shadowStore: &spyShadowStore{}}
	_, err := svc.ShadowComparison(context.Background(), 1)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
