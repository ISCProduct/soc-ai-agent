package company

import (
	"errors"
	"net/http"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/services/companyportal"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

type interviewQuestionService interface {
	List(companyID uint) ([]models.InterviewCompanyQuestion, error)
	Create(companyID uint, input companyportal.InterviewQuestionInput) (*models.InterviewCompanyQuestion, error)
	Update(id, companyID uint, input companyportal.InterviewQuestionInput) (*models.InterviewCompanyQuestion, error)
	Delete(id, companyID uint) error
}

type CompanyPortalInterviewQuestionController struct {
	questions interviewQuestionService
}

func NewCompanyPortalInterviewQuestionController(questions interviewQuestionService) *CompanyPortalInterviewQuestionController {
	return &CompanyPortalInterviewQuestionController{questions: questions}
}

type interviewQuestionBody struct {
	Position     string `json:"position"`
	Category     string `json:"category"`
	QuestionText string `json:"question_text"`
	Priority     int    `json:"priority"`
	IsRequired   bool   `json:"is_required"`
}

func (b interviewQuestionBody) input() companyportal.InterviewQuestionInput {
	return companyportal.InterviewQuestionInput{
		Position:     b.Position,
		Category:     b.Category,
		QuestionText: b.QuestionText,
		Priority:     b.Priority,
		IsRequired:   b.IsRequired,
	}
}

func (c *CompanyPortalInterviewQuestionController) List(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	questions, err := c.questions.List(companyID)
	if err != nil {
		return mapInterviewQuestionError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"questions": questions})
}

func (c *CompanyPortalInterviewQuestionController) Create(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	var body interviewQuestionBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	question, err := c.questions.Create(companyID, body.input())
	if err != nil {
		return mapInterviewQuestionError(err)
	}
	return ctx.JSON(http.StatusCreated, question)
}

func (c *CompanyPortalInterviewQuestionController) Update(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	id, apiErr := httpapi.UintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}
	var body interviewQuestionBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	question, err := c.questions.Update(id, companyID, body.input())
	if err != nil {
		return mapInterviewQuestionError(err)
	}
	return ctx.JSON(http.StatusOK, question)
}

func (c *CompanyPortalInterviewQuestionController) Delete(ctx echo.Context) error {
	companyID, apiErr := c.requireOwner(ctx)
	if apiErr != nil {
		return apiErr
	}
	id, apiErr := httpapi.UintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}
	if err := c.questions.Delete(id, companyID); err != nil {
		return mapInterviewQuestionError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

func (c *CompanyPortalInterviewQuestionController) requireOwner(ctx echo.Context) (uint, error) {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return 0, httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	if !middleware.CompanyUserIsOwner(ctx.Request().Context()) {
		return 0, httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "面接質問の編集は企業管理者のみ行えます")
	}
	return companyID, nil
}

func mapInterviewQuestionError(err error) error {
	var validationErr *shared.ValidationError
	switch {
	case errors.Is(err, companyportal.ErrInterviewQuestionForbidden):
		return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "この面接質問を操作する権限がありません")
	case errors.As(err, &validationErr):
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, validationErr.Message)
	default:
		return httpapi.NewAPIError(http.StatusInternalServerError, httpapi.ErrCodeInternalError, "面接質問の処理に失敗しました")
	}
}
