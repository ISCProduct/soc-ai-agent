package interview

import (
	"context"
	"strings"
	"testing"

	"Backend/internal/models"
)

func TestLightweightAnswerScore_採点ではなく深掘り判定だけ(t *testing.T) {
	longAbstract := strings.Repeat("あ", 80)
	if got := LightweightAnswerScore(longAbstract); got != 60 {
		t.Fatalf("抽象的な長文 = %d, want 60", got)
	}
	if got := LightweightAnswerScore("3名でAPIをリリースした"); got < 25 {
		t.Fatalf("具体性のある短文 = %d", got)
	}
	if LightweightAnswerScore("（聞き取れませんでした）") != 0 {
		t.Fatal("聞き取れなかった回答は 0")
	}
}

func TestShouldDeepen_閾値以上は深掘りしない(t *testing.T) {
	answer := strings.Repeat("あ", 80)
	if !NeedsDeepening(answer, 0) {
		t.Fatal("この回答は長さと具体性では深掘り対象")
	}
	threshold := 60
	if shouldDeepen(answer, 0, &threshold) {
		t.Fatal("閾値以上なのに深掘りする")
	}
	higher := 90
	if !shouldDeepen(answer, 0, &higher) {
		t.Fatal("閾値未満なのに深掘りを飛ばした")
	}
	if !shouldDeepen(answer, 0, nil) {
		t.Fatal("閾値が無いときは現行の判定")
	}
}

type skipStateRepo struct {
	states  []models.InterviewQuestionState
	created int
}

func (r *skipStateRepo) CountBySessionID(uint) (int64, error) {
	return int64(len(r.states)), nil
}
func (r *skipStateRepo) CreateBatch(states []models.InterviewQuestionState) error {
	r.states = append(r.states, states...)
	return nil
}
func (r *skipStateRepo) Create(state *models.InterviewQuestionState) error {
	r.created++
	r.states = append(r.states, *state)
	return nil
}
func (r *skipStateRepo) Update(state *models.InterviewQuestionState) error {
	for i := range r.states {
		if r.states[i].ID == state.ID {
			r.states[i] = *state
		}
	}
	return nil
}
func (r *skipStateRepo) FindBySessionID(uint) ([]models.InterviewQuestionState, error) {
	return r.states, nil
}
func (r *skipStateRepo) FindLatestAsked(uint) (*models.InterviewQuestionState, error) {
	for i := range r.states {
		if r.states[i].Status == questionStatusAsked {
			return &r.states[i], nil
		}
	}
	return nil, nil
}

func TestAdvanceQuestionPlan_高評価なら深掘りしない(t *testing.T) {
	answer := strings.Repeat("あ", 80)
	newRepo := func() *skipStateRepo {
		return &skipStateRepo{states: []models.InterviewQuestionState{
			{ID: 1, SessionID: 1, Source: questionSourceTopic, QuestionText: "自己紹介", Status: questionStatusAsked},
			{ID: 2, SessionID: 1, Source: questionSourceTopic, QuestionText: "次の質問", Status: questionStatusPending},
		}}
	}
	session := &models.InterviewSession{ID: 1}

	t.Run("閾値以上は次の質問へ進む", func(t *testing.T) {
		t.Setenv("INTERVIEW_SKIP_SCORE_THRESHOLD", "60")
		repo := newRepo()
		service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
		service.SetQuestionStateRepo(repo)
		directive, err := service.advanceQuestionPlan(context.Background(), session, 0, "", "", answer, false)
		if err != nil {
			t.Fatal(err)
		}
		if repo.created != 0 {
			t.Fatalf("深掘りを作った: %d", repo.created)
		}
		if directive == nil || directive.Text != "次の質問" || directive.IsDeepening {
			t.Fatalf("directive = %+v", directive)
		}
	})

	t.Run("閾値が無ければ深掘りする", func(t *testing.T) {
		t.Setenv("INTERVIEW_SKIP_SCORE_THRESHOLD", "")
		repo := newRepo()
		service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
		service.SetQuestionStateRepo(repo)
		directive, err := service.advanceQuestionPlan(context.Background(), session, 0, "", "", answer, false)
		if err != nil {
			t.Fatal(err)
		}
		if repo.created != 1 || directive == nil || !directive.IsDeepening || directive.Source != questionSourceFollowUp {
			t.Fatalf("created=%d directive=%+v", repo.created, directive)
		}
	})
}
