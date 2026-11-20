package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/services/chat"
)

// SOCAIAGENT-632: LLM 障害中のセッションは、劣化した値を保存せず、計算版も現行にしない。
// 失敗（llm_degraded）として返すので、Advance が failed_sessions に数える。

type degradedChatMessages struct {
	repository.ChatMessageRepository
	messages []models.ChatMessage
}

func (r degradedChatMessages) FindBySessionIDForUser(string, uint) ([]models.ChatMessage, error) {
	return r.messages, nil
}

type degradedAIQuestions struct {
	repository.AIGeneratedQuestionRepository
	questions []models.AIGeneratedQuestion
}

func (r degradedAIQuestions) FindByUserAndSession(uint, string) ([]models.AIGeneratedQuestion, error) {
	return r.questions, nil
}

func TestRecalculateTarget_LLMDegradedは保存せず失敗に数える(t *testing.T) {
	// 評価として読めない応答で LLM の失敗を作る（エラー応答はクライアントが再試行して待たされる）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"評価できません"}}],"usage":{}}`))
	}))
	defer srv.Close()

	messages := degradedChatMessages{messages: []models.ChatMessage{
		{Role: "assistant", Content: "チームで協力した経験を教えてください"},
		{Role: "user", Content: "文化祭で協力しました。"},
	}}
	questions := degradedAIQuestions{questions: []models.AIGeneratedQuestion{
		{QuestionText: "チームで協力した経験を教えてください", Template: &models.AIQuestionTemplate{Category: "チームワーク志向"}},
	}}
	chatService := chat.NewChatService(openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"), nil, messages, nil, questions,
		nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// DB には何も期待しない。置換・計算版の更新・マッチングのどれかを呼べば sqlmock が失敗する
	db, mock := newRecalculationServiceSQLMock(t)
	matching := &spyRecalculationMatching{}
	service := &UserScoreRecalculationService{
		scores:   repositories.NewUserWeightScoreRepository(db),
		chat:     chatService,
		matching: matching,
	}
	target := repositories.UserScoreRecalculationTarget{ScoreID: 1, UserID: 7, SessionID: "session-1"}

	failure, err := service.recalculateTarget(context.Background(), 1, target, "v2")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if failure != models.UserScoreRecalculationFailureLLMDegraded {
		t.Fatalf("failure = %q, want %q（失敗に数える）", failure, models.UserScoreRecalculationFailureLLMDegraded)
	}
	if matching.calls != 0 {
		t.Fatalf("マッチングを作り直さない: calls = %d", matching.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("DB に書いていないこと（スコアも計算版も変えない）: %v", err)
	}
}
