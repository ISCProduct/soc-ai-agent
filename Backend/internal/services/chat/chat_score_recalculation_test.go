package chat

import (
	"context"
	"math"
	"testing"

	"Backend/internal/models"
)

type recalculationChatMessageRepository struct {
	messages []models.ChatMessage
}

func (r recalculationChatMessageRepository) Create(*models.ChatMessage) error { return nil }
func (r recalculationChatMessageRepository) FindBySessionIDForUser(string, uint) ([]models.ChatMessage, error) {
	return r.messages, nil
}
func (r recalculationChatMessageRepository) FindRecentBySessionIDForUser(string, uint, int) ([]models.ChatMessage, error) {
	return nil, nil
}
func (r recalculationChatMessageRepository) ExistsBySessionIDForOtherUser(string, uint) (bool, error) {
	return false, nil
}
func (r recalculationChatMessageRepository) FindByUserID(uint) ([]models.ChatMessage, error) {
	return nil, nil
}
func (r recalculationChatMessageRepository) GetUserSessions(uint) ([]models.ChatSession, error) {
	return nil, nil
}

func TestRecalculateChatSession_ReplaysAnswersWithMovingAverage(t *testing.T) {
	messages := []models.ChatMessage{
		{Role: "assistant", Content: "チームで協力した経験を教えてください", WeightCategory: "チームワーク志向"},
		{Role: "user", Content: "チームで協力して開発し、課題を解決しました。"},
		{Role: "assistant", Content: "その経験で工夫したことを教えてください", WeightCategory: "チームワーク志向"},
		{Role: "user", Content: "メンバーと話し合い、役割を分けて最後までやり遂げました。"},
	}
	service := NewChatService(nil, nil, recalculationChatMessageRepository{messages: messages},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	first := service.answerEvaluator.EvaluateHumanScoringWithContext(context.Background(),
		messages[0].Content, messages[1].Content, false, false, nil)
	second := service.answerEvaluator.EvaluateHumanScoringWithContext(context.Background(),
		messages[2].Content, messages[3].Content, false, false, nil)
	want := int(math.Round(float64(first.Score)*0.7 + float64(second.Score)*0.3))

	got, err := service.RecalculateChatSession(context.Background(), 7, "session-1")
	if err != nil {
		t.Fatalf("RecalculateChatSession returned error: %v", err)
	}
	if got["チームワーク志向"] != want {
		t.Fatalf("recalculated score = %d, want moving average %d", got["チームワーク志向"], want)
	}
}

func TestRecalculateChatSession_IgnoresSkipAndMissingQuestion(t *testing.T) {
	messages := []models.ChatMessage{
		{Role: "user", Content: "回答"},
		{Role: "assistant", Content: "技術について教えてください", WeightCategory: "技術志向"},
		{Role: "user", Content: "わかりません"},
	}
	service := NewChatService(nil, nil, recalculationChatMessageRepository{messages: messages},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	got, err := service.RecalculateChatSession(context.Background(), 7, "session-1")
	if err != nil {
		t.Fatalf("RecalculateChatSession returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("recalculated scores = %v, want no scored categories", got)
	}
}

func TestRecalculateChatSession_IgnoresAnswersFollowedByValidationFeedback(t *testing.T) {
	question := "チームで協力した経験を教えてください"
	validAnswer := "チームで協力して開発し、課題を解決しました。"
	messages := []models.ChatMessage{
		{Role: "assistant", Content: question, WeightCategory: "チームワーク志向"},
		{Role: "user", Content: "今日はいい天気ですね"},
		{Role: "assistant", Content: validationRetryMessage(1, 3)},
		{Role: "user", Content: validAnswer},
	}
	service := NewChatService(nil, nil, recalculationChatMessageRepository{messages: messages},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	want := service.answerEvaluator.EvaluateHumanScoringWithContext(
		context.Background(), question, validAnswer, false, false, nil,
	).Score

	got, err := service.RecalculateChatSession(context.Background(), 7, "session-1")
	if err != nil {
		t.Fatalf("RecalculateChatSession returned error: %v", err)
	}
	if got["チームワーク志向"] != want {
		t.Fatalf("recalculated score = %d, want valid answer score %d", got["チームワーク志向"], want)
	}
}
