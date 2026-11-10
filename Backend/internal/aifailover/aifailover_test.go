package aifailover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"Backend/internal/openai"
)

// #1833: OpenAI 障害時の STT / LLM / TTS の縮退。

func TestDegradable(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{name: "429", err: &openai.StatusError{Format: "chat error %d: %s", StatusCode: 429}, want: true},
		{name: "500", err: &openai.StatusError{Format: "chat error %d: %s", StatusCode: 500}, want: true},
		{name: "503 を包んだもの", err: fmt.Errorf("wrap: %w", &openai.StatusError{Format: "tts error %d: %s", StatusCode: 503}), want: true},
		{name: "API 側のタイムアウト", err: context.DeadlineExceeded, want: true},
		{name: "接続できない", err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}, want: true},
		{name: "名前解決できない", err: &net.DNSError{Err: "no such host", Name: "api.openai.com"}, want: true},
		{name: "認証（隠さない）", err: &openai.StatusError{Format: "chat error %d: %s", StatusCode: 401}, want: false},
		{name: "権限（隠さない）", err: &openai.StatusError{Format: "chat error %d: %s", StatusCode: 403}, want: false},
		{name: "入力不正（隠さない）", err: &openai.StatusError{Format: "chat error %d: %s", StatusCode: 400}, want: false},
		{name: "その他のエラー", err: errors.New("json 解析失敗"), want: false},
		{name: "クライアントが切断した", ctx: canceled, err: context.DeadlineExceeded, want: false},
		{name: "nil", err: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			if got := Degradable(ctx, tt.err); got != tt.want {
				t.Fatalf("Degradable = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBreaker(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	b := NewBreaker(2, 30*time.Second)
	b.now = func() time.Time { return now }

	if !b.Allow() {
		t.Fatal("閉じている間は通す")
	}
	b.Failure()
	if !b.Allow() || b.Open() {
		t.Fatal("1回の失敗では開かない")
	}
	b.Failure()
	if b.Allow() || !b.Open() {
		t.Fatal("2回続けて失敗したら開く")
	}
	now = now.Add(31 * time.Second)
	if !b.Allow() {
		t.Fatal("cooldown が過ぎたら1回だけ試す（半開）")
	}
	if b.Allow() {
		t.Fatal("試している間は他を通さない")
	}
	b.Failure()
	if b.Allow() {
		t.Fatal("試しが失敗したらまた開く")
	}
	now = now.Add(31 * time.Second)
	if !b.Allow() {
		t.Fatal("再び半開")
	}
	b.Success()
	if !b.Allow() || !b.Allow() || b.Open() {
		t.Fatal("試しが成功したら閉じる")
	}
}

// fakePrimary は OpenAI 側の呼び出しの偽物。
type fakePrimary struct {
	err   error
	calls atomic.Int32
}

func (f *fakePrimary) TranscribeWithHints(context.Context, []byte, string, string) (string, error) {
	f.calls.Add(1)
	if f.err != nil {
		return "", f.err
	}
	return "OpenAI の文字起こし", nil
}

func (f *fakePrimary) ChatInterview(context.Context, string, []map[string]string) (string, error) {
	f.calls.Add(1)
	if f.err != nil {
		return "", f.err
	}
	return "OpenAI の応答", nil
}

func (f *fakePrimary) TTS(context.Context, string, string) ([]byte, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return []byte("ID3mp3"), nil
}

var testWAV = []byte("RIFF\x24\x00\x00\x00WAVEfmt ")

// newLocalServer は Whisper.cpp / Ollama / VOICEVOX の偽物をまとめて立てる。
func newLocalServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/inference":
			if err := r.ParseMultipartForm(1 << 20); err != nil || r.FormValue("language") != "ja" {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			if _, _, err := r.FormFile("file"); err != nil {
				http.Error(w, "no file", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"text":" ローカルの文字起こし "}`))
		case "/v1/chat/completions":
			var body struct {
				Model    string `json:"model"`
				Messages []struct {
					Role string `json:"role"`
				} `json:"messages"`
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			if body.Model != "qwen3:1.7b" || len(body.Messages) == 0 || body.Messages[0].Role != "system" {
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"<think>考え中</think>ローカルの応答"}}]}`))
		case "/audio_query":
			if r.URL.Query().Get("speaker") != "3" || r.URL.Query().Get("text") == "" {
				http.Error(w, "bad query", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"accent_phrases":[]}`))
		case "/synthesis":
			_, _ = w.Write(testWAV)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, &paths
}

func newTestSpeech(primary Primary, localURL string) *Speech {
	return New(primary, Config{
		STTURL: localURL, LLMBaseURL: localURL + "/v1", LLMModel: "qwen3:1.7b",
		TTSURL: localURL, TTSSpeaker: "3",
		BreakerThreshold: 2, BreakerCooldown: time.Minute, LocalTimeout: 5 * time.Second,
	})
}

func TestSpeech_一時的な障害ならローカルへ縮退する(t *testing.T) {
	local, paths := newLocalServer(t)
	primary := &fakePrimary{err: &openai.StatusError{Format: "error %d: %s", StatusCode: 503}}
	s := newTestSpeech(primary, local.URL)
	ctx := context.Background()

	text, fallback, err := s.Transcribe(ctx, []byte("audio"), "a.webm", "御社")
	if err != nil || !fallback || text != "ローカルの文字起こし" {
		t.Fatalf("STT: text=%q fallback=%v err=%v", text, fallback, err)
	}
	reply, fallback, err := s.Chat(ctx, "system", []map[string]string{{"role": "user", "content": "回答"}})
	if err != nil || !fallback || reply != "ローカルの応答" {
		t.Fatalf("LLM: reply=%q fallback=%v err=%v（<think> は落とす）", reply, fallback, err)
	}
	audio, fallback, err := s.Speak(ctx, "次の質問です", "alloy")
	if err != nil || !fallback || !IsWAV(audio) {
		t.Fatalf("TTS: fallback=%v err=%v wav=%v", fallback, err, IsWAV(audio))
	}
	if strings.Join(*paths, ",") != "/inference,/v1/chat/completions,/audio_query,/synthesis" {
		t.Fatalf("ローカルへの呼び出し = %v", *paths)
	}
}

func TestSpeech_認証や入力不正はローカルで隠さない(t *testing.T) {
	local, paths := newLocalServer(t)
	primary := &fakePrimary{err: &openai.StatusError{Format: "error %d: %s", StatusCode: 401}}
	s := newTestSpeech(primary, local.URL)

	if _, fallback, err := s.Chat(context.Background(), "system", nil); err == nil || fallback {
		t.Fatalf("fallback=%v err=%v, want エラーのまま", fallback, err)
	}
	if len(*paths) != 0 {
		t.Fatalf("ローカルを呼んだ: %v", *paths)
	}
}

func TestSpeech_ローカル未設定なら今までと同じ(t *testing.T) {
	primary := &fakePrimary{err: &openai.StatusError{Format: "error %d: %s", StatusCode: 503}}
	s := New(primary, Config{BreakerThreshold: 1, BreakerCooldown: time.Minute})
	for range 3 {
		if _, fallback, err := s.Chat(context.Background(), "system", nil); err == nil || fallback {
			t.Fatalf("fallback=%v err=%v", fallback, err)
		}
	}
	// Breaker を使わないので、毎回 OpenAI を呼ぶ
	if primary.calls.Load() != 3 {
		t.Fatalf("OpenAI の呼び出し = %d, want 3", primary.calls.Load())
	}
	if New(nil, Config{}) != nil {
		t.Fatal("primary が無ければ nil")
	}
}

func TestSpeech_Breakerが開いたらOpenAIを呼ばない(t *testing.T) {
	local, _ := newLocalServer(t)
	primary := &fakePrimary{err: &openai.StatusError{Format: "error %d: %s", StatusCode: 500}}
	s := newTestSpeech(primary, local.URL)
	for range 5 {
		if _, fallback, err := s.Chat(context.Background(), "system", nil); err != nil || !fallback {
			t.Fatalf("fallback=%v err=%v", fallback, err)
		}
	}
	// 閾値 2 回で開き、その後は OpenAI を呼ばずにローカルへ
	if primary.calls.Load() != 2 {
		t.Fatalf("OpenAI の呼び出し = %d, want 2", primary.calls.Load())
	}
	// 他の工程の Breaker には影響しない
	if s.sttBreaker.Open() || s.ttsBreaker.Open() {
		t.Fatal("LLM の障害で STT / TTS の Breaker まで開いた")
	}
}

func TestSpeech_OpenAIが成功ならローカルを呼ばない(t *testing.T) {
	local, paths := newLocalServer(t)
	s := newTestSpeech(&fakePrimary{}, local.URL)
	if text, fallback, err := s.Transcribe(context.Background(), []byte("a"), "a.webm", ""); err != nil || fallback || text != "OpenAI の文字起こし" {
		t.Fatalf("text=%q fallback=%v err=%v", text, fallback, err)
	}
	if len(*paths) != 0 {
		t.Fatalf("ローカルを呼んだ: %v", *paths)
	}
}

func TestLocalTTS_WAVでなければエラー(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	if _, err := (&LocalTTS{URL: server.URL, Speaker: "1"}).Speak(context.Background(), "こんにちは"); err == nil {
		t.Fatal("WAV でない応答をエラーにしない")
	}
}

func TestStripThinking(t *testing.T) {
	for in, want := range map[string]string{
		"<think>a</think>答え": "答え",
		"答え":                 "答え",
		"<think>閉じていない":      "",
		"前<think>x</think>後": "前後",
	} {
		if got := stripThinking(in); got != want {
			t.Errorf("stripThinking(%q) = %q, want %q", in, got, want)
		}
	}
}
