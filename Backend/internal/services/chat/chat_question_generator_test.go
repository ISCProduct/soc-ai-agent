package chat

import (
	"Backend/domain/valueobject"
	"Backend/internal/models"
	"testing"
)

func TestIsJobSelectionQuestion(t *testing.T) {
	s := &ChatService{}
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"empty", "", false},
		{"whitespace", "   ", false},
		{"job keyword", "どの職種に興味がありますか？", true},
		{"select with job context", "以下の職種から選んでください", true},
		{"select without job context", "以下から選んでください", false},
		{"number hint", "番号で答えても職種名でも構いません", true},
		{"undecided", "まだ決めていない場合も教えてください", true},
		{"preference clarification", "どんな作業が好きですか？", true},
		{"closest with job options", "どれが近いですか？\n1. エンジニア\n2. 営業", true},
		{"closest without job context", "どれが近いですか？", false},
		{"interview mcq closest", "その方向性で作るときに一番モヤっとしたのはどれですか？（一番近いものを選んでください）", false},
		{"unrelated", "最近頑張ったことを教えてください", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.isJobSelectionQuestion(tc.text)
			if got != tc.want {
				t.Fatalf("isJobSelectionQuestion(%q)=%v want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestShouldValidateJobCategory(t *testing.T) {
	s := &ChatService{}
	cases := []struct {
		name    string
		history []models.ChatMessage
		want    bool
	}{
		{
			name:    "empty history",
			history: nil,
			want:    true,
		},
		{
			name: "job selection last assistant",
			history: []models.ChatMessage{
				{Role: "assistant", Content: "どの職種に興味がありますか？"},
			},
			want: true,
		},
		{
			name: "non job selection last assistant",
			history: []models.ChatMessage{
				{Role: "assistant", Content: "最近頑張ったことを教えてください"},
				{Role: "user", Content: "ハッカソンに参加しました"},
			},
			want: false,
		},
		{
			name: "interview choice after experience must not re-enter job validation",
			history: []models.ChatMessage{
				{Role: "assistant", Content: "経験について具体的に教えてください。"},
				{Role: "user", Content: "児童養護施設向けのUIを作りました"},
				{Role: "assistant", Content: "一番モヤっとしたのはどれですか？（一番近いものを選んでください）\nA) 要件が曖昧\nB) 技術制約\nC) ステークホルダー調整"},
				{Role: "user", Content: "A"},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.shouldValidateJobCategory(tc.history)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

// TestGetCategoryOrder_ReturnsCanonicalCategories は、質問の優先順位が
// 正典カテゴリだけで構成され、10種すべてを重複なく網羅することを検証する（#929）。
//
// ここが正典と食い違うと、chat_question_predefined.go の絞り込みが一致せず
// 事前定義質問が使われないまま毎回AI生成に落ち、未評価カテゴリの判定も
// scoreMap(正典キー)と突き合わないため機能しなくなる。
func TestGetCategoryOrder_ReturnsCanonicalCategories(t *testing.T) {
	canonical := map[string]bool{}
	for _, c := range valueobject.AllWeightCategories() {
		canonical[string(c)] = true
	}

	check := func(label string, got []string) {
		t.Helper()
		if len(got) != 10 {
			t.Fatalf("%s: len=%d want 10 (%v)", label, len(got), got)
		}
		seen := map[string]bool{}
		for _, c := range got {
			if !canonical[c] {
				t.Errorf("%s: 正典外のカテゴリ %q が含まれる", label, c)
			}
			if seen[c] {
				t.Errorf("%s: カテゴリ %q が重複している", label, c)
			}
			seen[c] = true
		}
		if len(seen) != 10 {
			t.Errorf("%s: 10種を網羅していない (%d種)", label, len(seen))
		}
	}

	// 職種未決定
	check("undecided", (&ChatService{}).getCategoryOrder(0))
	// 各職種コードの分岐（DBに触らない純関数側で検証する）
	for _, code := range []string{"ENG01", "SALES01", "MKT01", "HR01", "FIN01", "CONS01", "OTHER"} {
		check(code, categoryOrderForJobCode(code))
	}
}

// 全ての正典カテゴリにフォールバック質問が用意されていること。
// 用意が無いとカテゴリ指定が無視され、汎用質問に落ちる。
func TestFallbackQuestions_CoverAllCanonicalCategories(t *testing.T) {
	s := &ChatService{}
	for _, c := range valueobject.AllWeightCategories() {
		for _, level := range []string{"新卒", "中途"} {
			if got := s.fallbackQuestionForCategory(string(c), 0, level); got == "" {
				t.Errorf("%s(%s): 単数フォールバックが空", c, level)
			}
			if got := s.fallbackQuestionsForCategory(string(c), 0, level); len(got) == 0 {
				t.Errorf("%s(%s): 複数フォールバックが空", c, level)
			}
		}
	}
}

func TestSelectFallbackQuestion_SkipsAsked(t *testing.T) {
	s := &ChatService{}
	asked := map[string]bool{
		"最近頑張ったことはありますか？": true,
	}
	got := s.selectFallbackQuestion("不明カテゴリ", 0, "新卒", asked)
	if got == "" {
		t.Fatal("expected non-empty fallback")
	}
	if asked[got] {
		t.Fatalf("returned already-asked question: %q", got)
	}
}

func TestSelectFallbackQuestion_CategoryOptions(t *testing.T) {
	s := &ChatService{}
	got := s.selectFallbackQuestion("創造性志向", 0, "新卒", map[string]bool{})
	if got == "" {
		t.Fatal("expected creativity fallback")
	}
	options := s.fallbackQuestionsForCategory("創造性志向", 0, "新卒")
	found := false
	for _, q := range options {
		if q == got {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("got %q not in category options %+v", got, options)
	}
}

func TestFallbackQuestionForCategory_DefaultEmpty(t *testing.T) {
	s := &ChatService{}
	if got := s.fallbackQuestionForCategory("存在しないカテゴリ", 0, "新卒"); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}
