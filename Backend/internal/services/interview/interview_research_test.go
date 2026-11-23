package interview

import (
	"context"
	"errors"
	"testing"
	"time"

	"Backend/internal/models"
)

type memResearch struct {
	questions []string
	expires   time.Time
	saved     int
	err       error
}

func (m *memResearch) FindFresh(_ context.Context, _, _ string, now time.Time) ([]string, bool, error) {
	if m.err != nil {
		return nil, false, m.err
	}
	if len(m.questions) == 0 || !m.expires.After(now) {
		return nil, false, nil
	}
	return append([]string(nil), m.questions...), true, nil
}

func (m *memResearch) Save(_ context.Context, _, _ string, questions []string, _, expiresAt time.Time) error {
	if m.err != nil {
		return m.err
	}
	m.questions = append([]string(nil), questions...)
	m.expires = expiresAt
	m.saved++
	return nil
}

func TestConfirmedResearchQuestions_期限内は保存済みを使い新しい候補では上書きしない(t *testing.T) {
	store := &memResearch{questions: []string{"保存済み"}, expires: time.Now().Add(time.Hour)}
	service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	service.SetResearchRepo(store)
	got, err := service.confirmedResearchQuestions(context.Background(), "架空製作所", "エンジニア", []string{"新しい候補"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "保存済み" || store.saved != 0 {
		t.Fatalf("got=%v saved=%d", got, store.saved)
	}
}

func TestConfirmedResearchQuestions_期限切れは今回の候補を保存する(t *testing.T) {
	store := &memResearch{questions: []string{"古い"}, expires: time.Now().Add(-time.Hour)}
	service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	service.SetResearchRepo(store)
	got, err := service.confirmedResearchQuestions(context.Background(), "架空製作所", "エンジニア", []string{"新しい候補"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "新しい候補" || store.saved != 1 {
		t.Fatalf("got=%v saved=%d", got, store.saved)
	}
}

func TestConfirmedResearchQuestions_読めないときは候補を使わない(t *testing.T) {
	store := &memResearch{err: errors.New("db down")}
	service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	service.SetResearchRepo(store)
	got, err := service.confirmedResearchQuestions(context.Background(), "架空製作所", "エンジニア", []string{"新しい候補"})
	if err == nil || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestAdvanceQuestionPlan_調査質問はai_researchとして積む(t *testing.T) {
	repo := &questionPlanStateRepoStub{}
	service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	service.SetQuestionStateRepo(repo)
	service.SetResearchRepo(&memResearch{})
	_, err := service.advanceQuestionPlan(
		WithCompanyQuestionHints(context.Background(), []string{"調査した質問"}),
		&models.InterviewSession{ID: 9},
		0, "", "架空製作所", "", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.states) == 0 || repo.states[0].Source != questionSourceAIResearch || repo.states[0].QuestionText != "調査した質問" {
		t.Fatalf("queue head = %+v", repo.states[0])
	}
}
