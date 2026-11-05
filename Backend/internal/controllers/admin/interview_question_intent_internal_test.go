package admin

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// #1092: AI が生成した観点は、拒否せず上限（300字）に切り詰める。
func TestClipEvaluationIntent(t *testing.T) {
	long := strings.Repeat("あ", 301)
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"空はそのまま", "", ""},
		{"前後の空白を落とす", "  粘り強さ  ", "粘り強さ"},
		{"300字ちょうどはそのまま", strings.Repeat("あ", 300), strings.Repeat("あ", 300)},
		{"301字は300字に切り詰める", long, string([]rune(long)[:300])},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := clipEvaluationIntent(tc.in)
			if got != tc.want {
				t.Errorf("clipEvaluationIntent = %d字, want %d字", utf8.RuneCountInString(got), utf8.RuneCountInString(tc.want))
			}
		})
	}
}
