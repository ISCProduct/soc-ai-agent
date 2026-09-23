package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCountWebSearchTools は web_search ツールの検出を固定する。
//
// web_search はトークンとは別に1コール単位で課金される。モデル名では判別
// できない（同じ gpt-4o-mini に通常のチャットが混ざる）ため、リクエストの
// tools から数えている。ここが取りこぼすと、検索コストの85%を占める
// ツール料が api_call_logs に記録されない。
func TestCountWebSearchTools(t *testing.T) {
	tests := []struct {
		name  string
		tools []map[string]any
		want  int
	}{
		{name: "tools なし", tools: nil, want: 0},
		{name: "空スライス", tools: []map[string]any{}, want: 0},
		{
			name:  "WebSearchJSON が実際に送る形",
			tools: []map[string]any{{"type": "web_search", "search_context_size": "medium"}},
			want:  1,
		},
		{
			name:  "プレビュー版の型名も数える",
			tools: []map[string]any{{"type": "web_search_preview"}},
			want:  1,
		},
		{
			name:  "検索以外のツールは数えない",
			tools: []map[string]any{{"type": "function", "name": "do_something"}},
			want:  0,
		},
		{
			name: "混在していても検索だけ数える",
			tools: []map[string]any{
				{"type": "function"},
				{"type": "web_search", "search_context_size": "low"},
				{"type": "file_search"},
			},
			want: 1,
		},
		{
			name:  "type が文字列でなければ数えない",
			tools: []map[string]any{{"type": 1}},
			want:  0,
		},
		{
			name:  "type が無ければ数えない",
			tools: []map[string]any{{"search_context_size": "medium"}},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countWebSearchTools(tt.tools); got != tt.want {
				t.Errorf("countWebSearchTools() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestWebSearchJSON_ReportsWebSearchCall は WebSearchJSON が実際に
// WebSearchCalls=1 を記録することを、リクエスト内容ごと固定する。
//
// countWebSearchTools の単体テストだけでは「呼び出し側が tools を
// 付け忘れた」「usageReport へ繋ぎ忘れた」場合に気付けない。
func TestWebSearchJSON_ReportsWebSearchCall(t *testing.T) {
	var gotTools []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Tools []map[string]any `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotTools = req.Tools
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"{}","usage":{"input_tokens":9000,"output_tokens":500}}`))
	}))
	defer srv.Close()

	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
	var got []Usage
	cli.OnUsage = func(u Usage) { got = append(got, u) }

	if _, err := cli.WebSearchJSON(context.Background(), "テスト企業の概要", 600); err != nil {
		t.Fatalf("WebSearchJSON: %v", err)
	}

	if n := countWebSearchTools(gotTools); n != 1 {
		t.Fatalf("送信された tools に web_search が %d 個（tools=%v）", n, gotTools)
	}
	if len(got) != 1 {
		t.Fatalf("使用量の通知が %d 件、want 1", len(got))
	}
	if got[0].WebSearchCalls != 1 {
		t.Errorf("WebSearchCalls = %d, want 1（ツール料が記録されない）", got[0].WebSearchCalls)
	}
}

// TestResponses_DoesNotReportWebSearchCall は検索を伴わない Responses 呼び出しに
// ツール料を付けないことを固定する。付けると通常のチャットまで水増しされる。
func TestResponses_DoesNotReportWebSearchCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"ok","usage":{"input_tokens":100,"output_tokens":20}}`))
	}))
	defer srv.Close()

	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
	var got []Usage
	cli.OnUsage = func(u Usage) { got = append(got, u) }

	if _, err := cli.Responses(context.Background(), "こんにちは"); err != nil {
		t.Fatalf("Responses: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("使用量の通知が %d 件、want 1", len(got))
	}
	if got[0].WebSearchCalls != 0 {
		t.Errorf("WebSearchCalls = %d, want 0", got[0].WebSearchCalls)
	}
}
