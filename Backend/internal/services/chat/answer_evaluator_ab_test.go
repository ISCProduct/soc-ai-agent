package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	internalOpenAI "Backend/internal/openai"
	"Backend/internal/services/prompts"
	"Backend/internal/usagectx"
)

// #1976: 割り当てた版の本文で LLM を呼び、同じ版と群を出力と利用量の記録に残す。
// 本文の選択・割当・記録のどれかがずれると、比較の結果が別の本文のものになる。
func TestLLMEvaluate_ABの本文と記録が揃う(t *testing.T) {
	t.Cleanup(prompts.RegisterCandidateBodies(prompts.StageScoreEvaluation, "v3", map[string]string{
		prompts.BodyAnswerQuality: "候補の評価プロンプト",
	}))
	t.Setenv("PROMPT_AB_SCORE_EVALUATION", "v2:50,v3:50")

	// 候補（v3）と今の版（v2）に割り当てられる利用者を1人ずつ探す
	users := map[string]uint{}
	for id := uint(1); len(users) < 2 && id < 1000; id++ {
		version, _ := prompts.AssignPromptVersion(prompts.StageScoreEvaluation, prompts.UserSubject(id))
		if _, ok := users[version]; !ok {
			users[version] = id
		}
	}
	tests := []struct {
		version    string
		wantSystem string
	}{
		{version: "v3", wantSystem: "候補の評価プロンプト"},
		{version: "v2", wantSystem: prompts.AnswerQualitySystemPrompt},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			var system string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Messages []struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"messages"`
				}
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &body)
				for _, m := range body.Messages {
					if m.Role == "system" {
						system = m.Content
					}
				}
				_, _ = w.Write([]byte(`{"model":"gpt-4o-mini","usage":{"prompt_tokens":10,"completion_tokens":5},` +
					`"choices":[{"message":{"content":"{\"score\":80,\"confidence\":\"high\"}"}}]}`))
			}))
			defer server.Close()

			var usage internalOpenAI.Usage
			client := internalOpenAI.NewWithBaseURL(server.URL, "gpt-4o-mini")
			client.OnUsage = func(u internalOpenAI.Usage) { usage = u }
			e := &AnswerEvaluator{llmClient: client}

			ctx := usagectx.WithActor(context.Background(), users[tt.version], 0)
			result, version := e.llmEvaluateWithPromptVersion(ctx, "質問", "回答")
			if result == nil {
				t.Fatal("評価が返らない")
			}
			if version != tt.version || !strings.Contains(system, tt.wantSystem) {
				t.Fatalf("version=%q system=%q, want %q と %q", version, system, tt.version, tt.wantSystem)
			}
			if usage.PromptStage != string(prompts.StageScoreEvaluation) || usage.PromptVersion != tt.version || usage.PromptABGroup != tt.version {
				t.Fatalf("usage stage=%q version=%q group=%q", usage.PromptStage, usage.PromptVersion, usage.PromptABGroup)
			}
		})
	}
}
