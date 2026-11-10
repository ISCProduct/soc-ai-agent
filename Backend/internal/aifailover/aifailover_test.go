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
		BreakerThreshold: 2, BreakerCooldown: time.Minute,
		PrimaryTimeout: 5 * time.Second, LocalTimeout: 5 * time.Second,
	})
}

func TestSpeech_一時的な障害ならローカルへ縮退する(t *testing.T) {
	local, paths := newLocalServer(t)
	primary := &fakePrimary{err: &openai.StatusError{Format: "error %d: %s", StatusCode: 503}}
	s := newTestSpeech(primary, local.URL)
	ctx := context.Background()

	text, out, err := s.Transcribe(ctx, []byte("audio"), "a.webm", "御社")
	if err != nil || !out.Local || text != "ローカルの文字起こし" {
		t.Fatalf("STT: text=%q fallback=%v err=%v", text, out.Local, err)
	}
	reply, out, err := s.Chat(ctx, "system", []map[string]string{{"role": "user", "content": "回答"}})
	if err != nil || !out.Local || reply != "ローカルの応答" {
		t.Fatalf("LLM: reply=%q fallback=%v err=%v（<think> は落とす）", reply, out.Local, err)
	}
	audio, out, err := s.Speak(ctx, "次の質問です", "alloy")
	if err != nil || !out.Local || !IsWAV(audio) {
		t.Fatalf("TTS: fallback=%v err=%v wav=%v", out.Local, err, IsWAV(audio))
	}
	if strings.Join(*paths, ",") != "/inference,/v1/chat/completions,/audio_query,/synthesis" {
		t.Fatalf("ローカルへの呼び出し = %v", *paths)
	}
}

func TestSpeech_認証や入力不正はローカルで隠さない(t *testing.T) {
	local, paths := newLocalServer(t)
	primary := &fakePrimary{err: &openai.StatusError{Format: "error %d: %s", StatusCode: 401}}
	s := newTestSpeech(primary, local.URL)

	if _, out, err := s.Chat(context.Background(), "system", nil); err == nil || out.Local {
		t.Fatalf("fallback=%v err=%v, want エラーのまま", out.Local, err)
	}
	if len(*paths) != 0 {
		t.Fatalf("ローカルを呼んだ: %v", *paths)
	}
}

func TestSpeech_ローカル未設定なら今までと同じ(t *testing.T) {
	primary := &fakePrimary{err: &openai.StatusError{Format: "error %d: %s", StatusCode: 503}}
	s := New(primary, Config{BreakerThreshold: 1, BreakerCooldown: time.Minute})
	for range 3 {
		if _, out, err := s.Chat(context.Background(), "system", nil); err == nil || out.Local {
			t.Fatalf("fallback=%v err=%v", out.Local, err)
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
		if _, out, err := s.Chat(context.Background(), "system", nil); err != nil || !out.Local {
			t.Fatalf("fallback=%v err=%v", out.Local, err)
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
	if text, out, err := s.Transcribe(context.Background(), []byte("a"), "a.webm", ""); err != nil || out.Local || text != "OpenAI の文字起こし" {
		t.Fatalf("text=%q fallback=%v err=%v", text, out.Local, err)
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

// 半開の試しで OpenAI が 401 を返したら、OpenAI は応答しているので閉じる。
// 試しを終えずに返すと、以後ずっとローカルへ迂回して認証エラーが表に出なくなる。
func TestSpeech_半開の試しが縮退しない失敗なら閉じる(t *testing.T) {
	local, _ := newLocalServer(t)
	primary := &fakePrimary{err: &openai.StatusError{Format: "error %d: %s", StatusCode: 503}}
	s := newTestSpeech(primary, local.URL)
	now := time.Now()
	s.llmBreaker.now = func() time.Time { return now }
	for range 2 {
		_, _, _ = s.Chat(context.Background(), "system", nil)
	}
	if !s.llmBreaker.Open() {
		t.Fatal("2回の 503 で開くはず")
	}
	now = now.Add(2 * time.Minute)
	primary.err = &openai.StatusError{Format: "error %d: %s", StatusCode: 401}
	if _, out, err := s.Chat(context.Background(), "system", nil); err == nil || out.Local || !out.PrimaryTried {
		t.Fatalf("試しの 401 はそのまま返す: out=%+v err=%v", out, err)
	}
	// 閉じているので、次も OpenAI を呼ぶ（認証エラーが表に出続ける）
	before := primary.calls.Load()
	if _, out, err := s.Chat(context.Background(), "system", nil); err == nil || out.Local {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if primary.calls.Load() != before+1 {
		t.Fatal("Breaker が閉じず、OpenAI を呼ばなかった")
	}
}

// Breaker で OpenAI を飛ばしたときは「OpenAI を試していない」（利用量を記録しない）。
func TestSpeech_Breakerで飛ばしたらPrimaryTriedはfalse(t *testing.T) {
	local, _ := newLocalServer(t)
	primary := &fakePrimary{err: &openai.StatusError{Format: "error %d: %s", StatusCode: 503}}
	s := newTestSpeech(primary, local.URL)
	_, first, _ := s.Speak(context.Background(), "a", "alloy")
	_, _, _ = s.Speak(context.Background(), "a", "alloy")
	_, skipped, _ := s.Speak(context.Background(), "a", "alloy")
	if !first.PrimaryTried || !first.Local {
		t.Fatalf("1回目は OpenAI を試してから縮退: %+v", first)
	}
	if skipped.PrimaryTried || !skipped.Local {
		t.Fatalf("Breaker が開いたら OpenAI を試さない: %+v", skipped)
	}
}

// OpenAI が応答しないときは、上限（PrimaryTimeout）で切ってローカルへ回す。
// 既定の60秒を待つと、フロントの90秒の打ち切りまでにローカルの時間が残らない。
func TestSpeech_OpenAIが遅ければ上限で切って縮退する(t *testing.T) {
	local, _ := newLocalServer(t)
	s := newTestSpeech(slowPrimary{}, local.URL)
	s.primaryTimeout = 50 * time.Millisecond
	start := time.Now()
	text, out, err := s.Chat(context.Background(), "system", nil)
	if err != nil || !out.Local || text != "ローカルの応答" {
		t.Fatalf("text=%q out=%+v err=%v", text, out, err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("OpenAI を待ちすぎた: %v", time.Since(start))
	}
}

type slowPrimary struct{}

func (slowPrimary) TranscribeWithHints(ctx context.Context, _ []byte, _, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func (slowPrimary) ChatInterview(ctx context.Context, _ string, _ []map[string]string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func (slowPrimary) TTS(ctx context.Context, _, _ string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// Qwen3 が推論（<think>）だけで max_tokens を使い切ったら、空の応答を成功にしない。
func TestLocalLLM_thinkだけの応答はエラー(t *testing.T) {
	for _, content := range []string{"<think>考え中</think>", "<think>閉じていない"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := json.Marshal(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": content}}}})
			_, _ = w.Write(b)
		}))
		_, err := (&LocalLLM{BaseURL: server.URL, Model: "qwen3:1.7b"}).Chat(context.Background(), "system", nil)
		server.Close()
		if err == nil {
			t.Fatalf("%q を成功にした", content)
		}
	}
}
