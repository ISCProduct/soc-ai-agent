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
	got := service.confirmedResearchQuestions(context.Background(), "架空製作所", "エンジニア", []string{"新しい候補"})
	if len(got) != 1 || got[0] != "保存済み" || store.saved != 0 {
		t.Fatalf("got=%v saved=%d", got, store.saved)
	}
}

// クライアントが送った候補は共有の保存先に書かない（他の利用者の面接に混ざらないようにする）。
func TestConfirmedResearchQuestions_クライアントの候補はセッション限りで保存しない(t *testing.T) {
	for name, store := range map[string]*memResearch{
		"期限切れ": {questions: []string{"古い"}, expires: time.Now().Add(-time.Hour)},
		"未保存":  {},
	} {
		t.Run(name, func(t *testing.T) {
			service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
			service.SetResearchRepo(store)
			got := service.confirmedResearchQuestions(context.Background(), "架空製作所", "エンジニア", []string{"新しい候補"})
			if len(got) != 1 || got[0] != "新しい候補" {
				t.Fatalf("got=%v", got)
			}
			if store.saved != 0 {
				t.Fatalf("クライアントの候補を保存した: saved=%d", store.saved)
			}
		})
	}
}

func TestConfirmedResearchQuestions_読めないときもセッション限りの候補で続ける(t *testing.T) {
	store := &memResearch{err: errors.New("db down")}
	service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	service.SetResearchRepo(store)
	got := service.confirmedResearchQuestions(context.Background(), "架空製作所", "エンジニア", []string{"新しい候補"})
	if len(got) != 1 || got[0] != "新しい候補" || store.saved != 0 {
		t.Fatalf("got=%v saved=%d", got, store.saved)
	}
	if got := service.confirmedResearchQuestions(context.Background(), "架空製作所", "エンジニア", nil); len(got) != 0 {
		t.Fatalf("候補なしのときは空: %v", got)
	}
}

func TestSaveServerResearch_サーバー側の調査結果だけを期限つきで保存する(t *testing.T) {
	store := &memResearch{}
	service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	service.SetResearchRepo(store)
	if err := service.SaveServerResearch(context.Background(), "架空製作所", "エンジニア", []string{"調査した質問"}); err != nil {
		t.Fatal(err)
	}
	if store.saved != 1 || len(store.questions) != 1 || !store.expires.After(time.Now()) {
		t.Fatalf("saved=%d questions=%v expires=%v", store.saved, store.questions, store.expires)
	}
	if err := service.SaveServerResearch(context.Background(), "", "エンジニア", []string{"x"}); err != nil || store.saved != 1 {
		t.Fatalf("企業名が空なら保存しない: err=%v saved=%d", err, store.saved)
	}
}

// 保存先が落ちていても、質問キューは作られる。
func TestAdvanceQuestionPlan_調査結果を読めなくてもキューを作る(t *testing.T) {
	repo := &questionPlanStateRepoStub{}
	service := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	service.SetQuestionStateRepo(repo)
	service.SetResearchRepo(&memResearch{err: errors.New("db down")})
	_, err := service.advanceQuestionPlan(
		WithCompanyQuestionHints(context.Background(), []string{"調査した質問"}),
		&models.InterviewSession{ID: 9},
		0, "", "架空製作所", "", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.states) == 0 || repo.states[0].QuestionText != "調査した質問" {
		t.Fatalf("queue = %+v", repo.states)
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
