package services

import (
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

func TestGetCategoryOrder_Undecided(t *testing.T) {
	s := &ChatService{}
	got := s.getCategoryOrder(0)
	if len(got) != 10 {
		t.Fatalf("len=%d want 10", len(got))
	}
	if got[0] != "コミュニケーション能力" {
		t.Fatalf("first=%q want コミュニケーション能力", got[0])
	}
	if got[len(got)-1] != "技術志向" {
		t.Fatalf("last=%q want 技術志向", got[len(got)-1])
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
	got := s.selectFallbackQuestion("創造性・発想力", 0, "新卒", map[string]bool{})
	if got == "" {
		t.Fatal("expected creativity fallback")
	}
	options := s.fallbackQuestionsForCategory("創造性・発想力", 0, "新卒")
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
