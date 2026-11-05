package interview

import (
	"context"
	"testing"

	"Backend/internal/models"
)

// fakeQuestionStateRepo は質問キューをメモリで持つ。
type fakeQuestionStateRepo struct {
	states  []models.InterviewQuestionState
	latest  *models.InterviewQuestionState
	created []*models.InterviewQuestionState
}

func (r *fakeQuestionStateRepo) CountBySessionID(uint) (int64, error) {
	return int64(len(r.states)), nil
}
func (r *fakeQuestionStateRepo) CreateBatch(s []models.InterviewQuestionState) error {
	r.states = append(r.states, s...)
	return nil
}
func (r *fakeQuestionStateRepo) Create(s *models.InterviewQuestionState) error {
	r.created = append(r.created, s)
	return nil
}
func (r *fakeQuestionStateRepo) Update(*models.InterviewQuestionState) error { return nil }
func (r *fakeQuestionStateRepo) FindBySessionID(uint) ([]models.InterviewQuestionState, error) {
	return r.states, nil
}
func (r *fakeQuestionStateRepo) FindLatestAsked(uint) (*models.InterviewQuestionState, error) {
	return r.latest, nil
}

// #1092: 質問キューの次の1問に、元のカスタム質問の ID（評価したい観点の引き先）が載る。
// ここが壊れると、観点がプロンプトから静かに消える。
func TestAdvanceQuestionPlan_NextCarriesCompanyQuestionID(t *testing.T) {
	qid := uint(7)
	repo := &fakeQuestionStateRepo{states: []models.InterviewQuestionState{{
		ID: 1, SessionID: 1, CompanyQuestionID: &qid, Source: questionSourceCustom,
		Category: "志望動機", QuestionText: "志望理由を教えてください", Status: questionStatusPending,
	}}}
	s := &InterviewService{questionStateRepo: repo}

	d, err := s.advanceQuestionPlan(context.Background(), &models.InterviewSession{ID: 1}, 0, "", "", "", true)
	if err != nil || d == nil {
		t.Fatalf("advanceQuestionPlan = %+v, %v", d, err)
	}
	if d.CompanyQuestionID == nil || *d.CompanyQuestionID != qid {
		t.Fatalf("次の1問の CompanyQuestionID = %v, want %d", d.CompanyQuestionID, qid)
	}
}

// 深掘りの追質問にも、元のカスタム質問の ID を引き継ぐ（返す1問と、保存する状態行の両方）。
// 状態行に無いと、深掘りを2段以上にしたとき2段目で観点が落ちる。
func TestAdvanceQuestionPlan_FollowUpCarriesCompanyQuestionID(t *testing.T) {
	qid := uint(7)
	asked := &models.InterviewQuestionState{
		ID: 5, SessionID: 1, CompanyQuestionID: &qid, Source: questionSourceCustom,
		Category: "志望動機", QuestionText: "志望理由を教えてください", Status: questionStatusAsked,
	}
	repo := &fakeQuestionStateRepo{states: []models.InterviewQuestionState{*asked}, latest: asked}
	s := &InterviewService{questionStateRepo: repo}

	// 短い回答は深掘りの対象になる。openai が無いので追質問は定型文になる。
	d, err := s.advanceQuestionPlan(context.Background(), &models.InterviewSession{ID: 1}, 0, "", "", "はい", false)
	if err != nil || d == nil || !d.IsDeepening {
		t.Fatalf("深掘りの追質問にならない: %+v, %v", d, err)
	}
	if d.CompanyQuestionID == nil || *d.CompanyQuestionID != qid {
		t.Errorf("追質問の CompanyQuestionID = %v, want %d", d.CompanyQuestionID, qid)
	}
	if len(repo.created) != 1 || repo.created[0].CompanyQuestionID == nil || *repo.created[0].CompanyQuestionID != qid {
		t.Errorf("保存する追質問の状態行に CompanyQuestionID が無い: %+v", repo.created)
	}
}
