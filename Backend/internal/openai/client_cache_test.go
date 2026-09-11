package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
					"input_tokens":          100,
					"output_tokens":         10,
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

func TestChatCompletion_CachedTokensLogging(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/chat/completions") {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"choices": []map[string]any{{
					"message": map[string]string{"content": "hello"},
				}},
				"usage": map[string]any{
					"prompt_tokens":         100,
					"completion_tokens":     10,
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
	out, err := client.ChatCompletionJSON(ctx, "system", "user", 0.0, 100, "gpt-4o-mini")
	assert.NoError(t, err)
	assert.Equal(t, "hello", out)
	assert.True(t, called, "OnUsage should be called for ChatCompletion")
}

func TestWebSearchJSON_UsesResponsesWebSearchTool(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output_text": "found",
			"usage":       map[string]any{"input_tokens": 80, "output_tokens": 20},
		})
	}))
	defer server.Close()

	client := NewWithBaseURL(server.URL, "gpt-4o-mini")
	out, err := client.WebSearchJSON(context.Background(), "NECについて", 400, "gpt-5-search-api")
	assert.NoError(t, err)
	assert.Equal(t, "found", out)
	assert.Equal(t, "gpt-4o-mini", gotBody["model"])
	assert.Equal(t, "required", gotBody["tool_choice"])
	tools, _ := gotBody["tools"].([]any)
	assert.NotEmpty(t, tools)
	tool, _ := tools[0].(map[string]any)
	assert.Equal(t, "web_search", tool["type"])
	assert.Equal(t, "high", tool["search_context_size"])
}
