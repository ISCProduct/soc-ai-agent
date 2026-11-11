package interview

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/aifailover"
	"Backend/internal/models"
	"Backend/internal/openai"
)

// #1833: OpenAI が障害（503）のとき、面接の1ターンを STT / LLM / TTS ともローカルで続ける。
func TestTurn_OpenAI障害時はローカルへ縮退して続ける(t *testing.T) {
	openaiDown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
	}))
	defer openaiDown.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/inference":
			_, _ = w.Write([]byte(`{"text":"チームで開発しました"}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"具体的には何を担当しましたか"}}]}`))
		case "/audio_query":
			_, _ = w.Write([]byte(`{}`))
		case "/synthesis":
			_, _ = w.Write([]byte("RIFF\x24\x00\x00\x00WAVEfmt "))
		default:
			http.NotFound(w, r)
		}
	}))
	defer local.Close()
	t.Setenv("AI_FAILOVER_LOCAL_STT_URL", local.URL)
	t.Setenv("AI_FAILOVER_LOCAL_LLM_URL", local.URL+"/v1")
	t.Setenv("AI_FAILOVER_LOCAL_TTS_URL", local.URL)

	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress", Language: "ja"}),
		&utterRepoStub{}, nil, nil, nil, openai.NewWithBaseURL(openaiDown.URL, "gpt-4o-mini"), nil,
	)
	result, err := svc.Turn(context.Background(), 10, 1, []byte("audio"), "", "", false, "", "", "", 0, 1, 0, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if result.UserText != "チームで開発しました" || result.AIText != "具体的には何を担当しましたか" {
		t.Fatalf("user=%q ai=%q", result.UserText, result.AIText)
	}
	if !aifailover.IsWAV(result.Audio) {
		t.Fatal("ローカルの TTS（WAV）が返っていない")
	}
	if strings.Join(result.AIFallback, ",") != "stt,llm,tts" {
		t.Fatalf("AIFallback = %v", result.AIFallback)
	}
}

// ローカルの接続先が無ければ、今まで通り OpenAI の失敗を面接の言い換え応答で吸収する（#910）。
func TestTurn_ローカル未設定なら今まで通り(t *testing.T) {
	openaiDown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer openaiDown.Close()
	for _, name := range []string{"AI_FAILOVER_LOCAL_STT_URL", "AI_FAILOVER_LOCAL_LLM_URL", "AI_FAILOVER_LOCAL_TTS_URL"} {
		t.Setenv(name, "")
	}
	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress", Language: "ja"}),
		&utterRepoStub{}, nil, nil, nil, openai.NewWithBaseURL(openaiDown.URL, "gpt-4o-mini"), nil,
	)
	result, err := svc.Turn(context.Background(), 10, 1, []byte("audio"), "", "", false, "", "", "", 0, 1, 0, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if result.AIText != interviewFallbackReplyText || result.Audio != nil || len(result.AIFallback) != 0 {
		t.Fatalf("ai=%q audio=%v fallback=%v", result.AIText, result.Audio != nil, result.AIFallback)
	}
}
