package openai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractResponsesText(t *testing.T) {
	parse := func(t *testing.T, raw string) (responsesResponse, []byte) {
		t.Helper()
		var p responsesResponse
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatal(err)
		}
		return p, []byte(raw)
	}

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string // 空なら成功。部分一致
	}{
		{
			name: "output_text があればそれを優先する",
			raw:  `{"output_text":"  こんにちは  ","output":[{"content":[{"text":"別の本文"}]}]}`,
			want: "こんにちは",
		},
		{
			name: "output_text が無ければ content を改行でつなぐ",
			raw:  `{"output":[{"content":[{"text":"一行目"},{"text":" 二行目 "}]}]}`,
			want: "一行目\n二行目",
		},
		{
			name:    "error.message があればエラーにする",
			raw:     `{"error":{"message":"quota exceeded"}}`,
			wantErr: "quota exceeded",
		},
		{
			name:    "拒否(refusal)はエラーにする",
			raw:     `{"output":[{"content":[{"refusal":"お答えできません"}]}]}`,
			wantErr: "お答えできません",
		},
		{
			name:    "本文が無く、途中終了の理由があればそれを添える",
			raw:     `{"output":[],"incomplete_details":{"reason":"max_output_tokens"}}`,
			wantErr: "empty response from responses api: max_output_tokens",
		},
		{
			name:    "本文も理由も無ければ、レスポンスの先頭を添える",
			raw:     `{"output":[]}`,
			wantErr: `empty response from responses api: {"output":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, raw := parse(t, tt.raw)
			got, err := extractResponsesText(parsed, raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q を含むエラー", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// 空応答のエラー文は、呼び出し側(attemptResponses)が文字列で判定している。
// 文言を変えると、text.format を付け替えて試す救済が黙って効かなくなる。
func TestExtractResponsesText_EmptyErrorKeepsMarkerThatRetryLogicMatches(t *testing.T) {
	_, err := extractResponsesText(responsesResponse{}, []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "empty response from responses api") {
		t.Fatalf("err = %v, 救済の判定に使う文言を含めること", err)
	}
}

func TestExtractResponsesText_SnippetIsCapped(t *testing.T) {
	long := `{"output":[],"pad":"` + strings.Repeat("x", 5000) + `"}`
	_, err := extractResponsesText(responsesResponse{}, []byte(long))
	if err == nil {
		t.Fatal("エラーになるはず")
	}
	if n := len(err.Error()); n > 1100 {
		t.Fatalf("スニペットが長すぎる: %d バイト", n)
	}
}
