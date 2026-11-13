package chat

import (
	"context"
	"errors"
	"maps"
	"math"
	"testing"

	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"

	"gorm.io/gorm"
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

// parityScoreRepository はライブ採点の書き込みをメモリに保持する。
type parityScoreRepository struct {
	repository.UserWeightScoreRepository
	scores map[string]int
}

func (r *parityScoreRepository) SetScore(_ uint, _, category string, absoluteScore int) error {
	r.scores[category] = max(0, min(100, absoluteScore))
	return nil
}

func (r *parityScoreRepository) AddScore(_ uint, _, category string, delta int) error {
	r.scores[category] = max(0, min(100, r.scores[category]+delta))
	return nil
}

// ライブ採点は評価に使ったプロンプト版も一緒に保存する（#1778）。スコアの突き合わせでは版は見ない。
func (r *parityScoreRepository) SetScoreWithPromptVersion(userID uint, sessionID, category string, absoluteScore int, _ string) error {
	return r.SetScore(userID, sessionID, category, absoluteScore)
}

func (r *parityScoreRepository) AddScoreWithPromptVersion(userID uint, sessionID, category string, delta int, _ string) error {
	return r.AddScore(userID, sessionID, category, delta)
}

func (r *parityScoreRepository) FindByUserSessionAndCategory(_ uint, _, category string) (*entity.UserWeightScore, error) {
	score, ok := r.scores[category]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &entity.UserWeightScore{WeightCategory: category, Score: score}, nil
}

type parityAIQuestionRepository struct {
	repository.AIGeneratedQuestionRepository
}

func (parityAIQuestionRepository) FindByUserAndSession(uint, string) ([]models.AIGeneratedQuestion, error) {
	return nil, nil
}

// ライブ採点と再計算は別の実装なので、同じ会話で同じスコアになることを突き合わせる。
// 期待値はライブ側の実行結果から作るので、どちらか片方だけを直すとここで落ちる。
func TestRecalculateChatSession_MatchesLiveScoring(t *testing.T) {
	const choiceQuestion = "チームでの役割を教えてください\nA) まとめ役\nB) 調整役\nC) 実務担当\nD) 分析担当"
	tests := []struct {
		name     string
		messages []models.ChatMessage
	}{
		{
			name: "文章回答の移動平均",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: "チームで協力した経験を教えてください", WeightCategory: "チームワーク志向"},
				{Role: "user", Content: "チームで協力して開発し、課題を解決しました。"},
				{Role: "assistant", Content: "その経験で工夫したことを教えてください", WeightCategory: "チームワーク志向"},
				{Role: "user", Content: "メンバーと話し合い、役割を分けて最後までやり遂げました。"},
				{Role: "assistant", Content: "さらに工夫した点はありますか", WeightCategory: "チームワーク志向"},
				{Role: "user", Content: "進捗を毎日共有し、遅れている人を全員で手伝うようにしました。"},
			},
		},
		{
			name: "複数の軸に分かれる回答",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: "技術を学んだ経験を教えてください", WeightCategory: "技術志向"},
				{Role: "user", Content: "Go でサーバーを作り、性能を測定して改善しました。"},
				{Role: "assistant", Content: "挑戦した経験を教えてください", WeightCategory: "チャレンジ志向"},
				{Role: "user", Content: "未経験の分野のコンテストに参加して入賞しました。"},
			},
		},
		{
			name: "短い文章回答",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: "挑戦した経験を教えてください", WeightCategory: "チャレンジ志向"},
				{Role: "user", Content: "大会に出ました"},
			},
		},
		{
			name: "スキップは採点しない",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: "技術について教えてください", WeightCategory: "技術志向"},
				{Role: "user", Content: "わかりません"},
			},
		},
		{
			name: "選択肢の記号",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: choiceQuestion, WeightCategory: "チームワーク志向"},
				{Role: "user", Content: "A"},
			},
		},
		{
			name: "選択肢の記号と理由",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: choiceQuestion, WeightCategory: "チームワーク志向"},
				{Role: "user", Content: "B: 意見が割れたときに間に入って話し合いを進めたからです"},
			},
		},
		{
			name: "選択肢の本文",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: choiceQuestion, WeightCategory: "チームワーク志向"},
				{Role: "user", Content: "調整役"},
			},
		},
		{
			name: "選択肢に当たらない自由記述",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: choiceQuestion, WeightCategory: "チームワーク志向"},
				{Role: "user", Content: "状況に応じて役割を変え、足りない部分を補っていました。"},
			},
		},
		{
			name: "選択肢と文章を混ぜた移動平均",
			messages: []models.ChatMessage{
				{Role: "assistant", Content: choiceQuestion, WeightCategory: "チームワーク志向"},
				{Role: "user", Content: "C"},
				{Role: "assistant", Content: "その役割で工夫したことを教えてください", WeightCategory: "チームワーク志向"},
				{Role: "user", Content: "メンバーと話し合い、役割を分けて最後までやり遂げました。"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			live := &parityScoreRepository{scores: map[string]int{}}
			liveService := NewChatService(nil, nil, nil, live, parityAIQuestionRepository{},
				nil, nil, nil, nil, nil, nil, nil, nil, nil)
			for i, message := range tt.messages {
				if message.Role == "user" {
					// ライブでは職種を選んだ後に採点するので jobCategoryID を入れる。
					liveService.scoreAnswer(context.Background(), 7, "session-1", message.Content, 1, tt.messages[:i])
				}
			}

			service := NewChatService(nil, nil, recalculationChatMessageRepository{messages: tt.messages},
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			got, err := service.RecalculateChatSession(context.Background(), 7, "session-1")
			if err != nil {
				t.Fatalf("RecalculateChatSession returned error: %v", err)
			}
			if !maps.Equal(got, live.scores) {
				t.Fatalf("recalculated = %v, live = %v", got, live.scores)
			}
		})
	}
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

// #1770 B2: 保存軸が無い過去の質問は、ライブ採点と同じく AI 生成の質問のテンプレートから軸を引く。
// それでも決まらなければ推測せず、セッションごと再計算しない（推測が外れると置換で元の軸のスコアが消える）。
type templateAIQuestionRepository struct {
	parityAIQuestionRepository
	questions []models.AIGeneratedQuestion
}

func (r templateAIQuestionRepository) FindByUserAndSession(uint, string) ([]models.AIGeneratedQuestion, error) {
	return r.questions, nil
}

func TestRecalculateChatSession_AxisFromTemplateOrUnknown(t *testing.T) {
	messages := []models.ChatMessage{
		{Role: "assistant", Content: "チームで協力した経験を教えてください"},
		{Role: "user", Content: "文化祭の準備でメンバーと協力し、期限に間に合わせました。"},
	}
	withTemplate := templateAIQuestionRepository{questions: []models.AIGeneratedQuestion{
		{QuestionText: "チームで協力した経験を教えてください", Template: &models.AIQuestionTemplate{Category: "リーダーシップ志向"}},
	}}
	service := NewChatService(nil, nil, recalculationChatMessageRepository{messages: messages},
		nil, withTemplate, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	got, err := service.RecalculateChatSession(context.Background(), 7, "session-1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, ok := got["リーダーシップ志向"]; !ok || len(got) != 1 {
		t.Fatalf("テンプレートの軸に載せること: %v", got)
	}

	service = NewChatService(nil, nil, recalculationChatMessageRepository{messages: messages},
		nil, parityAIQuestionRepository{}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if _, err := service.RecalculateChatSession(context.Background(), 7, "session-1"); !errors.Is(err, ErrRecalculationAxisUnknown) {
		t.Fatalf("軸が決まらなければ ErrRecalculationAxisUnknown: err = %v", err)
	}
}
