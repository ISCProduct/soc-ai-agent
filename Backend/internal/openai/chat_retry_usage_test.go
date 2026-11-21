package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// SOCAIAGENT-655: Chat Completions の JSON 経路のリトライ判定と使用量の記録を Responses 経路に揃える。
//   - 429 以外の 4xx は1回で打ち切る（以前は最大5回・約62秒待っていた）
//   - 失敗・空応答・捨てた応答のトークンも api_call_logs（OnUsage）に記録する

type chatStep struct {
	status  int
	content string
	finish  string
	prompt  int
	output  int
}

func chatServer(t *testing.T, steps []chatStep) (*httptest.Server, *int) {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		step := steps[min(calls, len(steps)-1)]
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if step.status != http.StatusOK {
			w.WriteHeader(step.status)
			_, _ = w.Write([]byte(`{"error":{"message":"err","type":"invalid_request_error"}}`))
			return
		}
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": step.content}, "finish_reason": step.finish}},
			"usage":   map[string]any{"prompt_tokens": step.prompt, "completion_tokens": step.output, "total_tokens": step.prompt + step.output},
		})
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestChatCompletionJSON_RetryAndUsage(t *testing.T) {
	orig := chatCompletionBackoff
	chatCompletionBackoff = func(int) time.Duration { return time.Millisecond }
	t.Cleanup(func() { chatCompletionBackoff = orig })

	tests := []struct {
		name       string
		steps      []chatStep
		wantErr    bool
		wantCalls  int
		wantUsages []int // 記録された prompt_tokens（呼んだ順）
	}{
		{name: "400 は1回で打ち切る", steps: []chatStep{{status: 400}}, wantErr: true, wantCalls: 1},
		{name: "401 も1回で打ち切る", steps: []chatStep{{status: 401}}, wantErr: true, wantCalls: 1},
		{name: "422 も1回で打ち切る", steps: []chatStep{{status: 422}}, wantErr: true, wantCalls: 1},
		{
			name:      "429 の後に 200 ならリトライして成功（従来どおり）",
			steps:     []chatStep{{status: 429}, {status: 200, content: `{"ok":true}`, prompt: 10, output: 5}},
			wantCalls: 2, wantUsages: []int{10},
		},
		{
			name:      "500 の後に 200 ならリトライして成功",
			steps:     []chatStep{{status: 500}, {status: 200, content: `{"ok":true}`, prompt: 10, output: 5}},
			wantCalls: 2, wantUsages: []int{10},
		},
		{
			name: "出力上限で本文が空でも usage を記録し、捨てて再送した分も記録する",
			steps: []chatStep{
				{status: 200, content: "", finish: "length", prompt: 30, output: 200},
				{status: 200, content: `{"ok":true}`, finish: "stop", prompt: 31, output: 20},
			},
			wantCalls: 2, wantUsages: []int{30, 31},
		},
		{
			name:      "空応答が続いて失敗しても、すべての応答の usage を記録する",
			steps:     []chatStep{{status: 200, content: "", prompt: 7, output: 1}},
			wantErr:   true,
			wantCalls: 5, wantUsages: []int{7, 7, 7, 7, 7},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, calls := chatServer(t, tt.steps)
			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
			var mu sync.Mutex
			var usages []int
			cli.OnUsage = func(u Usage) {
				mu.Lock()
				usages = append(usages, u.PromptTokens)
				mu.Unlock()
			}

			_, err := cli.ChatCompletionJSON(context.Background(), "system", "user", 0.2, 100)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if *calls != tt.wantCalls {
				t.Errorf("リクエスト = %d 回, want %d", *calls, tt.wantCalls)
			}
			if len(usages) != len(tt.wantUsages) {
				t.Fatalf("記録した usage = %v, want %v", usages, tt.wantUsages)
			}
			for i := range usages {
				if usages[i] != tt.wantUsages[i] {
					t.Errorf("usage[%d] = %d, want %d", i, usages[i], tt.wantUsages[i])
				}
			}
		})
	}
}

func TestIsNonRetryableAPIErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"Responses の 400", &ResponsesAPIError{StatusCode: 400}, true},
		{"Responses の 429", &ResponsesAPIError{StatusCode: 429}, false},
		{"Responses の 503", &ResponsesAPIError{StatusCode: 503}, false},
		{"ステータスの無いエラー（ネットワーク等）は再送する", context.DeadlineExceeded, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNonRetryableAPIErr(tt.err); got != tt.want {
				t.Fatalf("isNonRetryableAPIErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// Embedding も、結果が空でも課金されたトークンを記録する。
func TestEmbedding_RecordsUsageEvenWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"usage":{"prompt_tokens":12,"total_tokens":12}}`))
	}))
	defer srv.Close()
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
	var recorded []int
	cli.OnUsage = func(u Usage) { recorded = append(recorded, u.PromptTokens) }

	if _, err := cli.Embedding(context.Background(), "テキスト"); err == nil {
		t.Fatal("空の結果はエラーにする")
	}
	if len(recorded) != 1 || recorded[0] != 12 {
		t.Fatalf("記録した usage = %v, want [12]", recorded)
	}
}
