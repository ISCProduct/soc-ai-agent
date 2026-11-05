package companyportal

import (
	"errors"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

type interviewQuestionRepoStub struct {
	question      *models.InterviewCompanyQuestion
	existing      []models.InterviewCompanyQuestion
	findCompanyID uint
	findID        uint
	created       *models.InterviewCompanyQuestion
	updated       *models.InterviewCompanyQuestion
	deletedID     uint
	findErr       error
}

func (r *interviewQuestionRepoStub) FindByCompanyID(uint) ([]models.InterviewCompanyQuestion, error) {
	return r.existing, nil
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

func TestInterviewQuestionServiceCreateEnforcesCompanyLimit(t *testing.T) {
	input := InterviewQuestionInput{Category: "技術", QuestionText: "質問"}

	tests := []struct {
		name     string
		existing int
		wantErr  bool
	}{
		{name: "上限の1件手前は登録できる", existing: maxInterviewQuestionsPerCompany - 1},
		{name: "上限に達したら登録できない", existing: maxInterviewQuestionsPerCompany, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &interviewQuestionRepoStub{existing: make([]models.InterviewCompanyQuestion, tt.existing)}

			_, err := NewInterviewQuestionService(repo).Create(42, input)

			var validationErr *shared.ValidationError
			if got := errors.As(err, &validationErr); got != tt.wantErr {
				t.Fatalf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (repo.created == nil) != tt.wantErr {
				t.Fatalf("created = %+v, wantErr %v", repo.created, tt.wantErr)
			}
		})
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
