package controllers_test

// QuestionControllerのHTTPハンドラーテスト
//
// 実行: cd Backend && go test ./test/controllers/... -run Question -v

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/controllers"
	"Backend/internal/models"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newQuestionController(svc *mocks.QuestionServiceMock) *controllers.QuestionController {
	return controllers.NewQuestionController(svc)
}

// ---- GenerateQuestions ----

func TestQuestionController_GenerateQuestions_CountDefault(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	req := services.GenerateQuestionsRequest{Category: "自己PR", Count: 5}
	questions := []models.QuestionWeight{{Question: "自己PRをしてください", WeightCategory: "自己PR"}}
	svc.On("GenerateAndSaveQuestions", context.Background(), req).Return(questions, nil)

	body, _ := json.Marshal(map[string]any{"category": "自己PR"}) // count省略 → デフォルト5
	httpReq := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).GenerateQuestions, newCtx(httpReq, rec), http.StatusOK)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, float64(1), resp["generated_count"])
	svc.AssertExpectations(t)
}

func TestQuestionController_GenerateQuestions_ServiceError(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	req := services.GenerateQuestionsRequest{Category: "志望動機", Count: 3}
	svc.On("GenerateAndSaveQuestions", context.Background(), req).Return(nil, errors.New("AI error"))

	body, _ := json.Marshal(map[string]any{"category": "志望動機", "count": 3})
	httpReq := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).GenerateQuestions, newCtx(httpReq, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

// ---- CreateQuestion ----

func TestQuestionController_CreateQuestion_Success(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	qw := &models.QuestionWeight{Question: "自己PRは？", WeightCategory: "自己PR"}
	svc.On("CreateQuestion", qw).Return(nil)

	body, _ := json.Marshal(map[string]any{"question": "自己PRは？", "weight_category": "自己PR"})
	httpReq := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).CreateQuestion, newCtx(httpReq, rec), http.StatusCreated)
	svc.AssertExpectations(t)
}

func TestQuestionController_CreateQuestion_ServiceError(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	qw := &models.QuestionWeight{Question: "志望動機は？", WeightCategory: "志望動機"}
	svc.On("CreateQuestion", qw).Return(errors.New("DB error"))

	body, _ := json.Marshal(map[string]any{"question": "志望動機は？", "weight_category": "志望動機"})
	httpReq := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).CreateQuestion, newCtx(httpReq, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

// ---- GetQuestionsByCategory ----

func TestQuestionController_GetQuestionsByCategory_Success(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	questions := []models.QuestionWeight{
		{Question: "自己PRは？", WeightCategory: "自己PR"},
		{Question: "強みは？", WeightCategory: "自己PR"},
	}
	svc.On("GetQuestionsByCategory", "自己PR").Return(questions, nil)

	httpReq := httptest.NewRequest(http.MethodGet, "/api/questions?category=自己PR", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).GetQuestionsByCategory, newCtx(httpReq, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestQuestionController_GetQuestionsByCategory_ServiceError(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	svc.On("GetQuestionsByCategory", "志望動機").Return(nil, errors.New("DB error"))

	httpReq := httptest.NewRequest(http.MethodGet, "/api/questions?category=志望動機", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).GetQuestionsByCategory, newCtx(httpReq, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

// unused import guard
var _ = assert.New
