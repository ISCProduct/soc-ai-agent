package resume

import (
	"encoding/json"
	"strings"
	"testing"

	"Backend/internal/models"
)

func TestFormatFindingsFor_KeepsBlockIndexZero(t *testing.T) {
	fs := formatFindingsFor([]models.ResumeTextBlock{{PageNumber: 1, BlockIndex: 0, Text: "飲食店のバイトをしました。"}})
	if len(fs) != 1 {
		t.Fatalf("got %+v", fs)
	}
	raw, err := json.Marshal(fs)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{`"block_index":0`, `"page_hint":1`} {
		if !strings.Contains(s, want) {
			t.Errorf("%s が無い: %s", want, s)
		}
	}
	if n := strings.Count(s, `"block_index"`); n != 1 {
		t.Errorf("block_index が %d 回出た: %s", n, s)
	}
}

// 画面はブロックの本文を持たないので、該当箇所の文字列を excerpt で返す（#1973 のレビュー）。
func TestFormatFindingsFor_Excerpt(t *testing.T) {
	blocks := []models.ResumeTextBlock{
		{PageNumber: 1, BlockIndex: 0, Text: "私は学生です。"},
		{PageNumber: 1, BlockIndex: 1, Text: "毎日勉強しています。"},
		{PageNumber: 2, BlockIndex: 0, Text: "自己PR"},
		{PageNumber: 2, BlockIndex: 1, Text: "🎓成果を上げた。"},
		{PageNumber: 2, BlockIndex: 2, Text: "飲食店の バイト をしました。"},
	}
	got := map[string]FormatFinding{}
	for _, f := range formatFindingsFor(blocks) {
		got[f.Rule] = f
	}
	if f := got["mixed_style"]; f.PageHint != 2 || f.BlockIndex != 1 || f.Excerpt != "🎓成果を上げた。" {
		t.Errorf("mixed_style = %+v（見出しではなく本文のブロックを指し、絵文字の後ろもずれないこと）", f)
	}
	if f := got["spoken_language"]; f.Excerpt != "バイト" {
		t.Errorf("spoken_language の excerpt = %q", f.Excerpt)
	}
}

func TestFormatExcerpt(t *testing.T) {
	cases := []struct {
		name, text string
		start, end int
		want       string
	}{
		{"空白をまとめる", "a\n  b", 0, 5, "a b"},
		{"範囲外は空", "短い", 0, 9, ""},
		{"長いと切る", strings.Repeat("あ", 40), 0, 40, strings.Repeat("あ", formatExcerptMax) + "…"},
	}
	for _, tc := range cases {
		if got := formatExcerpt(tc.text, tc.start, tc.end); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestFormatFindingsFor_EmptyIsArray(t *testing.T) {
	raw, _ := json.Marshal(formatFindingsFor(nil))
	if string(raw) != "[]" {
		t.Errorf("got %s", raw)
	}
}
