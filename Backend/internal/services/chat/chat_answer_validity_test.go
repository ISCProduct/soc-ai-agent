package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
)

// checkAnswerValidity / validateAnswerRelevance の振る舞いを固定するテスト。
// 回答が無効のときの回数の数え方・打ち切り、埋め込みとフォールバックの使い分けは、
// 学生に見える挙動なので、関数を分けても変えない。

const (
	textQuestion   = "これまでの経験で印象に残っていることを具体的に教えてください。"
	choiceQuestion = "どれに興味がありますか？\nA) 開発\nB) インフラ\nC) その他（自由記述）"
)

// ---- checkAnswerValidity ----

type fakeValidationRepo struct {
	repository.SessionValidationRepository
	count         int // IncrementInvalidCount が返す回数の起点
	incrementErr  error
	incremented   int
	reset         int
	terminated    int
	terminatedIDs []string
}

func (r *fakeValidationRepo) IncrementInvalidCount(string) (*models.SessionValidation, error) {
	if r.incrementErr != nil {
		return nil, r.incrementErr
	}
	r.incremented++
	r.count++
	return &models.SessionValidation{InvalidAnswerCount: r.count}, nil
}
func (r *fakeValidationRepo) ResetInvalidCount(string) error { r.reset++; return nil }
func (r *fakeValidationRepo) TerminateSession(id string) error {
	r.terminated++
	r.terminatedIDs = append(r.terminatedIDs, id)
	return nil
}

type fakeChatRepo struct {
	repository.ChatMessageRepository
	createErr error
	created   []*models.ChatMessage
}

func (r *fakeChatRepo) Create(m *models.ChatMessage) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, m)
	return nil
}

func newValidityService(v *fakeValidationRepo, c *fakeChatRepo) *ChatService {
	// aiClient は nil: 文章系の質問は isLikelyAnswer で判定される(AI を呼ばない)。
	return &ChatService{sessionValidationRepo: v, chatMessageRepo: c}
}

func assistantQuestion(content string) []models.ChatMessage {
	return []models.ChatMessage{{Role: "assistant", Content: content}}
}

func TestCheckAnswerValidity_ValidAnswerResetsCount(t *testing.T) {
	v, c := &fakeValidationRepo{count: 2}, &fakeChatRepo{}
	s := newValidityService(v, c)

	handled, text, err := s.checkAnswerValidity(context.Background(), assistantQuestion(textQuestion),
		"その経験は少ないですが児童養護施設の案件を担当しました", 1, "s1")

	if err != nil || handled || text != "" {
		t.Fatalf("handled=%v text=%q err=%v, want 通常の処理へ進む", handled, text, err)
	}
	if v.reset != 1 || v.incremented != 0 {
		t.Fatalf("reset=%d incremented=%d, want reset=1 incremented=0", v.reset, v.incremented)
	}
	if len(c.created) != 0 {
		t.Fatal("有効な回答でアシスタントの案内を保存してはいけない")
	}
}

func TestCheckAnswerValidity_InvalidAnswerWarnsThenTerminatesOnThird(t *testing.T) {
	v, c := &fakeValidationRepo{}, &fakeChatRepo{}
	s := newValidityService(v, c)
	hist := assistantQuestion(textQuestion)

	// 1回目・2回目は警告。2回目は終了の予告つき
	for i, wantWarn := range []string{"", "次も同じだと、このチャットは一度終了します"} {
		handled, text, err := s.checkAnswerValidity(context.Background(), hist, "あ", 7, "s1")
		if err != nil || !handled {
			t.Fatalf("%d回目: handled=%v err=%v", i+1, handled, err)
		}
		if text != validationRetryMessage(i+1, 3) {
			t.Fatalf("%d回目: text=%q", i+1, text)
		}
		if wantWarn != "" && !strings.Contains(text, wantWarn) {
			t.Fatalf("%d回目に終了の予告が無い: %q", i+1, text)
		}
	}
	if v.terminated != 0 {
		t.Fatal("3回目より前に終了してはいけない")
	}

	// 3回目で強制終了
	handled, text, err := s.checkAnswerValidity(context.Background(), hist, "あ", 7, "s1")
	if err != nil || !handled || text != validationTerminatedMessage() {
		t.Fatalf("3回目: handled=%v text=%q err=%v", handled, text, err)
	}
	if v.terminated != 1 || v.terminatedIDs[0] != "s1" {
		t.Fatalf("terminated=%d ids=%v", v.terminated, v.terminatedIDs)
	}

	// 保存したアシスタント応答は、セッションとユーザーに紐づく
	if len(c.created) != 3 {
		t.Fatalf("保存件数 = %d, want 3", len(c.created))
	}
	for _, m := range c.created {
		if m.Role != "assistant" || m.SessionID != "s1" || m.UserID != 7 {
			t.Fatalf("保存内容 = %+v", m)
		}
	}
}

func TestCheckAnswerValidity_EmptyOtherChoiceDoesNotCount(t *testing.T) {
	v, c := &fakeValidationRepo{count: 1}, &fakeChatRepo{}
	s := newValidityService(v, c)

	// 「その他」を記号だけで選んだ: 数えず、書き方だけ伝える
	handled, text, err := s.checkAnswerValidity(context.Background(), assistantQuestion(choiceQuestion), "C", 1, "s1")

	if err != nil || !handled || text != validationOtherNeedsTextMessage() {
		t.Fatalf("handled=%v text=%q err=%v", handled, text, err)
	}
	if v.incremented != 0 || v.reset != 0 || v.terminated != 0 {
		t.Fatalf("回数に触れてはいけない: inc=%d reset=%d term=%d", v.incremented, v.reset, v.terminated)
	}
	if len(c.created) != 1 || c.created[0].Content != text {
		t.Fatalf("保存 = %+v", c.created)
	}
}

func TestCheckAnswerValidity_NoQuestionInHistoryExpectsJobChoice(t *testing.T) {
	// 履歴に質問が無い初回は、職種選択の質問を前提にする。単語の列挙も許容される。
	v, c := &fakeValidationRepo{}, &fakeChatRepo{}
	s := newValidityService(v, c)

	handled, _, err := s.checkAnswerValidity(context.Background(), nil, "開発 インフラ", 1, "s1")
	if err != nil || handled {
		t.Fatalf("handled=%v err=%v, want 有効", handled, err)
	}
}

func TestCheckAnswerValidity_Errors(t *testing.T) {
	t.Run("回数の加算に失敗", func(t *testing.T) {
		v := &fakeValidationRepo{incrementErr: errors.New("db down")}
		s := newValidityService(v, &fakeChatRepo{})
		handled, text, err := s.checkAnswerValidity(context.Background(), assistantQuestion(textQuestion), "あ", 1, "s1")
		if !handled || text != "" || err == nil || !strings.Contains(err.Error(), "failed to increment invalid count") {
			t.Fatalf("handled=%v text=%q err=%v", handled, text, err)
		}
	})
	t.Run("無効回答の案内の保存に失敗", func(t *testing.T) {
		s := newValidityService(&fakeValidationRepo{}, &fakeChatRepo{createErr: errors.New("db down")})
		handled, text, err := s.checkAnswerValidity(context.Background(), assistantQuestion(textQuestion), "あ", 1, "s1")
		if !handled || text != "" || err == nil || !strings.Contains(err.Error(), "failed to save assistant message for invalid answer") {
			t.Fatalf("handled=%v text=%q err=%v", handled, text, err)
		}
	})
	t.Run("「その他」の案内の保存に失敗", func(t *testing.T) {
		s := newValidityService(&fakeValidationRepo{}, &fakeChatRepo{createErr: errors.New("db down")})
		handled, text, err := s.checkAnswerValidity(context.Background(), assistantQuestion(choiceQuestion), "C", 1, "s1")
		if !handled || text != "" || err == nil || !strings.Contains(err.Error(), "failed to save assistant message for empty other choice") {
			t.Fatalf("handled=%v text=%q err=%v", handled, text, err)
		}
	})
}

// ---- validateAnswerRelevance ----

// relevanceServer は /embeddings と /responses を返すテスト用サーバー。
// embeddings は呼び出し順に vectors を返す(尽きたら最後の値)。
func relevanceServer(t *testing.T, vectors [][]float32, embeddingStatus int, responsesBody string, responsesCalls *atomic.Int32) *openai.Client {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/embeddings"):
			if embeddingStatus != 0 && embeddingStatus != http.StatusOK {
				http.Error(w, `{"error":{"message":"boom"}}`, embeddingStatus)
				return
			}
			i := int(n.Add(1)) - 1
			if i >= len(vectors) {
				i = len(vectors) - 1
			}
			fmt.Fprintf(w, `{"data":[{"embedding":%s}],"usage":{"prompt_tokens":3,"total_tokens":3}}`, vectorJSON(vectors[i]))
		case strings.HasSuffix(r.URL.Path, "/responses"):
			if responsesCalls != nil {
				responsesCalls.Add(1)
			}
			fmt.Fprint(w, responsesBody)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
}

func vectorJSON(v []float32) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = fmt.Sprintf("%g", x)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

const goodEpisode = "その経験は少ないですが児童養護施設の案件を担当しました"

func TestValidateAnswerRelevance_TextQuestion(t *testing.T) {
	tests := []struct {
		name    string
		client  func(t *testing.T) *openai.Client // nil なら aiClient 無し
		answer  string
		want    bool
		wantErr bool
	}{
		{"AI 無し: 実質的な回答は受理", nil, goodEpisode, true, false},
		{"AI 無し: 無意味な回答は却下", nil, "あ", false, false},
		{"類似度が閾値以上なら受理(短い回答でも)", func(t *testing.T) *openai.Client {
			return relevanceServer(t, [][]float32{{1, 0}, {1, 0}}, 0, "", nil)
		}, "あ", true, false},
		{"類似度 0.7 は閾値(0.30)以上なので、短い回答でも受理", func(t *testing.T) *openai.Client {
			return relevanceServer(t, [][]float32{{1, 0}, {1, 1}}, 0, "", nil)
		}, "あ", true, false},
		{"類似度 0.2 は閾値未満なので、無意味な回答は却下", func(t *testing.T) *openai.Client {
			return relevanceServer(t, [][]float32{{1, 0}, {0.2, 1}}, 0, "", nil)
		}, "あ", false, false},
		{"閾値未満でも実質的な回答なら受理(観点ズレを落とさない)", func(t *testing.T) *openai.Client {
			return relevanceServer(t, [][]float32{{1, 0}, {0, 1}}, 0, "", nil)
		}, goodEpisode, true, false},
		{"閾値未満で無意味な回答は却下", func(t *testing.T) *openai.Client {
			return relevanceServer(t, [][]float32{{1, 0}, {0, 1}}, 0, "", nil)
		}, "あ", false, false},
		{"埋め込みに失敗したら isLikelyAnswer に倒す(受理)", func(t *testing.T) *openai.Client {
			return relevanceServer(t, [][]float32{{1, 0}}, http.StatusBadRequest, "", nil)
		}, goodEpisode, true, false},
		{"埋め込みに失敗したら isLikelyAnswer に倒す(却下)", func(t *testing.T) *openai.Client {
			return relevanceServer(t, [][]float32{{1, 0}}, http.StatusBadRequest, "", nil)
		}, "あ", false, false},
		{"ゼロベクトルなら isLikelyAnswer に倒す", func(t *testing.T) *openai.Client {
			return relevanceServer(t, [][]float32{{0, 0}, {0, 0}}, 0, "", nil)
		}, goodEpisode, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChatService{}
			if tt.client != nil {
				s.aiClient = tt.client(t)
			}
			got, err := s.validateAnswerRelevance(context.Background(), textQuestion, tt.answer)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("got=%v err=%v, want got=%v wantErr=%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestValidateAnswerRelevance_ChoiceQuestion(t *testing.T) {
	var calls atomic.Int32
	verdict := func(body string) func(t *testing.T) *openai.Client {
		return func(t *testing.T) *openai.Client {
			return relevanceServer(t, [][]float32{{1}}, 0, `{"output_text":`+quoteJSON(body)+`,"usage":{"input_tokens":1,"output_tokens":1}}`, &calls)
		}
	}
	tests := []struct {
		name      string
		client    func(t *testing.T) *openai.Client
		answer    string
		want      bool
		wantErr   bool
		wantCalls int32 // AI(/responses) を呼んだ回数
	}{
		{"記号はローカルで受理(AI を呼ばない)", verdict(`{"valid":false}`), "A", true, false, 0},
		{"その他経路: 2文字以上の自由記述は受理", verdict(`{"valid":false}`), "AI に興味があります", true, false, 0},
		{"1文字の記号外は AI 判定(valid=true)", verdict(`{"valid":true}`), "あ", true, false, 1},
		{"AI 判定(valid=false)", verdict(`{"valid":false}`), "あ", false, false, 1},
		{"コードフェンスつきの JSON も読める", verdict("```json\n{\"valid\":true}\n```"), "あ", true, false, 1},
		{"JSON として読めなければ無効", verdict("valid です"), "あ", false, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls.Store(0)
			s := &ChatService{aiClient: tt.client(t)}
			got, err := s.validateAnswerRelevance(context.Background(), choiceQuestion, tt.answer)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("got=%v err=%v, want got=%v wantErr=%v", got, err, tt.want, tt.wantErr)
			}
			if calls.Load() != tt.wantCalls {
				t.Fatalf("AI 呼び出し = %d, want %d", calls.Load(), tt.wantCalls)
			}
		})
	}
}

func TestValidateAnswerRelevance_ChoiceQuestionAIErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad request"}}`, http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)
	s := &ChatService{aiClient: openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")}

	got, err := s.validateAnswerRelevance(context.Background(), choiceQuestion, "あ")
	if got || err == nil || !strings.Contains(err.Error(), "AI validation error") {
		t.Fatalf("got=%v err=%v, want AI 判定の失敗を返す", got, err)
	}
}

func quoteJSON(s string) string {
	b, _ := jsonMarshalString(s)
	return b
}

func jsonMarshalString(s string) (string, error) {
	b, err := json.Marshal(s)
	return string(b), err
}

// ---- 切り出した純粋関数 ----

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name   string
		a, b   []float32
		want   float64
		wantOK bool
	}{
		{"同じ向きは1", []float32{1, 2, 3}, []float32{2, 4, 6}, 1, true},
		{"直交は0", []float32{1, 0}, []float32{0, 1}, 0, true},
		{"逆向きは-1", []float32{1, 0}, []float32{-1, 0}, -1, true},
		{"一方がゼロベクトルなら使えない", []float32{0, 0}, []float32{1, 1}, 0, false},
		{"空なら使えない", nil, nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := cosineSimilarity(tt.a, tt.b)
			if ok != tt.wantOK || (ok && (got-tt.want > 1e-9 || tt.want-got > 1e-9)) {
				t.Fatalf("cosineSimilarity() = %v, %v, want %v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestParseAnswerValidity(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{`{"valid":true}`, true},
		{`{"valid":false}`, false},
		{"```json\n{\"valid\":true}\n```", true},
		{"```\n{\"valid\":true}\n```", true},
		{"  {\"valid\": true}  ", true},
		{`{}`, false},
		{`valid`, false},
		{``, false},
	}
	for _, tt := range tests {
		if got := parseAnswerValidity(tt.in); got != tt.want {
			t.Errorf("parseAnswerValidity(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
