package controllers

import (
	"Backend/internal/models"
	"Backend/internal/services"
	"encoding/json"
	"net/http"
)

type QuestionController struct {
	questionService *services.QuestionGeneratorService
}

func NewQuestionController(questionService *services.QuestionGeneratorService) *QuestionController {
	return &QuestionController{questionService: questionService}
}

// GenerateQuestions AIで質問を生成
func (c *QuestionController) GenerateQuestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req services.GenerateQuestionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// バリデーション
	if req.Category == "" {
		http.Error(w, "category is required", http.StatusBadRequest)
		return
	}
	if req.Count <= 0 {
		req.Count = 5 // デフォルト5個
	}

	questions, err := c.questionService.GenerateAndSaveQuestions(r.Context(), req)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"generated_count": len(questions),
		"questions":       questions,
	})
}

// CreateQuestion 手動で質問を登録
func (c *QuestionController) CreateQuestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var qw models.QuestionWeight
	if err := json.NewDecoder(r.Body).Decode(&qw); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// バリデーション
	if qw.Question == "" || qw.WeightCategory == "" {
		http.Error(w, "question and weight_category are required", http.StatusBadRequest)
		return
	}

	if err := c.questionService.CreateQuestion(&qw); err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(qw)
}

// GetQuestionsByCategory カテゴリ別質問取得
func (c *QuestionController) GetQuestionsByCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	category := r.URL.Query().Get("category")
	if category == "" {
		http.Error(w, "category is required", http.StatusBadRequest)
		return
	}

	questions, err := c.questionService.GetQuestionsByCategory(category)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(questions)
}
