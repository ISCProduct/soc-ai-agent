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
	"Backend/internal/services/chat"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/mock"
)

func newQuestionController(svc *mocks.QuestionServiceMock) *controllers.QuestionController {
	return controllers.NewQuestionController(svc)
}

// ---- GenerateQuestions ----

func TestQuestionController_GenerateQuestions_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewQuestionController(nil).GenerateQuestions, newCtx(req, rec), http.StatusBadRequest)
}

func TestQuestionController_GenerateQuestions_DefaultCount(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	expected := chat.GenerateQuestionsRequest{Category: "technical", Count: 5}
	questions := []models.QuestionWeight{{Question: "test question"}}
	svc.On("GenerateAndSaveQuestions", mock.Anything, expected).Return(questions, nil)

	body, _ := json.Marshal(map[string]string{"category": "technical"})
	req := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).GenerateQuestions, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestQuestionController_GenerateQuestions_ServiceError(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	svc.On("GenerateAndSaveQuestions", mock.Anything, mock.Anything).Return(nil, errors.New("openai error"))

	body, _ := json.Marshal(map[string]interface{}{"category": "technical", "count": 3})
	req := httptest.NewRequest(http.MethodPost, "/api/questions/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).GenerateQuestions, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

// ---- CreateQuestion ----

func TestQuestionController_CreateQuestion_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewQuestionController(nil).CreateQuestion, newCtx(req, rec), http.StatusBadRequest)
}

func TestQuestionController_CreateQuestion_Success(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	svc.On("CreateQuestion", mock.Anything).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"question":        "テスト質問",
		"weight_category": "技術志向",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).CreateQuestion, newCtx(req, rec), http.StatusCreated)
	svc.AssertExpectations(t)
}

func TestQuestionController_CreateQuestion_ServiceError(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	svc.On("CreateQuestion", mock.Anything).Return(errors.New("db error"))

	body, _ := json.Marshal(map[string]string{
		"question":        "テスト質問",
		"weight_category": "技術志向",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).CreateQuestion, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

// ---- GetQuestionsByCategory ----

func TestQuestionController_GetQuestionsByCategory_Success(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	questions := []models.QuestionWeight{{Question: "技術質問"}}
	svc.On("GetQuestionsByCategory", "technical").Return(questions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/questions?category=technical", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).GetQuestionsByCategory, newCtx(req, rec), http.StatusOK)
	svc.AssertExpectations(t)
}

func TestQuestionController_GetQuestionsByCategory_ServiceError(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	svc.On("GetQuestionsByCategory", "technical").Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/questions?category=technical", nil)
	rec := httptest.NewRecorder()
	assertStatus(t, newQuestionController(svc).GetQuestionsByCategory, newCtx(req, rec), http.StatusInternalServerError)
	svc.AssertExpectations(t)
}

// TestQuestionController_CreateQuestion_RejectsUnknownCategory は、
// 正典外のカテゴリが question_weights へ入らないことを検証する（#929）。
//
// user_weight_scores 側はリポジトリで塞いだが、このAPIは認証ユーザーからの
// 任意文字列をそのまま保存する別経路だった。
func TestQuestionController_CreateQuestion_RejectsUnknownCategory(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}

	body, _ := json.Marshal(map[string]string{
		"question":        "テスト質問",
		"weight_category": "ぜんぜん違うカテゴリ",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewQuestionController(svc).CreateQuestion, newCtx(req, rec), http.StatusBadRequest)
	svc.AssertNotCalled(t, "CreateQuestion", mock.Anything)
}

// 表記揺れは弾かずに正典へ寄せて保存する。
func TestQuestionController_CreateQuestion_NormalizesAlias(t *testing.T) {
	svc := &mocks.QuestionServiceMock{}
	var saved *models.QuestionWeight
	svc.On("CreateQuestion", mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(0).(*models.QuestionWeight)
	}).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"question":        "テスト質問",
		"weight_category": "チームワーク", // 正典は「チームワーク志向」
	})
	req := httptest.NewRequest(http.MethodPost, "/api/questions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	assertStatus(t, controllers.NewQuestionController(svc).CreateQuestion, newCtx(req, rec), http.StatusCreated)

	if saved == nil {
		t.Fatal("CreateQuestion が呼ばれていない")
	}
	if saved.WeightCategory != "チームワーク志向" {
		t.Errorf("WeightCategory = %q, want 「チームワーク志向」", saved.WeightCategory)
	}
}
