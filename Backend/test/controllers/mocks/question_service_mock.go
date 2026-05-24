package mocks

import (
	"Backend/internal/models"
	"Backend/internal/services"
	"context"

	"github.com/stretchr/testify/mock"
)

// QuestionServiceMock QuestionGeneratorServiceのモック実装
type QuestionServiceMock struct {
	mock.Mock
}

func (m *QuestionServiceMock) GenerateAndSaveQuestions(ctx context.Context, req services.GenerateQuestionsRequest) ([]models.QuestionWeight, error) {
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.([]models.QuestionWeight), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *QuestionServiceMock) CreateQuestion(qw *models.QuestionWeight) error {
	args := m.Called(qw)
	return args.Error(0)
}

func (m *QuestionServiceMock) GetQuestionsByCategory(category string) ([]models.QuestionWeight, error) {
	args := m.Called(category)
	if v := args.Get(0); v != nil {
		return v.([]models.QuestionWeight), args.Error(1)
	}
	return nil, args.Error(1)
}
