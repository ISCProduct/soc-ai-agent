package services

import (
	"Backend/internal/models"
	"testing"
)

// チャット主要機能の回帰ケース（職種誤判定・無効回答連鎖）。
// 本番で致命傷になる経路を重点的に固定する。

func TestIsJobSelectionQuestion_RegressionInterviewMCQ(t *testing.T) {
	t.Parallel()
	s := &ChatService{}

	interviewQ := "児童養護施設向けに「ITに不慣れな職員でも使いやすいUI」を考えたのは素敵だと思います。" +
		"正直に聞きたいのですが、その方向性で作るときに一番モヤっとした／妥協せざるを得なかったのはどれですか？" +
		"（一番近いものを選んでください）\n" +
		"A) 要件が曖昧でスコープが膨らむ\n" +
		"B) 技術制約でやりたいUIが出せない\n" +
		"C) ステークホルダー調整が難しい"

	if s.isJobSelectionQuestion(interviewQ) {
		t.Fatal("面接の経験談MCQを職種選択と誤判定してはいけない")
	}
}

func TestShouldValidateJobCategory_RegressionChoiceANotJob(t *testing.T) {
	t.Parallel()
	s := &ChatService{}
	history := []models.ChatMessage{
		{Role: "assistant", Content: "これまでの経験を具体的に教えてください。"},
		{Role: "user", Content: "児童養護施設向けのUIを作りました"},
		{
			Role: "assistant",
			Content: "一番モヤっとしたのはどれですか？（一番近いものを選んでください）\n" +
				"A) 要件が曖昧\nB) 技術制約\nC) 調整が難しい",
		},
		{Role: "user", Content: "A"},
	}
	if s.shouldValidateJobCategory(history) {
		t.Fatal("経験談MCQへの「A」回答で職種判定へ再突入してはいけない")
	}
}

func TestShouldValidateJobCategory_TrueForRealJobQuestion(t *testing.T) {
	t.Parallel()
	s := &ChatService{}
	history := []models.ChatMessage{
		{Role: "assistant", Content: "どの職種に興味がありますか？\n1. エンジニア\n2. 営業\n3. まだ決めていない"},
		{Role: "user", Content: "1"},
	}
	if !s.shouldValidateJobCategory(history) {
		t.Fatal("本物の職種質問では職種判定が必要")
	}
}

func TestFindLastAssistantQuestion_AfterTwoWarnings(t *testing.T) {
	t.Parallel()
	q := "チームでの役割について具体的に教えてください。"
	history := []models.ChatMessage{
		{Role: "assistant", Content: q},
		{Role: "user", Content: "あ"},
		{Role: "assistant", Content: "書かれた内容にはお答えできません。質問に回答してください。（1/3回目の警告）"},
		{Role: "user", Content: "あああ"},
		{Role: "assistant", Content: "書かれた内容にはお答えできません。質問に回答してください。（2/3回目の警告）"},
	}
	if got := findLastAssistantQuestion(history); got != q {
		t.Fatalf("got %q want %q", got, q)
	}
}

func TestIsLikelyAnswer_RejectsGarbageAcceptsRetry(t *testing.T) {
	t.Parallel()
	q := "これまでの経験で印象に残っていることを具体的に教えてください。"

	if isLikelyAnswer("あ", q) {
		t.Fatal("極短入力は無効であるべき")
	}
	if isLikelyAnswer("あああああ", q) {
		t.Fatal("連続同一文字は無効であるべき")
	}

	retry := "その経験は少ないですが児童養護施設の案件をやっていて思ったのはIT弱者の職員が使いやすいUIを作成することがいい経験になると思います"
	if !isLikelyAnswer(retry, q) {
		t.Fatal("十分な再回答は有効であるべき")
	}
}

func TestIsJobSelectionQuestion_SelectionCueNeedsContext(t *testing.T) {
	t.Parallel()
	s := &ChatService{}
	cases := []struct {
		text string
		want bool
	}{
		{"一番近いものを選んでください", false},
		{"以下から選んでください", false},
		{"どれが近いですか？", false},
		{"以下の職種から選んでください", true},
		{"どれが近いですか？\n1. エンジニア\n2. 営業", true},
		{"まだ決めていない場合、どんな作業が好きですか？", true},
	}
	for _, tc := range cases {
		if got := s.isJobSelectionQuestion(tc.text); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.text, got, tc.want)
		}
	}
}
