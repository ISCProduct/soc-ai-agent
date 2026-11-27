package interview

import (
	"context"
	"errors"
	"strings"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/interview/shadowscorer"
)

func identityShadowMask(context.Context, uint) (func(string) string, error) {
	return func(text string) string { return text }, nil
}

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
	svc := &InterviewService{shadowStore: store, shadowMaskFor: identityShadowMask}
	svc.recordShadowScore(context.Background(), 1, 1, []models.InterviewUtterance{{Role: "user", Text: "回答"}})
	if store.called {
		t.Fatal("フラグがオフなのに保存した")
	}
}

func TestRecordShadowScore_ErrorDoesNotSave(t *testing.T) {
	store := &spyShadowStore{}
	svc := &InterviewService{shadowScorer: fixedScorer{err: errors.New("壊れたモデル")}, shadowStore: store, shadowMaskFor: identityShadowMask}
	svc.recordShadowScore(context.Background(), 1, 9, []models.InterviewUtterance{{Role: "user", Text: "回答"}})
	if store.called {
		t.Fatal("採点が失敗したのに保存した")
	}
}

func TestRecordShadowScore_MaskErrorDoesNotSave(t *testing.T) {
	store := &spyShadowStore{}
	svc := &InterviewService{
		shadowScorer: fixedScorer{},
		shadowStore:  store,
		shadowMaskFor: func(context.Context, uint) (func(string) string, error) {
			return nil, errors.New("辞書を作れない")
		},
	}
	svc.recordShadowScore(context.Background(), 1, 9, []models.InterviewUtterance{{Role: "user", Text: "山田です"}})
	if store.called {
		t.Fatal("伏字に失敗したのに保存した")
	}
}

type captureScorer struct {
	turns []shadowscorer.Turn
}

func (c *captureScorer) Score(_ context.Context, in shadowscorer.ScoreInput) (shadowscorer.ScoreOutput, error) {
	c.turns = in.Turns
	return shadowscorer.ScoreOutput{
		ModelVersion: "abc",
		Scores:       map[string]shadowscorer.AxisScore{"logic": {Score: 3, Confidence: 2.5}},
	}, nil
}

func TestRecordShadowScore_MasksBeforeScore(t *testing.T) {
	store := &spyShadowStore{}
	scorer := &captureScorer{}
	svc := &InterviewService{
		shadowScorer: scorer,
		shadowStore:  store,
		shadowMaskFor: func(context.Context, uint) (func(string) string, error) {
			return func(text string) string {
				return strings.ReplaceAll(text, "山田", "<NAME>")
			}, nil
		},
	}
	svc.recordShadowScore(context.Background(), 1, 9, []models.InterviewUtterance{
		{Role: "ai", Text: "山田さん、自己紹介を"},
		{Role: "user", Text: "山田です"},
	})
	if !store.called {
		t.Fatal("保存していない")
	}
	if len(scorer.turns) != 1 || scorer.turns[0].Q != "<NAME>さん、自己紹介を" || scorer.turns[0].A != "<NAME>です" {
		t.Fatalf("turns = %+v", scorer.turns)
	}
}

func TestRecordShadowScore_SavesShadowWithoutReportFields(t *testing.T) {
	store := &spyShadowStore{}
	svc := &InterviewService{shadowScorer: fixedScorer{}, shadowStore: store, shadowMaskFor: identityShadowMask}
	svc.recordShadowScore(context.Background(), 1, 9, []models.InterviewUtterance{
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

type shadowReportRepo struct {
	report *models.InterviewReport
	err    error
}

func (r shadowReportRepo) FindBySessionID(uint) (*models.InterviewReport, error) {
	return r.report, r.err
}
func (r shadowReportRepo) FindBySessionIDs([]uint) ([]models.InterviewReport, error) {
	return nil, nil
}
func (r shadowReportRepo) Upsert(*models.InterviewReport) error { return nil }
func (r shadowReportRepo) ClaimScoresApplication(uint) (bool, error) {
	return false, nil
}
func (r shadowReportRepo) ReleaseScoresApplication(uint) error { return nil }

func TestShadowComparison_ReportReadError(t *testing.T) {
	store := &spyShadowStore{row: &models.InterviewShadowScore{SessionID: 9, ScoresJSON: `{"logic":{"score":3,"confidence":1}}`}}
	svc := &InterviewService{shadowStore: store, reportRepo: shadowReportRepo{err: errors.New("db down")}}
	if _, err := svc.ShadowComparison(context.Background(), 9); err == nil {
		t.Fatal("LLM 採点の取得失敗を空の結果にした")
	}
}

func TestShadowComparison_BadLLMJSON(t *testing.T) {
	store := &spyShadowStore{row: &models.InterviewShadowScore{SessionID: 9, ScoresJSON: `{"logic":{"score":3,"confidence":1}}`}}
	svc := &InterviewService{
		shadowStore: store,
		reportRepo:  shadowReportRepo{report: &models.InterviewReport{ScoresJSON: "{"}},
	}
	if _, err := svc.ShadowComparison(context.Background(), 9); err == nil {
		t.Fatal("壊れた LLM 採点を空の結果にした")
	}
}

func TestShadowComparison_MissingIsNotFound(t *testing.T) {
	svc := &InterviewService{shadowStore: &spyShadowStore{}}
	_, err := svc.ShadowComparison(context.Background(), 1)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
