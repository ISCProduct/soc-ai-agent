package portfolio

import (
	"slices"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/repositories"
)

// #2030: 自己分析ポートフォリオ。

func TestParseSections(t *testing.T) {
	tests := []struct {
		raw  string
		want []string
	}{
		{"", AllSections},
		{"es, interview,es", []string{"es", "interview"}},
		{"unknown", nil},
	}
	for _, tt := range tests {
		if got := ParseSections(tt.raw); !slices.Equal(got, tt.want) {
			t.Errorf("ParseSections(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

type fakeStore struct{ calledUser []uint }

func (f *fakeStore) RecentInterviews(userID uint, limit int) ([]repositories.PortfolioInterview, error) {
	f.calledUser = append(f.calledUser, userID)
	return []repositories.PortfolioInterview{{
		SessionID: 3, SummaryText: "結論から話せている",
		ScoresJSON: `{"logic": 3.6, "communication": 4}`, StrengthsJSON: `["具体例がある"]`, ImprovementsJSON: `壊れた JSON`,
	}}, nil
}
func (f *fakeStore) RecentESReviews(userID uint, limit int) ([]repositories.PortfolioES, error) {
	f.calledUser = append(f.calledUser, userID)
	return nil, nil
}
func (f *fakeStore) RecentAssessments(userID uint, limit int) ([]repositories.PortfolioAssessment, error) {
	f.calledUser = append(f.calledUser, userID)
	return []repositories.PortfolioAssessment{{Title: "適性検査A", CategoryScoresJSON: `{"協調性": 60, "主体性": 80}`}}, nil
}

type fakeScores struct{ calledUser uint }

func (f *fakeScores) FindLatestByUser(userID uint) ([]entity.UserWeightScore, error) {
	f.calledUser = userID
	t := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	return []entity.UserWeightScore{
		{WeightCategory: "安定志向", Score: 40, UpdatedAt: t},
		{WeightCategory: "技術志向", Score: 80, UpdatedAt: t.Add(time.Hour)},
	}, nil
}

func TestBuild(t *testing.T) {
	store, scores := &fakeStore{}, &fakeScores{}
	p, err := NewService(store, scores).Build(7, AllSections)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// どの材料も本人の ID で引く
	if scores.calledUser != 7 || !slices.Equal(store.calledUser, []uint{7, 7, 7}) {
		t.Fatalf("scores=%d store=%v", scores.calledUser, store.calledUser)
	}
	// 診断は高い順、更新日時は最新
	if p.Diagnosis == nil || p.Diagnosis.Scores[0].Category != "技術志向" || !p.Diagnosis.UpdatedAt.Equal(time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("diagnosis = %+v", p.Diagnosis)
	}
	iv := p.Interviews[0]
	if iv.Scores["logic"] != 4 || len(iv.Strengths) != 1 || len(iv.Improvements) != 0 {
		t.Fatalf("interview = %+v（壊れた JSON は空にして全体は止めない）", iv)
	}
	if p.ESReviews == nil || len(p.ESReviews) != 0 {
		t.Fatalf("ES が無ければ空の配列: %v", p.ESReviews)
	}
	if a := p.Assessments[0]; a.Title != "適性検査A" || a.Scores[0].Category != "主体性" {
		t.Fatalf("assessment = %+v", a)
	}
}

func TestBuild_選んだセクションだけ(t *testing.T) {
	store, scores := &fakeStore{}, &fakeScores{}
	p, err := NewService(store, scores).Build(7, []string{SectionES})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if p.Diagnosis != nil || p.Interviews != nil || p.Assessments != nil || p.ESReviews == nil {
		t.Fatalf("選ばなかった内容まで出ている: %+v", p)
	}
	if scores.calledUser != 0 || len(store.calledUser) != 1 {
		t.Fatalf("選ばなかった内容の材料まで引いている")
	}
}
