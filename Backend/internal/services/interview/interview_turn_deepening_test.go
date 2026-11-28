package interview

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
)

// 深掘りのターンでも LLM は面接官の応答の1回だけ呼ぶ（追質問を別に作ると応答が1往復遅れる）。
// 保存する追質問の文は、定型文ではなく実際に投げかけた文にする。
func TestTurn_DeepeningUsesSingleLLMCall(t *testing.T) {
	var chatCalls, otherCalls atomic.Int32
	var chatBody atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio/transcriptions":
			_, _ = w.Write([]byte(`{"text":"山田太郎です"}`))
		case "/chat/completions":
			chatCalls.Add(1)
			b, _ := io.ReadAll(r.Body)
			chatBody.Store(string(b))
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"そのとき、具体的に何をしましたか？"}}]}`))
		case "/audio/speech":
			_, _ = w.Write([]byte("audio"))
		default:
			otherCalls.Add(1)
			_, _ = w.Write([]byte(`{"output_text":"別の追質問"}`))
		}
	}))
	defer server.Close()

	asked := &models.InterviewQuestionState{
		ID: 5, SessionID: 1, Source: questionSourceCustom,
		Category: "学生時代", QuestionText: "力を入れたことを教えてください", Status: questionStatusAsked,
	}
	repo := &fakeQuestionStateRepo{states: []models.InterviewQuestionState{*asked}, latest: asked}
	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress", Language: "ja"}),
		&utterRepoStub{}, nil, nil, nil, openai.NewWithBaseURL(server.URL, "gpt-4o-mini"), nil,
	)
	svc.questionStateRepo = repo

	res, err := svc.Turn(context.Background(), 10, 1, []byte("audio"), "", "", "", false, "", "", "", 0, 1, 0, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("Turn error: %v", err)
	}
	if !res.IsDeepening {
		t.Fatalf("深掘りのターンにならない: %+v", res)
	}
	if n := chatCalls.Load(); n != 1 {
		t.Errorf("面接官の応答の呼び出し = %d 回, want 1", n)
	}
	if n := otherCalls.Load(); n != 0 {
		t.Errorf("追質問のために別の LLM 呼び出しをした（%d 回）", n)
	}
	if len(repo.created) != 1 || repo.created[0].QuestionText != "そのとき、具体的に何をしましたか？" {
		t.Errorf("保存した追質問の文が実際の発話になっていない: %+v", repo.created)
	}
	// 学生の回答は履歴（伏字を通る）にだけ載せ、伏字の対象外のシステムプロンプトには入れない
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	_ = json.Unmarshal([]byte(chatBody.Load().(string)), &req)
	for _, m := range req.Messages {
		if m.Role == "system" && strings.Contains(m.Content, "山田") {
			t.Errorf("学生の回答がシステムプロンプトに入っている")
		}
	}
	if res.UtteranceSaveFailed {
		t.Errorf("保存に成功したのに失敗を知らせた")
	}
}

// 追質問の文の上書きに失敗したら、書き起こしの保存失敗と同じく呼び出し元へ知らせる。
func TestTurn_DeepeningReportsFollowUpSaveFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio/transcriptions":
			_, _ = w.Write([]byte(`{"text":"はい"}`))
		case "/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"具体的には？"}}]}`))
		default:
			_, _ = w.Write([]byte("audio"))
		}
	}))
	defer server.Close()

	asked := &models.InterviewQuestionState{ID: 5, SessionID: 1, Source: questionSourceCustom, QuestionText: "力を入れたこと", Status: questionStatusAsked}
	repo := &fakeQuestionStateRepo{states: []models.InterviewQuestionState{*asked}, latest: asked,
		failUpdate: func(s *models.InterviewQuestionState) error {
			if s.Source == questionSourceFollowUp {
				return errors.New("db down")
			}
			return nil
		}}
	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress", Language: "ja"}),
		&utterRepoStub{}, nil, nil, nil, openai.NewWithBaseURL(server.URL, "gpt-4o-mini"), nil,
	)
	svc.questionStateRepo = repo

	res, err := svc.Turn(context.Background(), 10, 1, []byte("audio"), "", "", "", false, "", "", "", 0, 1, 0, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("Turn error: %v", err)
	}
	if !res.UtteranceSaveFailed {
		t.Errorf("追質問の文の保存失敗を知らせていない")
	}
}

// 深掘りでは定型の質問文を読ませず、狙いと STAR の欠落を指示する。通常の質問はそのまま投げかけさせる。
func TestBuildInterviewSystemPrompt_DeepeningRephrasesTemplate(t *testing.T) {
	deep := buildInterviewSystemPrompt("テスト株式会社", "", "エンジニア", "", "general", nil, nil, 0, 0, 1, 5, 0, 180,
		&questionDirective{Text: "「山田太郎です」について…", IsDeepening: true, DeepeningOf: "力を入れたことを教えてください", DeepeningFocus: "「成果」を引き出す質問にしてください。"})
	if !containsAll(deep, []string{"深掘り質問を1問だけ", "力を入れたことを教えてください", "「成果」を引き出す質問にしてください。"}) || strings.Contains(deep, "そのまま面接官として") {
		t.Errorf("深掘りの指示が言い直しになっていない: %s", deep)
	}
	// 定型文は学生の回答の一部を含む。伏字の対象外のシステムプロンプトには入れない
	if strings.Contains(deep, "山田") {
		t.Errorf("深掘りの定型文（学生の回答を含む）がシステムプロンプトに入っている")
	}
	plain := buildInterviewSystemPrompt("テスト株式会社", "", "エンジニア", "", "general", nil, nil, 0, 0, 1, 5, 0, 180,
		&questionDirective{Text: "志望動機を教えてください"})
	if !strings.Contains(plain, "そのまま面接官として") {
		t.Errorf("通常の質問をそのまま投げかけさせていない: %s", plain)
	}
}
