package services

import (
	"Backend/internal/models"
	"testing"
)

func TestIsValidationFeedbackMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "warning", content: "書かれた内容にはお答えできません。質問に回答してください。（1/3回目の警告）", want: true},
		{name: "terminate", content: "申し訳ございませんが、質問と関係のない内容が3回続いたため、チャットを終了させていただきます。", want: true},
		{name: "normal question", content: "チームでの経験について具体的に教えてください。", want: false},
		{name: "empty", content: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isValidationFeedbackMessage(tc.content); got != tc.want {
				t.Fatalf("isValidationFeedbackMessage(%q)=%v want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestFindLastAssistantQuestion_SkipsValidationFeedback(t *testing.T) {
	t.Parallel()
	realQuestion := "その経験について、具体的なエピソードを教えてください。"
	history := []models.ChatMessage{
		{Role: "assistant", Content: realQuestion},
		{Role: "user", Content: "その経験は少ないですが、やはりIT弱者の職員が使いやすいUIwo"},
		{Role: "assistant", Content: "書かれた内容にはお答えできません。質問に回答してください。（1/3回目の警告）"},
	}

	got := findLastAssistantQuestion(history)
	if got != realQuestion {
		t.Fatalf("findLastAssistantQuestion()=%q want %q", got, realQuestion)
	}
}

func TestFindLastAssistantQuestion_EmptyWhenOnlyFeedback(t *testing.T) {
	t.Parallel()
	history := []models.ChatMessage{
		{Role: "assistant", Content: "書かれた内容にはお答えできません。質問に回答してください。（2/3回目の警告）"},
	}
	if got := findLastAssistantQuestion(history); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestIsLikelyAnswer_AcceptsEpisodeAboutUI(t *testing.T) {
	t.Parallel()
	question := "これまでの経験で印象に残っていることを具体的に教えてください。"
	answer := "その経験は少ないですが児童養護施設の案件をやっていて思ったのはIT弱者の職員が使いやすいUIを作成することがいい経験になると思います"
	if !isLikelyAnswer(answer, question) {
		t.Fatalf("expected valid answer for substantial episode text")
	}
}
