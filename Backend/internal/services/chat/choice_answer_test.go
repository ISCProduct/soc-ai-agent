package chat

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
		wantReason   string
	}{
		{"A", true, "A", false, ""},
		{"a", true, "A", false, ""},
		{"自分から主導して進める", true, "A", false, ""},
		{"みんなで協力して進める", true, "B", false, ""},
		{"チームで相談しながら進めたいです", false, "", true, ""},
		{"その他（自由記述）", false, "", true, ""},
		{"A: チームで進めるのが好きです", true, "A", false, "チームで進めるのが好きです"},
		{"B：理由を添えます", true, "B", false, "理由を添えます"},
	}
	for _, tc := range cases {
		got := ResolveChoiceAnswer(q, tc.in)
		if got.IsChoice != tc.wantChoice || got.IsFreeText != tc.wantFreeText || got.Letter != tc.wantLetter || got.Reason != tc.wantReason {
			t.Fatalf("in=%q got=%+v want choice=%v letter=%q free=%v reason=%q",
				tc.in, got, tc.wantChoice, tc.wantLetter, tc.wantFreeText, tc.wantReason)
		}
	}
}

func TestBlendChoiceAndReasonScore(t *testing.T) {
	if got := blendChoiceAndReasonScore(100, 40, "短い"); got != 100 {
		t.Fatalf("short reason should keep choice score, got %d", got)
	}
	got := blendChoiceAndReasonScore(100, 40, "これは十分長い理由テキストです")
	want := int(0.7*100 + 0.3*40) // 82
	if got != want {
		t.Fatalf("blended=%d want %d", got, want)
	}
}

func TestSplitChoiceAndReason(t *testing.T) {
	letter, reason, ok := SplitChoiceAndReason("A: hello")
	if !ok || letter != "A" || reason != "hello" {
		t.Fatalf("got %q %q %v", letter, reason, ok)
	}
	_, _, ok = SplitChoiceAndReason("ただの自由記述")
	if ok {
		t.Fatal("expected not ok")
	}
}

func TestResolveChoiceAnswer_NonChoiceQuestion(t *testing.T) {
	q := "具体的なエピソードを教えてください。"
	got := ResolveChoiceAnswer(q, "インターンでAPIを作りました")
	if !got.IsFreeText || got.IsChoice {
		t.Fatalf("expected free text: %+v", got)
	}
}
