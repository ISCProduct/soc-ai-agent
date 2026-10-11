package interview

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
)

// #1636-2: 面接官へ渡す履歴は保存済みの発話から組み直し、クライアント値を使わない。
func TestTurn_BuildsHistoryFromSavedUtterances(t *testing.T) {
	var chatBody struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio/transcriptions":
			_, _ = w.Write([]byte(`{"text":"チームで開発しました"}`))
		case "/chat/completions":
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), `"messages"`) && chatBody.Messages == nil {
				_ = json.Unmarshal(b, &chatBody)
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"次の質問です"}}]}`))
		case "/audio/speech":
			_, _ = w.Write([]byte("audio"))
		default:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
		}
	}))
	defer server.Close()

	utters := &utterRepoStub{utterances: []models.InterviewUtterance{
		{SessionID: 1, Role: "ai", Text: "自己紹介をお願いします"},
		{SessionID: 1, Role: "user", Text: "山田です"},
	}}
	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress", Language: "ja"}),
		utters, nil, nil, nil, openai.NewWithBaseURL(server.URL, "gpt-4o-mini"), nil,
	)

	if _, err := svc.Turn(context.Background(), 10, 1, []byte("audio"), "", "", false, "", "", "", 0, 1, 0, 1, 1, 0, 0); err != nil {
		t.Fatalf("Turn error: %v", err)
	}

	var got []string
	for _, m := range chatBody.Messages {
		if m.Role != "system" {
			got = append(got, m.Role+":"+m.Content)
		}
	}
	want := []string{"assistant:自己紹介をお願いします", "user:山田です", "user:チームで開発しました"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("履歴 = %v, want %v", got, want)
	}
}

func TestHistoryFromUtterances_CapsCountAndLength(t *testing.T) {
	var utters []models.InterviewUtterance
	for i := range maxHistoryMessages + 10 {
		role := "user"
		if i%2 == 0 {
			role = "ai"
		}
		utters = append(utters, models.InterviewUtterance{Role: role, Text: strings.Repeat("あ", maxHistoryRunes+50)})
	}
	h := historyFromUtterances(utters)
	if len(h) != maxHistoryMessages {
		t.Fatalf("件数 = %d, want %d", len(h), maxHistoryMessages)
	}
	for _, m := range h {
		if n := len([]rune(m["content"])); n > maxHistoryRunes {
			t.Fatalf("1件の長さ = %d, want <= %d", n, maxHistoryRunes)
		}
		if m["role"] != "user" && m["role"] != "assistant" {
			t.Fatalf("role = %q", m["role"])
		}
	}
}

// #1636-3: 発話の直接投稿に長さ・件数の上限を置く。
func TestSaveUtterance_RejectsOverlongAndTooMany(t *testing.T) {
	repo := newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress"})

	t.Run("長すぎる発話は弾く", func(t *testing.T) {
		svc := NewInterviewService(repo, &utterRepoStub{}, nil, nil, nil, nil, nil)
		err := svc.SaveUtterance(10, 1, "user", strings.Repeat("あ", maxUtteranceRunes+1), "")
		if err != ErrUtteranceTooLong {
			t.Fatalf("err = %v, want ErrUtteranceTooLong", err)
		}
		if err := svc.SaveUtterance(10, 1, "user", strings.Repeat("あ", maxUtteranceRunes), ""); err != nil {
			t.Fatalf("上限ちょうどは保存できること: %v", err)
		}
	})

	t.Run("件数上限を超えたら弾く", func(t *testing.T) {
		full := make([]models.InterviewUtterance, maxUtterancesPerSession)
		svc := NewInterviewService(repo, &utterRepoStub{utterances: full}, nil, nil, nil, nil, nil)
		if err := svc.SaveUtterance(10, 1, "user", "こんにちは", ""); err != ErrTooManyUtterances {
			t.Fatalf("err = %v, want ErrTooManyUtterances", err)
		}
	})
}
