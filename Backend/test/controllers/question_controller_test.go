package controllers_test

// QuestionControllerのHTTPハンドラーテスト (Issue #422)
//
// 実行: cd Backend && go test ./test/controllers/... -run Question -v

import (
	"bytes"
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
	"github.com/stretchr/testify/mock"
)

func newQuestionController(svc *mocks.QuestionServiceMock) *controllers.QuestionController {
	return controllers.NewQuestionController(svc)
}

// ---- GenerateQuestions ----

func TestQuestionController_GenerateQuestions_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/questions/generate", nil)
	w := httptest.NewRecorder()
	controllers.NewQuestionController(nil).GenerateQuestions(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestQuestionController_GenerateQuestions_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewQuestionController(nil).GenerateQuestions(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQuestionController_GenerateQuestions_MissingCategory(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{"count": 3})
	req := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	controllers.NewQuestionController(nil).GenerateQuestions(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQuestionController_GenerateQuestions_DefaultCount(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	expected := services.GenerateQuestionsRequest{Category: "technical", Count: 5}
	questions := []models.QuestionWeight{{Question: "test question"}}
	svc.On("GenerateAndSaveQuestions", mock.Anything, expected).Return(questions, nil)

	body, _ := json.Marshal(map[string]string{"category": "technical"})
	req := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newQuestionController(svc).GenerateQuestions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestQuestionController_GenerateQuestions_ServiceError(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	svc.On("GenerateAndSaveQuestions", mock.Anything, mock.Anything).Return(nil, errors.New("openai error"))

	body, _ := json.Marshal(map[string]interface{}{"category": "technical", "count": 3})
	req := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newQuestionController(svc).GenerateQuestions(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// ---- CreateQuestion ----

func TestQuestionController_CreateQuestion_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/questions", nil)
	w := httptest.NewRecorder()
	controllers.NewQuestionController(nil).CreateQuestion(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestQuestionController_CreateQuestion_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	controllers.NewQuestionController(nil).CreateQuestion(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQuestionController_CreateQuestion_MissingFields(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"question": "test"})
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	controllers.NewQuestionController(nil).CreateQuestion(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQuestionController_CreateQuestion_Success(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	svc.On("CreateQuestion", mock.Anything).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"question":        "テスト質問",
		"weight_category": "technical",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newQuestionController(svc).CreateQuestion(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestQuestionController_CreateQuestion_ServiceError(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	svc.On("CreateQuestion", mock.Anything).Return(errors.New("db error"))

	body, _ := json.Marshal(map[string]string{
		"question":        "テスト質問",
		"weight_category": "technical",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newQuestionController(svc).CreateQuestion(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// ---- GetQuestionsByCategory ----

func TestQuestionController_GetQuestionsByCategory_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/questions?category=technical", nil)
	w := httptest.NewRecorder()
	controllers.NewQuestionController(nil).GetQuestionsByCategory(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestQuestionController_GetQuestionsByCategory_MissingCategory(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/questions", nil)
	w := httptest.NewRecorder()
	controllers.NewQuestionController(nil).GetQuestionsByCategory(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQuestionController_GetQuestionsByCategory_Success(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	questions := []models.QuestionWeight{{Question: "技術質問"}}
	svc.On("GetQuestionsByCategory", "technical").Return(questions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/questions?category=technical", nil)
	w := httptest.NewRecorder()
	newQuestionController(svc).GetQuestionsByCategory(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestQuestionController_GetQuestionsByCategory_ServiceError(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	svc.On("GetQuestionsByCategory", "technical").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/questions?category=technical", nil)
	w := httptest.NewRecorder()
	newQuestionController(svc).GetQuestionsByCategory(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
