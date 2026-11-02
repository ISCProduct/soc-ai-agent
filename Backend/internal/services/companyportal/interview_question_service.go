package companyportal

import (
	"errors"
	"strings"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

var ErrInterviewQuestionForbidden = errors.New("interview question is not owned by company")

const (
	maxInterviewQuestionTextRunes = 2000
	maxInterviewQuestionPriority  = 999
)

type interviewQuestionRepository interface {
	FindByCompanyID(companyID uint) ([]models.InterviewCompanyQuestion, error)
	FindByIDForCompany(id, companyID uint) (*models.InterviewCompanyQuestion, error)
	Create(q *models.InterviewCompanyQuestion) error
	Update(q *models.InterviewCompanyQuestion) error
	Delete(id uint) error
}

type InterviewQuestionInput struct {
	Position     string
	Category     string
	QuestionText string
	Priority     int
	IsRequired   bool
}

type InterviewQuestionService struct {
	repo interviewQuestionRepository
}

func NewInterviewQuestionService(repo interviewQuestionRepository) *InterviewQuestionService {
	return &InterviewQuestionService{repo: repo}
}

func (s *InterviewQuestionService) List(companyID uint) ([]models.InterviewCompanyQuestion, error) {
	if companyID == 0 {
		return nil, ErrInterviewQuestionForbidden
	}
	return s.repo.FindByCompanyID(companyID)
}

func (s *InterviewQuestionService) Create(companyID uint, input InterviewQuestionInput) (*models.InterviewCompanyQuestion, error) {
	if companyID == 0 {
		return nil, ErrInterviewQuestionForbidden
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	question := &models.InterviewCompanyQuestion{CompanyID: companyID}
	applyInterviewQuestionInput(question, input)
	if err := s.repo.Create(question); err != nil {
		return nil, err
	}
	return question, nil
}

func (s *InterviewQuestionService) Update(id, companyID uint, input InterviewQuestionInput) (*models.InterviewCompanyQuestion, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	question, err := s.ownedQuestion(id, companyID)
	if err != nil {
		return nil, err
	}
	applyInterviewQuestionInput(question, input)
	if err := s.repo.Update(question); err != nil {
		return nil, err
	}
	return question, nil
}

func (s *InterviewQuestionService) Delete(id, companyID uint) error {
	question, err := s.ownedQuestion(id, companyID)
	if err != nil {
		return err
	}
	return s.repo.Delete(question.ID)
}

func (in InterviewQuestionInput) Validate() error {
	if strings.TrimSpace(in.Category) == "" {
		return &shared.ValidationError{Message: "カテゴリは必須です"}
	}
	if len([]rune(strings.TrimSpace(in.Category))) > 100 {
		return &shared.ValidationError{Message: "カテゴリは100文字以内で入力してください"}
	}
	if len([]rune(strings.TrimSpace(in.Position))) > 100 {
		return &shared.ValidationError{Message: "対象職種は100文字以内で入力してください"}
	}
	if strings.TrimSpace(in.QuestionText) == "" {
		return &shared.ValidationError{Message: "質問文は必須です"}
	}
	if len([]rune(strings.TrimSpace(in.QuestionText))) > maxInterviewQuestionTextRunes {
		return &shared.ValidationError{Message: "質問文は2000文字以内で入力してください"}
	}
	if in.Priority < 0 || in.Priority > maxInterviewQuestionPriority {
		return &shared.ValidationError{Message: "優先度は0〜999で入力してください"}
	}
	return nil
}

func (s *InterviewQuestionService) ownedQuestion(id, companyID uint) (*models.InterviewCompanyQuestion, error) {
	if id == 0 || companyID == 0 {
		return nil, ErrInterviewQuestionForbidden
	}
	question, err := s.repo.FindByIDForCompany(id, companyID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrInterviewQuestionForbidden
	}
	return question, nil
}

func applyInterviewQuestionInput(question *models.InterviewCompanyQuestion, input InterviewQuestionInput) {
	question.Position = strings.TrimSpace(input.Position)
	question.Category = strings.TrimSpace(input.Category)
	question.QuestionText = strings.TrimSpace(input.QuestionText)
	question.Priority = input.Priority
	question.IsRequired = input.IsRequired
}
