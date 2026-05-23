package mocks

import (
	"Backend/internal/models"
	"Backend/internal/services"
	"context"

	"github.com/stretchr/testify/mock"
)

type QuestionServiceMock struct {
	mock.Mock
}

func (m *QuestionServiceMock) GenerateAndSaveQuestions(ctx context.Context, req services.GenerateQuestionsRequest) ([]models.QuestionWeight, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuestionWeight), args.Error(1)
}

func (m *QuestionServiceMock) CreateQuestion(qw *models.QuestionWeight) error {
	return m.Called(qw).Error(0)
}

func (m *QuestionServiceMock) GetQuestionsByCategory(category string) ([]models.QuestionWeight, error) {
	args := m.Called(category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuestionWeight), args.Error(1)
}
