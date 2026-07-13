package services

import "testing"

func TestParseChoiceOptions(t *testing.T) {
	q := `興味のある働き方はどれですか？

1) 新しい技術やツールに触れる
2) 仕組みを考えたり設計する
3) 人と関わりながら進める
4) コツコツ改善・整理する
5) その他（自由記述）`
	got := ParseChoiceOptions(q)
	if len(got) != 5 {
		t.Fatalf("len=%d want 5: %+v", len(got), got)
	}
	if got[0].Value != "1" || got[0].Text != "新しい技術やツールに触れる" {
		t.Fatalf("first option: %+v", got[0])
	}
	if got[4].Value != "5" || !isOtherChoiceText(got[4].Text) {
		t.Fatalf("other option: %+v", got[4])
	}
}

func TestResolveChoiceAnswer_LetterAndLabel(t *testing.T) {
	q := `どれに近いですか？
A) 自分から主導して進める
B) みんなで協力して進める
C) その他（自由記述）`

	cases := []struct {
		in           string
		wantChoice   bool
		wantLetter   string
		wantFreeText bool
	}{
		{"A", true, "A", false},
		{"a", true, "A", false},
		{"自分から主導して進める", true, "A", false},
		{"みんなで協力して進める", true, "B", false},
		{"チームで相談しながら進めたいです", false, "", true},
		{"その他（自由記述）", false, "", true},
	}
	for _, tc := range cases {
		got := ResolveChoiceAnswer(q, tc.in)
		if got.IsChoice != tc.wantChoice || got.IsFreeText != tc.wantFreeText || got.Letter != tc.wantLetter {
			t.Fatalf("in=%q got=%+v want choice=%v letter=%q free=%v",
				tc.in, got, tc.wantChoice, tc.wantLetter, tc.wantFreeText)
		}
	}
}

func TestResolveChoiceAnswer_NonChoiceQuestion(t *testing.T) {
	q := "具体的なエピソードを教えてください。"
	got := ResolveChoiceAnswer(q, "インターンでAPIを作りました")
	if !got.IsFreeText || got.IsChoice {
		t.Fatalf("expected free text: %+v", got)
	}
}
