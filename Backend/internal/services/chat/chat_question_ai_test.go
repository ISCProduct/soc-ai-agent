package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	internalOpenAI "Backend/internal/openai"
)

func TestSimplifyQuestionWithAI_RecordsQuestionGenerationVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output_text": "簡略化した質問",
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
		})
	}))
	defer server.Close()

	client := internalOpenAI.NewWithBaseURL(server.URL, "gpt-4o-mini")
	var recorded internalOpenAI.Usage
	client.OnUsage = func(usage internalOpenAI.Usage) {
		recorded = usage
	}
	service := &ChatService{aiClient: client}

	got, err := service.simplifyQuestionWithAI(context.Background(), "長い質問です", 42)
	if err != nil {
		t.Fatalf("simplifyQuestionWithAI returned error: %v", err)
	}
	if got != "簡略化した質問" {
		t.Fatalf("simplified question = %q, want %q", got, "簡略化した質問")
	}
	if recorded.PromptStage != "question_generation" || recorded.PromptVersion != "v2" || recorded.PromptVariant != "control" {
		t.Fatalf("recorded prompt metadata = (%q, %q, %q), want question_generation/v2/control",
			recorded.PromptStage, recorded.PromptVersion, recorded.PromptVariant)
	}
}
