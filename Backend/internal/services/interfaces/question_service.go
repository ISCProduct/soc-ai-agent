package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services"
	"context"
)

type QuestionGeneratorService interface {
	GenerateAndSaveQuestions(ctx context.Context, req services.GenerateQuestionsRequest) ([]models.QuestionWeight, error)
	CreateQuestion(qw *models.QuestionWeight) error
	GetQuestionsByCategory(category string) ([]models.QuestionWeight, error)
}
