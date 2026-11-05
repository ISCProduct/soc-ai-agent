package chat

import (
	"Backend/internal/models"
	"strings"
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

// 回答が質問に沿っていないときの案内文。
//
// 文面を変えるときは frontend 側の3箇所（utils.ts の isValidationFeedbackMessage、
// ChatMessageList.tsx の判定）も合わせること。目印が揃っていないと、
// 画面側が警告を「質問」として扱い選択肢の復元が壊れる。
func TestValidationRetryMessage(t *testing.T) {
	t.Run("1回目は終了を予告しない", func(t *testing.T) {
		got := validationRetryMessage(1, 3)
		if strings.Contains(got, "終了します") {
			t.Errorf("1回目で終了を予告している: %q", got)
		}
		if !strings.Contains(got, validationMarkerRetry) {
			t.Errorf("目印が入っていない: %q", got)
		}
	})

	t.Run("残り1回になったら事実として伝える", func(t *testing.T) {
		got := validationRetryMessage(2, 3)
		if !strings.Contains(got, "次も同じだと、このチャットは一度終了します") {
			t.Errorf("終了が近いことを伝えていない: %q", got)
		}
	})

	t.Run("学生を責める表現と回数の数え上げを含まない", func(t *testing.T) {
		// 「書かれた内容にはお答えできません」と断じる形と
		// 「（1/3回目の警告）」という数え上げは、指導や処分に読める。
		for _, count := range []int{1, 2} {
			got := validationRetryMessage(count, 3)
			for _, ng := range []string{"お答えできません", "回目の警告", "申し訳"} {
				if strings.Contains(got, ng) {
					t.Errorf("避けたい表現が残っている %q: %q", ng, got)
				}
			}
		}
	})

	t.Run("次にできることを示す", func(t *testing.T) {
		got := validationRetryMessage(1, 3)
		if !strings.Contains(got, "選択肢") {
			t.Errorf("次の手段を示していない: %q", got)
		}
	})
}

func TestValidationTerminatedMessage(t *testing.T) {
	got := validationTerminatedMessage()
	if !strings.Contains(got, validationMarkerTerminated) {
		t.Errorf("目印が入っていない: %q", got)
	}
	// 打ち切りは学生にとって重いので、やり直し方と相談先を必ず添える。
	if !strings.Contains(got, "やり直せます") {
		t.Errorf("やり直し方を示していない: %q", got)
	}
	if !strings.Contains(got, "先生") {
		t.Errorf("相談先を示していない: %q", got)
	}
}

func TestValidationFeedbackMessageAcceptsLegacyWording(t *testing.T) {
	// 既存セッションの DB には旧文言が残っている。落とすと過去の警告が
	// 「直近の質問」として拾われ、選択肢の復元が壊れる。
	legacy := []string{
		"書かれた内容にはお答えできません。質問に回答してください。（1/3回目の警告）",
		"申し訳ございませんが、質問と関係のない内容が3回続いたため、チャットを終了させていただきます。",
	}
	for _, msg := range legacy {
		if !isValidationFeedbackMessage(msg) {
			t.Errorf("旧文言を判定できていない: %q", msg)
		}
	}
}
