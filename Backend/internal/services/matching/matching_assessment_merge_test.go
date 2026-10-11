package matching

import (
	"context"
	"errors"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/models"
)

type stubAssessmentScores struct {
	scores map[string]float64
	err    error
}

func (s stubAssessmentScores) LatestStandardScores(uint) (map[string]float64, error) {
	return s.scores, s.err
}

// 合流（#1678）: チャットと検査を 0.5:0.5、片側だけのカテゴリは片側の値、フラグで切れる。
func TestCalculateMatching_MergesAssessmentScores(t *testing.T) {
	profile := &models.CompanyWeightProfile{CompanyID: 1, TechnicalOrientation: 100, StabilityOrientation: 100, GrowthOrientation: 100}
	chat := []entity.UserWeightScore{{WeightCategory: "技術志向", Score: 40}, {WeightCategory: "安定志向", Score: 60}}

	tests := []struct {
		name     string
		env      string
		reader   AssessmentScoreReader
		wantTech float64
		wantStab float64
		wantGrow *float64
		wantAxes int
	}{
		// 技術: (40+80)/2=60 → 100-|60-100|=60 / 安定: チャットのみ60 → 60 / 成長: 検査のみ90 → 90
		{"合流する", "", stubAssessmentScores{scores: map[string]float64{"技術志向": 80, "成長志向": 90}}, 60, 60, ptr(90), 3},
		{"フラグで切るとチャットだけ", "false", stubAssessmentScores{scores: map[string]float64{"技術志向": 80, "成長志向": 90}}, 40, 60, nil, 2},
		{"未受検ならチャットだけ", "", stubAssessmentScores{}, 40, 60, nil, 2},
		{"読み出し失敗でもチャットだけで計算する", "", stubAssessmentScores{err: errors.New("db down")}, 40, 60, nil, 2},
		{"未注入ならチャットだけ", "", nil, 40, 60, nil, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASSESSMENT_SCORE_MERGE", tt.env)
			matchRepo := &matchingMatchRepo{}
			svc := NewMatchingService(&matchingScoreRepo{scores: chat}, &matchingCompanyRepo{
				companies: []models.Company{{ID: 1, Name: "A社"}},
				profiles:  map[uint]*models.CompanyWeightProfile{1: profile},
			}, matchRepo, nil)
			if tt.reader != nil {
				svc.SetAssessmentScoreReader(tt.reader)
			}
			if err := svc.CalculateMatching(context.Background(), 1, "sess-chat"); err != nil {
				t.Fatalf("CalculateMatching: %v", err)
			}
			m := matchRepo.savedMatches[0]
			if m.TechnicalMatch != tt.wantTech || m.StabilityMatch != tt.wantStab {
				t.Fatalf("tech=%v stab=%v, want %v %v", m.TechnicalMatch, m.StabilityMatch, tt.wantTech, tt.wantStab)
			}
			if m.MatchedAxisCount != tt.wantAxes {
				t.Fatalf("axes=%d want %d（欠損を0で埋めていないか）", m.MatchedAxisCount, tt.wantAxes)
			}
			if tt.wantGrow != nil && m.GrowthMatch != *tt.wantGrow {
				t.Fatalf("growth=%v want %v", m.GrowthMatch, *tt.wantGrow)
			}
			if m.SessionID != "sess-chat" {
				t.Fatalf("マッチはチャット診断のセッションに付ける: %q", m.SessionID)
			}
		})
	}
}

// 理由文の適性もマッチ度と同じ合流後の値にする（チャット40・検査80 → 60 と書く）。
func TestReasonScores_UsesMergedScores(t *testing.T) {
	chat := []entity.UserWeightScore{{SessionID: "sess-chat", WeightCategory: "安定志向", Score: 60}, {SessionID: "sess-chat", WeightCategory: "技術志向", Score: 40}}
	svc := NewMatchingService(nil, nil, nil, nil)
	svc.SetAssessmentScoreReader(stubAssessmentScores{scores: map[string]float64{"技術志向": 80, "成長志向": 90}})

	t.Setenv("ASSESSMENT_SCORE_MERGE", "")
	got := svc.ReasonScores(1, chat)
	want := []entity.UserWeightScore{
		{UserID: 1, SessionID: "sess-chat", WeightCategory: "技術志向", Score: 60},
		{UserID: 1, SessionID: "sess-chat", WeightCategory: "安定志向", Score: 60},
		{UserID: 1, SessionID: "sess-chat", WeightCategory: "成長志向", Score: 90},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d] got %+v want %+v", i, got[i], want[i])
		}
	}

	t.Setenv("ASSESSMENT_SCORE_MERGE", "false")
	if got := svc.ReasonScores(1, chat); len(got) != 2 || got[1].Score != 40 {
		t.Fatalf("フラグで切ったらチャットのまま: %+v", got)
	}
}

func ptr(v float64) *float64 { return &v }
