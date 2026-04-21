package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type QuestionGeneratorService struct {
	aiClient           *openai.Client
	questionWeightRepo repository.QuestionWeightRepository
}

func NewQuestionGeneratorService(
	aiClient *openai.Client,
	questionWeightRepo repository.QuestionWeightRepository,
) *QuestionGeneratorService {
	return &QuestionGeneratorService{
		aiClient:           aiClient,
		questionWeightRepo: questionWeightRepo,
	}
}

// GenerateQuestionsRequest 質問生成リクエスト
type GenerateQuestionsRequest struct {
	Category      string `json:"category"`        // 重みカテゴリ（例: "技術志向"）
	Count         int    `json:"count"`           // 生成する質問数
	IndustryID    uint   `json:"industry_id"`     // 業界ID（オプション）
	JobCategoryID uint   `json:"job_category_id"` // 職種ID（オプション）
}

// GeneratedQuestion AIが生成した質問
type GeneratedQuestion struct {
	Question    string `json:"question"`
	WeightValue int    `json:"weight_value"`
	Description string `json:"description"`
}

// GenerateAndSaveQuestions AIで質問を生成してDBに保存
func (s *QuestionGeneratorService) GenerateAndSaveQuestions(ctx context.Context, req GenerateQuestionsRequest) ([]models.QuestionWeight, error) {
	// AIに質問生成を依頼
	prompt := fmt.Sprintf(`就活適性診断のための質問を%d個生成してください。

カテゴリ: %s
条件:
- 各質問は応募者の「%s」に関する適性を判定できる内容
- 質問は自然な会話形式
- 重み係数は1-10の範囲で設定
- 各質問に簡単な説明を付ける

以下のJSON形式で返してください:
[
  {
    "question": "質問文",
    "weight_value": 7,
    "description": "この質問の意図"
  },
  ...
]`, req.Count, req.Category, req.Category)

	response, err := s.aiClient.Responses(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate questions: %w", err)
	}

	// JSONパース
	var generatedQuestions []GeneratedQuestion
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("invalid JSON response from AI")
	}
	jsonStr := response[jsonStart : jsonEnd+1]

	if err := json.Unmarshal([]byte(jsonStr), &generatedQuestions); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	// DBに保存（重複チェック付き）
	var savedQuestions []models.QuestionWeight
	for _, gq := range generatedQuestions {
		qw := &models.QuestionWeight{
			Question:       gq.Question,
			WeightCategory: req.Category,
			WeightValue:    gq.WeightValue,
			IndustryID:     req.IndustryID,
			JobCategoryID:  req.JobCategoryID,
			Description:    gq.Description,
			IsActive:       true,
		}

		err := s.questionWeightRepo.Create(qw)
		if err != nil {
			// 重複エラーの場合はスキップ
			if strings.Contains(err.Error(), "既に存在") {
				log.Printf("Question already exists, skipping: %s\n", gq.Question)
				continue
			}
			return nil, fmt.Errorf("failed to save question: %w", err)
		}

		savedQuestions = append(savedQuestions, *qw)
	}

	return savedQuestions, nil
}

// CreateQuestion 手動で質問を登録
func (s *QuestionGeneratorService) CreateQuestion(qw *models.QuestionWeight) error {
	return s.questionWeightRepo.Create(qw)
}

// GetQuestionsByCategory カテゴリ別に質問を取得
func (s *QuestionGeneratorService) GetQuestionsByCategory(category string) ([]models.QuestionWeight, error) {
	return s.questionWeightRepo.FindActiveByCategory(category)
}
