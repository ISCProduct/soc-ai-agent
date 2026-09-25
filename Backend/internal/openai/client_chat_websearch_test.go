package openai

import "testing"

// TestWebSearchContextSize は企業検索の web_search コストノブを固定する（#1124）。
//
// web_search は検索結果が固定トークンとして課金されるため、この設定が
// 1コールあたりの入力トークン＝コストに直結する。以前は "high" 固定だった。
func TestWebSearchContextSize(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "未設定は medium", env: "", want: "medium"},
		{name: "low", env: "low", want: "low"},
		{name: "medium", env: "medium", want: "medium"},
		{name: "high", env: "high", want: "high"},
		{name: "大文字空白は正規化", env: "  HIGH  ", want: "high"},
		{name: "不正な値は既定に倒す", env: "huge", want: "medium"},
		{name: "数値も既定に倒す", env: "2", want: "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENAI_WEB_SEARCH_CONTEXT_SIZE", tt.env)
			if got := webSearchContextSize(); got != tt.want {
				t.Errorf("webSearchContextSize() = %q, want %q", got, tt.want)
			}
		})
	}
}
