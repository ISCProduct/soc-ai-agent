package interfaces

import (
	"Backend/internal/models"
	"Backend/internal/services/chat"
	"context"
)

// QuestionGeneratorService 質問生成サービスのインターフェース
type QuestionGeneratorService interface {
	GenerateAndSaveQuestions(ctx context.Context, req chat.GenerateQuestionsRequest) ([]models.QuestionWeight, error)
	CreateQuestion(qw *models.QuestionWeight) error
	GetQuestionsByCategory(category string) ([]models.QuestionWeight, error)
}
