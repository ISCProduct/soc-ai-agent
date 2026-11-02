package companyportal

import (
	"errors"
	"testing"

	"Backend/internal/models"
)

type interviewQuestionRepoStub struct {
	question      *models.InterviewCompanyQuestion
	findCompanyID uint
	findID        uint
	created       *models.InterviewCompanyQuestion
	updated       *models.InterviewCompanyQuestion
	deletedID     uint
	findErr       error
}

func (r *interviewQuestionRepoStub) FindByCompanyID(uint) ([]models.InterviewCompanyQuestion, error) {
	return nil, nil
}

func (r *interviewQuestionRepoStub) FindByIDForCompany(id, companyID uint) (*models.InterviewCompanyQuestion, error) {
	r.findID = id
	r.findCompanyID = companyID
	return r.question, r.findErr
}

func (r *interviewQuestionRepoStub) Create(question *models.InterviewCompanyQuestion) error {
	r.created = question
	return nil
}

func (r *interviewQuestionRepoStub) Update(question *models.InterviewCompanyQuestion) error {
	r.updated = question
	return nil
}

func (r *interviewQuestionRepoStub) Delete(id uint) error {
	r.deletedID = id
	return nil
}

func TestInterviewQuestionServiceCreateTrimsInputAndScopesCompany(t *testing.T) {
	repo := &interviewQuestionRepoStub{}
	service := NewInterviewQuestionService(repo)

	created, err := service.Create(42, InterviewQuestionInput{
		Position:     "  エンジニア  ",
		Category:     "  技術  ",
		QuestionText: "  設計経験を教えてください。  ",
		Priority:     2,
		IsRequired:   true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CompanyID != 42 || created.Position != "エンジニア" || created.Category != "技術" ||
		created.QuestionText != "設計経験を教えてください。" {
		t.Fatalf("created question = %+v", created)
	}
	if repo.created != created {
		t.Fatal("repository did not receive created question")
	}
}

func TestInterviewQuestionServiceUpdateUsesCompanyScopedLookup(t *testing.T) {
	repo := &interviewQuestionRepoStub{
		question: &models.InterviewCompanyQuestion{ID: 8, CompanyID: 42, QuestionText: "旧質問"},
	}
	service := NewInterviewQuestionService(repo)

	updated, err := service.Update(8, 42, InterviewQuestionInput{
		Category:     "技術",
		QuestionText: "新質問",
		Priority:     1,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repo.findID != 8 || repo.findCompanyID != 42 {
		t.Fatalf("scoped lookup = (%d, %d), want (8, 42)", repo.findID, repo.findCompanyID)
	}
	if updated.ID != 8 || updated.CompanyID != 42 || updated.QuestionText != "新質問" {
		t.Fatalf("updated question = %+v", updated)
	}
}

func TestInterviewQuestionServiceDoesNotUpdateQuestionOutsideCompany(t *testing.T) {
	repo := &interviewQuestionRepoStub{}
	service := NewInterviewQuestionService(repo)

	_, err := service.Update(8, 42, InterviewQuestionInput{Category: "技術", QuestionText: "質問"})
	if !errors.Is(err, ErrInterviewQuestionForbidden) {
		t.Fatalf("Update() error = %v, want ownership error", err)
	}
	if repo.updated != nil {
		t.Fatal("question outside company was updated")
	}
}
