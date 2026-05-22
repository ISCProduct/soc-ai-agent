package controllers

import (
	"Backend/internal/models"
	"Backend/internal/services"
	"net/http"

	"github.com/labstack/echo/v4"
)

type QuestionController struct {
	questionService *services.QuestionGeneratorService
}

func NewQuestionController(questionService *services.QuestionGeneratorService) *QuestionController {
	return &QuestionController{questionService: questionService}
}

// GenerateQuestions AIで質問を生成
// POST /api/questions/generate
func (c *QuestionController) GenerateQuestions(ctx echo.Context) error {
	var req services.GenerateQuestionsRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// バリデーション
	if req.Category == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "category is required")
	}
	if req.Count <= 0 {
		req.Count = 5 // デフォルト5個
	}

	questions, err := c.questionService.GenerateAndSaveQuestions(ctx.Request().Context(), req)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"generated_count": len(questions),
		"questions":       questions,
	})
}

// CreateQuestion 手動で質問を登録
// POST /api/questions
func (c *QuestionController) CreateQuestion(ctx echo.Context) error {
	var qw models.QuestionWeight
	if err := ctx.Bind(&qw); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// バリデーション
	if qw.Question == "" || qw.WeightCategory == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "question and weight_category are required")
	}

	if err := c.questionService.CreateQuestion(&qw); err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusCreated, qw)
}

// GetQuestionsByCategory カテゴリ別質問取得
// GET /api/questions?category=xxx
func (c *QuestionController) GetQuestionsByCategory(ctx echo.Context) error {
	category := ctx.QueryParam("category")
	if category == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "category is required")
	}

	questions, err := c.questionService.GetQuestionsByCategory(category)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, questions)
}
