package services_test

// AnswerEvaluator LLM評価機能のテスト (Issue #450)
//
// EvaluateWithLLMFallback / EvaluateHybrid の振る舞いを、
// モックHTTPサーバー経由で検証する。
//
// 実行: cd Backend && go test ./test/services/... -run LLM -v

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/models"
	internalOpenAI "Backend/internal/openai"
	"Backend/internal/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockLLMServer はLLM評価レスポンスを返すモックHTTPサーバーを生成する
func newMockLLMServer(t *testing.T, score int, confidence string, explanation string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eval := map[string]any{
			"score":        score,
			"confidence":   confidence,
			"specificity":  2,
			"authenticity": 2,
			"consistency":  2,
			"explanation":  explanation,
		}
		body, _ := json.Marshal(eval)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": string(body)}},
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

// ──────────────────────────────────────────────
// EvaluateWithLLMFallback のテスト
// ──────────────────────────────────────────────

// TestLLM_FallbackUpgradesLowConfidence は、ルールベースが "low" でも
// LLMが "high" を返すと信頼度が引き上げられることを確認する
func TestLLM_FallbackUpgradesLowConfidence(t *testing.T) {
	srv := newMockLLMServer(t, 80, "high", "本音が感じられる回答です")
	defer srv.Close()

	client := internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	e := services.NewAnswerEvaluatorWithLLM(client)

	// キーワードなし・短答 → ルールベースは "low" になる
	q := &models.PredefinedQuestion{}
	result, err := e.EvaluateWithLLMFallback(
		context.Background(), q,
		"ITに興味を持ったきっかけは？",
		"やりがいを感じました",
	)
	require.NoError(t, err)
	assert.Equal(t, "high", result.Confidence, "LLMがhighを返したのに信頼度が引き上げられなかった")
	assert.False(t, result.NeedsFollowUp, "confidence=highなのにNeedsFollowUpがtrueのまま")
	assert.Equal(t, "本音が感じられる回答です", result.Explanation)
}

// TestLLM_FallbackSkipsLLMWhenNotLow は、ルールベースが "low" でない場合に
// LLMを呼ばず既存の結果をそのまま返すことを確認する
func TestLLM_FallbackSkipsLLMWhenNotLow(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}],"usage":{}}`))
	}))
	defer srv.Close()

	client := internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	e := services.NewAnswerEvaluatorWithLLM(client)

	// キーワード多数 → ルールベースで "high" または "medium" になる
	q := &models.PredefinedQuestion{
		PositiveKeywords: `["実装","成果","改善","取り組み","経験"]`,
	}
	answer := "具体的には、実際に3ヶ月かけてシステムを実装して改善した経験があります。成果として取り組み効率が向上しました。"
	_, err := e.EvaluateWithLLMFallback(context.Background(), q, "開発経験について教えてください", answer)
	require.NoError(t, err)
	assert.False(t, called, "confidence がlowでないのにLLMが呼ばれた")
}

// TestLLM_FallbackNilClientFallsBackToRuleBase は llmClient が nil でも
// panic せず通常の Evaluate と同じ結果を返すことを確認する
func TestLLM_FallbackNilClientFallsBackToRuleBase(t *testing.T) {
	e := services.NewAnswerEvaluator()
	q := &models.PredefinedQuestion{}

	resultFallback, err := e.EvaluateWithLLMFallback(context.Background(), q, "質問", "回答")
	require.NoError(t, err)

	resultRule, err := e.Evaluate(q, "回答")
	require.NoError(t, err)

	assert.Equal(t, resultRule.Score, resultFallback.Score)
	assert.Equal(t, resultRule.Confidence, resultFallback.Confidence)
}

// ──────────────────────────────────────────────
// EvaluateHybrid のテスト
// ──────────────────────────────────────────────

// TestLLM_HybridMergesConfidence は LLM が "high" を返した場合に
// ハイブリッド評価の信頼度が "high" になることを確認する
func TestLLM_HybridMergesConfidence(t *testing.T) {
	srv := newMockLLMServer(t, 85, "high", "具体的な回答です")
	defer srv.Close()

	client := internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	e := services.NewAnswerEvaluatorWithLLM(client)

	q := &models.PredefinedQuestion{}
	result, err := e.EvaluateHybrid(context.Background(), q, "強みを教えてください", "問題解決が得意です", 0.4)
	require.NoError(t, err)
	assert.Equal(t, "high", result.Confidence)
}

// TestLLM_HybridScoreIsBlended は加重平均によるスコア合成で
// スコアが 0〜10 の範囲に収まることを確認する
func TestLLM_HybridScoreIsBlended(t *testing.T) {
	srv := newMockLLMServer(t, 60, "medium", "標準的な回答です")
	defer srv.Close()

	client := internalOpenAI.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	e := services.NewAnswerEvaluatorWithLLM(client)

	q := &models.PredefinedQuestion{}
	result, err := e.EvaluateHybrid(context.Background(), q, "質問", "回答", 0.5)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Score, 0, "スコアが負になった")
	assert.LessOrEqual(t, result.Score, 10, "スコアが10を超えた")
}

// TestLLM_HybridNilClientEqualsEvaluate は llmClient が nil の場合に
// EvaluateHybrid が Evaluate と同じ結果を返すことを確認する
func TestLLM_HybridNilClientEqualsEvaluate(t *testing.T) {
	e := services.NewAnswerEvaluator()
	q := &models.PredefinedQuestion{}

	hybrid, err := e.EvaluateHybrid(context.Background(), q, "質問", "回答", 0.5)
	require.NoError(t, err)

	rule, err := e.Evaluate(q, "回答")
	require.NoError(t, err)

	assert.Equal(t, rule.Score, hybrid.Score)
	assert.Equal(t, rule.Confidence, hybrid.Confidence)
}

// ──────────────────────────────────────────────
// MergeConfidence のテスト
// ──────────────────────────────────────────────

func TestLLM_MergeConfidence(t *testing.T) {
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
		got := services.MergeConfidence(tt.a, tt.b)
		assert.Equal(t, tt.want, got, "MergeConfidence(%q, %q)", tt.a, tt.b)
	}
}
