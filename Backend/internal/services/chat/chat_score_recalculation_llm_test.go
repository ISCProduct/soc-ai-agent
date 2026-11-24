package chat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
)

// SOCAIAGENT-632: LLM で評価すべき回答があるのに LLM が使えなかった再計算は、ルール採点だけの値で
// 確定させない（呼び出し側は保存せず、計算版も進めず、llm_degraded として失敗に数える）。
// LLM クライアントが未設定の構成は、今までどおりルール採点だけで成功にする。
func TestRecalculateChatSession_LLMUnavailable(t *testing.T) {
	// 短い自由回答は、ルール採点と LLM を混ぜる対象（shouldBlendLLMForHumanScore）
	messages := []models.ChatMessage{
		{Role: "assistant", Content: "チームで協力した経験を教えてください"},
		{Role: "user", Content: "文化祭で協力しました。"},
	}
	questions := templateAIQuestionRepository{questions: []models.AIGeneratedQuestion{
		{QuestionText: "チームで協力した経験を教えてください", Template: &models.AIQuestionTemplate{Category: "チームワーク志向"}},
	}}

	t.Run("LLM が失敗したら ErrRecalculationLLMUnavailable を返し、値を返さない", func(t *testing.T) {
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			// 評価として読めない応答（JSON でない）。エラー応答はクライアントが再試行して待たされるので、
			// 1回で返る「使えない応答」で LLM の失敗を作る
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"評価できません"}}],"usage":{}}`))
		}))
		defer srv.Close()
		service := NewChatService(openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"), nil, recalculationChatMessageRepository{messages: messages},
			nil, questions, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		got, err := service.RecalculateChatSession(context.Background(), 7, "session-1")
		if !errors.Is(err, ErrRecalculationLLMUnavailable) {
			t.Fatalf("err = %v, want ErrRecalculationLLMUnavailable", err)
		}
		if got != nil {
			t.Fatalf("劣化した値を返さない: %v", got)
		}
		if calls == 0 {
			t.Fatal("LLM を呼んでいない（混ぜる対象の回答になっていない）")
		}
	})

	t.Run("LLM が未設定ならルール採点だけで成功にする", func(t *testing.T) {
		service := NewChatService(nil, nil, recalculationChatMessageRepository{messages: messages},
			nil, questions, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		got, err := service.RecalculateChatSession(context.Background(), 7, "session-1")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if _, ok := got["チームワーク志向"]; !ok {
			t.Fatalf("ルール採点の値が入ること: %v", got)
		}
	})
}
