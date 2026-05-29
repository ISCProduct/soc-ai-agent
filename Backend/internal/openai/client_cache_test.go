package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponses_CachedTokensLogging(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/responses" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"output_text": "hello",
				"usage": map[string]any{
					"input_tokens":  100,
					"output_tokens": 10,
					"prompt_tokens_details": map[string]int{"cached_tokens": 40},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewWithBaseURL(server.URL, "gpt-4o-mini")
	called := false
	client.OnUsage = func(model string, promptTokens, completionTokens int) {
		called = true
		assert.Equal(t, "gpt-4o-mini", model)
		assert.Equal(t, 100, promptTokens)
		assert.Equal(t, 10, completionTokens)
	}

	ctx := context.Background()
	out, err := client.Responses(ctx, "input", "gpt-4o-mini")
	assert.NoError(t, err)
	assert.Equal(t, "hello", out)
	assert.True(t, called, "OnUsage should be called")
}
