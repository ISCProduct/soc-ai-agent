package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	internalOpenAI "Backend/internal/openai"
	"Backend/internal/models"
)

// mockLLMServer はテスト用のOpenAI互換サーバー
func mockLLMServer(t *testing.T, response LLMAnswerEvaluation) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(response)
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": string(body),
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     100,
				"completion_tokens": 50,
			},
			"model": "gpt-4o-mini",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// TestNewAnswerEvaluatorWithLLM はllmClientが設定されることを確認する
func TestNewAnswerEvaluatorWithLLM(t *testing.T) {
	client := internalOpenAI.NewWithBaseURL("http://localhost", "gpt-4o-mini")
	e := NewAnswerEvaluatorWithLLM(client)
	if e.llmClient == nil {
		t.Error("llmClient が設定されていない")
	}
}

// TestNewAnswerEvaluator_NoLLM はllmClientがnilであることを確認する
func TestNewAnswerEvaluator_NoLLM(t *testing.T) {
	e := NewAnswerEvaluator()
	if e.llmClient != nil {
		t.Error("デフォルトコンストラクタでllmClientがnilでない")
	}
}

// TestEvaluateWithLLMFallback_LowConfidence_UpgradedByLLM は
// ルールベースが "low" でもLLMが "high" を返した場合に信頼度が引き上げられることを確認する
func TestEvaluateWithLLMFallback_LowConfidence_UpgradedByLLM(t *testing.T) {
	srv := mockLLMServer(t, LLMAnswerEvaluation{
		Score:        80,
		Confidence:   "high",
		Specificity:  2,
		Authenticity: 3,
		Consistency:  2,
		Explanation:  "本音が感じられる回答です",
	})
	defer srv.Close()

	client := internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	e := NewAnswerEvaluatorWithLLM(client)

	q := &models.PredefinedQuestion{}
	// 短いがキーワードなし → ルールベースは "low" になりやすい
	result, err := e.EvaluateWithLLMFallback(context.Background(), q, "ITに興味を持ったきっかけは？", "やりがいを感じました")
	if err != nil {
		t.Fatalf("EvaluateWithLLMFallback error: %v", err)
	}
	if result.Confidence != "high" {
		t.Errorf("LLMが high を返したのに confidence=%s になった", result.Confidence)
	}
	if result.NeedsFollowUp {
		t.Error("confidence が high になったのに NeedsFollowUp が true のまま")
	}
}

// TestEvaluateWithLLMFallback_HighConfidence_NoLLMCall は
// ルールベースが "high" の場合はLLMを呼ばないことを確認する
func TestEvaluateWithLLMFallback_HighConfidence_NoLLMCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// 呼ばれたら空レスポンス
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}],"usage":{}}`))
	}))
	defer srv.Close()

	client := internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	e := NewAnswerEvaluatorWithLLM(client)

	q := &models.PredefinedQuestion{
		PositiveKeywords: `["実装","成果","改善","取り組み","経験"]`,
	}
	// キーワード多数＋長文 → ルールベースで "high" になる
	longAnswer := "具体的には、実際に3ヶ月かけてシステムを実装して改善した経験があります。成果として取り組み効率が向上しました。"
	result, err := e.EvaluateWithLLMFallback(context.Background(), q, "開発経験について教えてください", longAnswer)
	if err != nil {
		t.Fatalf("EvaluateWithLLMFallback error: %v", err)
	}
	if called {
		t.Error("confidence が low でないのにLLMが呼ばれた")
	}
	_ = result
}

// TestEvaluateWithLLMFallback_NilClient はllmClientがnilでもpanicしないことを確認する
func TestEvaluateWithLLMFallback_NilClient(t *testing.T) {
	e := NewAnswerEvaluator() // llmClient = nil
	q := &models.PredefinedQuestion{}
	result, err := e.EvaluateWithLLMFallback(context.Background(), q, "質問", "回答")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("result is nil")
	}
}

// TestEvaluateHybrid_MergesScores はルールベースとLLMのスコアが加重平均で合成されることを確認する
func TestEvaluateHybrid_MergesScores(t *testing.T) {
	srv := mockLLMServer(t, LLMAnswerEvaluation{
		Score:        80,
		Confidence:   "high",
		Specificity:  2,
		Authenticity: 2,
		Consistency:  2,
		Explanation:  "具体的な回答です",
	})
	defer srv.Close()

	client := internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	e := NewAnswerEvaluatorWithLLM(client)

	q := &models.PredefinedQuestion{}
	result, err := e.EvaluateHybrid(context.Background(), q, "強みを教えてください", "問題解決が得意です", 0.4)
	if err != nil {
		t.Fatalf("EvaluateHybrid error: %v", err)
	}
	// スコアが0以上であること
	if result.Score < 0 {
		t.Errorf("blended score が負: %d", result.Score)
	}
	// LLMが high を返したので mergeConfidence で "high" になるはず
	if result.Confidence != "high" {
		t.Errorf("expected high confidence, got %s", result.Confidence)
	}
}

// TestEvaluateHybrid_NilClient はllmClientがnilでも通常のEvaluateと同じ結果を返すことを確認する
func TestEvaluateHybrid_NilClient(t *testing.T) {
	e := NewAnswerEvaluator()
	q := &models.PredefinedQuestion{}
	hybridResult, err := e.EvaluateHybrid(context.Background(), q, "質問", "回答", 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ruleResult, _ := e.Evaluate(q, "回答")
	if hybridResult.Score != ruleResult.Score {
		t.Errorf("llmClient=nil の時 EvaluateHybrid と Evaluate のスコアが異なる: hybrid=%d rule=%d",
			hybridResult.Score, ruleResult.Score)
	}
}

// TestMergeConfidence は信頼度のマージが正しく動作することを確認する
func TestMergeConfidence(t *testing.T) {
	tests := []struct {
		a, b, want string
	}{
		{"low", "high", "high"},
		{"high", "low", "high"},
		{"medium", "high", "high"},
		{"low", "medium", "medium"},
		{"medium", "medium", "medium"},
		{"high", "high", "high"},
		{"low", "low", "low"},
	}
	for _, tt := range tests {
		got := mergeConfidence(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("mergeConfidence(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}
