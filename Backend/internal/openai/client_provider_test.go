package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// clearAIEnv は AI 関連の環境変数をテスト間で確実に初期化する。
// t.Setenv は設定した変数しか復元しないため、未設定にしたい変数は空で明示する。
func clearAIEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"AI_TEXT_PROVIDER", "AI_TEXT_BASE_URL", "AI_TEXT_MODEL",
		"AI_EMBEDDING_PROVIDER", "AI_EMBEDDING_BASE_URL", "AI_EMBEDDING_MODEL",
		"AI_AUDIO_PROVIDER", "AI_AUDIO_BASE_URL",
		"OPENAI_API_KEY", "OPENAI_MODEL", "OPENAI_EMBEDDING_MODEL",
	} {
		t.Setenv(k, "")
	}
}

// TestNewFromEnv_ProviderMatrix は env の組み合わせごとの解決結果を検証する（#1293）。
func TestNewFromEnv_ProviderMatrix(t *testing.T) {
	tests := []struct {
		name          string
		env           map[string]string
		optionalModel string

		wantErr            bool
		wantTextProvider   string
		wantTextBaseURL    string
		wantModel          string
		wantEmbedBaseURL   string
		wantEmbedModel     string
		wantAudioBaseURL   string
		wantTextAvailable  bool
		wantEmbedAvailable bool
		wantAudioAvailable bool
	}{
		{
			name:             "すべて未設定: 現行動作のままOpenAIを向くが、キーが無いので縮退する",
			env:              map[string]string{},
			wantTextProvider: providerOpenAI, wantTextBaseURL: defaultOpenAIBaseURL,
			wantModel: "gpt-4o-mini", wantEmbedBaseURL: defaultOpenAIBaseURL,
			wantEmbedModel: "text-embedding-3-small", wantAudioBaseURL: defaultOpenAIBaseURL,
		},
		{
			name:             "キーのみ設定: 従来どおり全系統が利用可能",
			env:              map[string]string{"OPENAI_API_KEY": "sk-test"},
			wantTextProvider: providerOpenAI, wantTextBaseURL: defaultOpenAIBaseURL,
			wantModel: "gpt-4o-mini", wantEmbedBaseURL: defaultOpenAIBaseURL,
			wantEmbedModel: "text-embedding-3-small", wantAudioBaseURL: defaultOpenAIBaseURL,
			wantTextAvailable: true, wantEmbedAvailable: true, wantAudioAvailable: true,
		},
		{
			name: "テキストのみlocal指定: provider と base URL が3系統に継承される（1台構成）",
			env: map[string]string{
				"AI_TEXT_PROVIDER": "local",
				"AI_TEXT_BASE_URL": "http://localhost:11434/v1",
				"AI_TEXT_MODEL":    "gpt-oss-20b",
			},
			wantTextProvider: providerLocal, wantTextBaseURL: "http://localhost:11434/v1",
			wantModel:        "gpt-oss-20b",
			wantEmbedBaseURL: "http://localhost:11434/v1", wantEmbedModel: "text-embedding-3-small",
			wantAudioBaseURL:  "http://localhost:11434/v1",
			wantTextAvailable: true, wantEmbedAvailable: true, wantAudioAvailable: true,
		},
		{
			name: "テキストlocal + 埋め込みは明示的にopenai: キー無しなら埋め込みだけ縮退",
			env: map[string]string{
				"AI_TEXT_PROVIDER":      "local",
				"AI_TEXT_BASE_URL":      "http://localhost:11434/v1",
				"AI_EMBEDDING_PROVIDER": "openai",
			},
			wantTextProvider: providerLocal, wantTextBaseURL: "http://localhost:11434/v1",
			wantModel: "gpt-4o-mini",
			// openai を名乗る系統は base URL を継承しない（明示指定が黙って無視されるのを防ぐ）
			wantEmbedBaseURL: defaultOpenAIBaseURL, wantEmbedModel: "text-embedding-3-small",
			wantAudioBaseURL:  "http://localhost:11434/v1",
			wantTextAvailable: true, wantAudioAvailable: true,
		},
		{
			name:    "localなのにbase URLが無い: 設定として成立しないのでエラー",
			env:     map[string]string{"AI_TEXT_PROVIDER": "local"},
			wantErr: true,
		},
		{
			// テキストが OpenAI 本家のままなので継承元が無く、local として成立しない
			name:    "埋め込みだけlocalでbase URLが無い場合はエラー",
			env:     map[string]string{"AI_EMBEDDING_PROVIDER": "local", "OPENAI_API_KEY": "sk-test"},
			wantErr: true,
		},
		{
			name: "プロバイダ名は大文字でも解釈する",
			env: map[string]string{
				"AI_TEXT_PROVIDER": "LOCAL", "AI_TEXT_BASE_URL": "http://localhost:8000/v1",
			},
			wantTextProvider: providerLocal, wantTextBaseURL: "http://localhost:8000/v1",
			wantModel: "gpt-4o-mini", wantEmbedBaseURL: "http://localhost:8000/v1",
			wantEmbedModel: "text-embedding-3-small", wantAudioBaseURL: "http://localhost:8000/v1",
			wantTextAvailable: true, wantEmbedAvailable: true, wantAudioAvailable: true,
		},
		{
			name:             "未知のプロバイダ名はopenaiへフォールバック（起動を止めない）",
			env:              map[string]string{"AI_TEXT_PROVIDER": "bedrock", "OPENAI_API_KEY": "sk-test"},
			wantTextProvider: providerOpenAI, wantTextBaseURL: defaultOpenAIBaseURL,
			wantModel: "gpt-4o-mini", wantEmbedBaseURL: defaultOpenAIBaseURL,
			wantEmbedModel: "text-embedding-3-small", wantAudioBaseURL: defaultOpenAIBaseURL,
			wantTextAvailable: true, wantEmbedAvailable: true, wantAudioAvailable: true,
		},
		{
			// ゲートウェイ経由は3系統すべてに継承される（provider が同じため）。
			// 継承しないと、埋め込みと STT/TTS だけ本家へ直行してしまう
			name: "openaiプロバイダでもbase URLは差し替えられる（プロキシ・互換ゲートウェイ）",
			env: map[string]string{
				"OPENAI_API_KEY": "sk-test", "AI_TEXT_BASE_URL": "https://gw.example.test/v1/",
			},
			wantTextProvider: providerOpenAI, wantTextBaseURL: "https://gw.example.test/v1",
			wantModel: "gpt-4o-mini", wantEmbedBaseURL: "https://gw.example.test/v1",
			wantEmbedModel: "text-embedding-3-small", wantAudioBaseURL: "https://gw.example.test/v1",
			wantTextAvailable: true, wantEmbedAvailable: true, wantAudioAvailable: true,
		},
		{
			name:    "base URL に scheme が無ければ起動時エラー",
			env:     map[string]string{"OPENAI_API_KEY": "sk-test", "AI_TEXT_BASE_URL": "api.openai.com/v1"},
			wantErr: true,
		},
		{
			name: "系統ごとに別の推論先を指定できる",
			env: map[string]string{
				"AI_TEXT_PROVIDER":      "local",
				"AI_TEXT_BASE_URL":      "http://text:11434/v1",
				"AI_EMBEDDING_PROVIDER": "local",
				"AI_EMBEDDING_BASE_URL": "http://embed:8080/v1",
				"AI_EMBEDDING_MODEL":    "bge-m3",
				"AI_AUDIO_PROVIDER":     "local",
				"AI_AUDIO_BASE_URL":     "http://audio:9000/v1",
			},
			wantTextProvider: providerLocal, wantTextBaseURL: "http://text:11434/v1",
			wantModel: "gpt-4o-mini", wantEmbedBaseURL: "http://embed:8080/v1",
			wantEmbedModel: "bge-m3", wantAudioBaseURL: "http://audio:9000/v1",
			wantTextAvailable: true, wantEmbedAvailable: true, wantAudioAvailable: true,
		},
		{
			name: "音声だけlocalにして文字起こしをローカル化（#1209の想定）",
			env: map[string]string{
				"AI_AUDIO_PROVIDER": "local", "AI_AUDIO_BASE_URL": "http://whisper:9000/v1",
			},
			wantTextProvider: providerOpenAI, wantTextBaseURL: defaultOpenAIBaseURL,
			wantModel: "gpt-4o-mini", wantEmbedBaseURL: defaultOpenAIBaseURL,
			wantEmbedModel: "text-embedding-3-small", wantAudioBaseURL: "http://whisper:9000/v1",
			wantAudioAvailable: true,
		},
		{
			name: "レガシーenv(OPENAI_MODEL / OPENAI_EMBEDDING_MODEL)も引き続き効く",
			env: map[string]string{
				"OPENAI_API_KEY": "sk-test", "OPENAI_MODEL": "gpt-4o",
				"OPENAI_EMBEDDING_MODEL": "text-embedding-3-large",
			},
			wantTextProvider: providerOpenAI, wantTextBaseURL: defaultOpenAIBaseURL,
			wantModel: "gpt-4o", wantEmbedBaseURL: defaultOpenAIBaseURL,
			wantEmbedModel: "text-embedding-3-large", wantAudioBaseURL: defaultOpenAIBaseURL,
			wantTextAvailable: true, wantEmbedAvailable: true, wantAudioAvailable: true,
		},
		{
			name: "AI_TEXT_MODEL は OPENAI_MODEL より優先される",
			env: map[string]string{
				"OPENAI_API_KEY": "sk-test", "OPENAI_MODEL": "gpt-4o", "AI_TEXT_MODEL": "qwen2.5",
			},
			wantTextProvider: providerOpenAI, wantTextBaseURL: defaultOpenAIBaseURL,
			wantModel: "qwen2.5", wantEmbedBaseURL: defaultOpenAIBaseURL,
			wantEmbedModel: "text-embedding-3-small", wantAudioBaseURL: defaultOpenAIBaseURL,
			wantTextAvailable: true, wantEmbedAvailable: true, wantAudioAvailable: true,
		},
		{
			name:             "引数のモデル指定が最優先",
			env:              map[string]string{"OPENAI_API_KEY": "sk-test", "AI_TEXT_MODEL": "qwen2.5"},
			optionalModel:    "gpt-4o-mini-explicit",
			wantTextProvider: providerOpenAI, wantTextBaseURL: defaultOpenAIBaseURL,
			wantModel: "gpt-4o-mini-explicit", wantEmbedBaseURL: defaultOpenAIBaseURL,
			wantEmbedModel: "text-embedding-3-small", wantAudioBaseURL: defaultOpenAIBaseURL,
			wantTextAvailable: true, wantEmbedAvailable: true, wantAudioAvailable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAIEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cli, err := NewFromEnv(tt.optionalModel)
			if tt.wantErr {
				if err == nil {
					t.Fatal("エラーを期待したが nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			text, embedding, audio := cli.Providers()
			if text != tt.wantTextProvider {
				t.Errorf("textProvider = %q, want %q", text, tt.wantTextProvider)
			}
			if cli.BaseURL() != tt.wantTextBaseURL {
				t.Errorf("BaseURL() = %q, want %q", cli.BaseURL(), tt.wantTextBaseURL)
			}
			if cli.DefaultModel != tt.wantModel {
				t.Errorf("DefaultModel = %q, want %q", cli.DefaultModel, tt.wantModel)
			}
			if cli.EmbeddingBaseURL() != tt.wantEmbedBaseURL {
				t.Errorf("EmbeddingBaseURL() = %q, want %q", cli.EmbeddingBaseURL(), tt.wantEmbedBaseURL)
			}
			if cli.EmbeddingModel != tt.wantEmbedModel {
				t.Errorf("EmbeddingModel = %q, want %q", cli.EmbeddingModel, tt.wantEmbedModel)
			}
			if cli.AudioBaseURL() != tt.wantAudioBaseURL {
				t.Errorf("AudioBaseURL() = %q, want %q", cli.AudioBaseURL(), tt.wantAudioBaseURL)
			}
			if cli.textAvailable != tt.wantTextAvailable {
				t.Errorf("textAvailable = %v, want %v", cli.textAvailable, tt.wantTextAvailable)
			}
			if cli.embeddingAvailable != tt.wantEmbedAvailable {
				t.Errorf("embeddingAvailable = %v, want %v", cli.embeddingAvailable, tt.wantEmbedAvailable)
			}
			if cli.audioAvailable != tt.wantAudioAvailable {
				t.Errorf("audioAvailable = %v, want %v", cli.audioAvailable, tt.wantAudioAvailable)
			}
			// 埋め込みの推論先がテキストと同じなら SDK クライアントを共有する（無駄なコネクションを作らない）
			sameTarget := tt.wantEmbedBaseURL == tt.wantTextBaseURL && embedding == text
			if sameTarget != (cli.embedC == cli.c) {
				t.Errorf("embedC 共有 = %v, want %v (embedding=%s text=%s audio=%s)",
					cli.embedC == cli.c, sameTarget, embedding, text, audio)
			}
		})
	}
}

// TestNewFromEnv_BootsWithoutAPIKey は OPENAI_API_KEY が無くても起動できることを検証する（#1293）。
//
// 修正前は main.go が log.Fatalf で終了していたため、キーが無いとサーバー自体が立たなかった。
func TestNewFromEnv_BootsWithoutAPIKey(t *testing.T) {
	clearAIEnv(t)

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatalf("キー無しで初期化が失敗した: %v", err)
	}
	if cli == nil {
		t.Fatal("クライアントが nil")
	}
	if !cli.Degraded() {
		t.Error("キー無しなのに Degraded() が false")
	}
}

// TestDegraded_AllEntryPointsFailFast は縮退時に全ての公開呼び出しが
// ErrAIUnavailable を返し、HTTPリクエストを1本も出さないことを検証する（#1293）。
//
// ネットワークに出てしまうと、キー無しの環境で毎回 401 待ちのレイテンシが乗る。
func TestDegraded_AllEntryPointsFailFast(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	clearAIEnv(t)
	// base URL だけ与え、キーは無し（= openai プロバイダで縮退）
	t.Setenv("AI_TEXT_BASE_URL", srv.URL)
	t.Setenv("AI_EMBEDDING_BASE_URL", srv.URL)
	t.Setenv("AI_AUDIO_BASE_URL", srv.URL)

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	calls := []struct {
		name string
		call func() error
	}{
		{"Embedding", func() error { _, err := cli.Embedding(ctx, "text"); return err }},
		{"ChatCompletionJSON", func() error {
			_, err := cli.ChatCompletionJSON(ctx, "sys", "user", 0, 16)
			return err
		}},
		{"WebSearchJSON", func() error { _, err := cli.WebSearchJSON(ctx, "user", 16); return err }},
		{"Responses", func() error { _, err := cli.Responses(ctx, "input"); return err }},
		{"ResponsesWithTemperature", func() error {
			_, err := cli.ResponsesWithTemperature(ctx, "sys", "user", 0.7)
			return err
		}},
		{"ResponsesWithMaxTokens", func() error {
			_, err := cli.ResponsesWithMaxTokens(ctx, "sys", "user", 0.7, 16)
			return err
		}},
		{"Transcribe", func() error { _, err := cli.Transcribe(ctx, []byte("audio"), "a.webm"); return err }},
		{"TranscribeWithHints", func() error {
			_, err := cli.TranscribeWithHints(ctx, []byte("audio"), "a.webm", "hint")
			return err
		}},
		{"TranscribeWithModel", func() error {
			_, err := cli.TranscribeWithModel(ctx, []byte("audio"), "a.webm", "hint", "gpt-4o-transcribe")
			return err
		}},
		{"TTS", func() error { _, err := cli.TTS(ctx, "text", "alloy"); return err }},
		{"ChatInterview", func() error {
			_, err := cli.ChatInterview(ctx, "sys", []map[string]string{{"role": "user", "content": "hi"}})
			return err
		}},
		{"CreateRealtimeClientSecret", func() error {
			_, err := cli.CreateRealtimeClientSecret(ctx, RealtimeSessionRequest{})
			return err
		}},
	}

	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			err := c.call()
			if !errors.Is(err, ErrAIUnavailable) {
				t.Fatalf("err = %v, want ErrAIUnavailable", err)
			}
		})
	}

	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("縮退中にHTTPリクエストが %d 回発行された", n)
	}
}

// TestLocalProvider_RoutesToLocalServer はキー無し・local プロバイダで
// ローカル推論先へリクエストが届くことを検証する（#1293）。
func TestLocalProvider_RoutesToLocalServer(t *testing.T) {
	var gotPath, gotAuth, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer srv.Close()

	clearAIEnv(t)
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", srv.URL)
	t.Setenv("AI_TEXT_MODEL", "gpt-oss-20b")

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}

	out, err := cli.ChatCompletionJSON(context.Background(), "sys", "user", 0, 64)
	if err != nil {
		t.Fatalf("ローカル推論先への呼び出しが失敗した: %v", err)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("応答が取れていない: %q", out)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotModel != "gpt-oss-20b" {
		t.Errorf("model = %q, want gpt-oss-20b", gotModel)
	}
	// キー未設定でも Authorization ヘッダー自体は送る（空だと401を返す互換サーバーがある）
	if gotAuth != "Bearer "+localPlaceholderAPIKey {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer "+localPlaceholderAPIKey)
	}
}

// TestSystemsRouteToTheirOwnServers は3系統がそれぞれの推論先へ振り分けられることを検証する（#1293）。
func TestSystemsRouteToTheirOwnServers(t *testing.T) {
	var textPaths, embedPaths, audioPaths []string

	textSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		textPaths = append(textPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/responses" {
			// Responses API は chat/completions と応答形式が違う
			_, _ = w.Write([]byte(`{"output_text":"ok"}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer textSrv.Close()

	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		embedPaths = append(embedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":1}}`))
	}))
	defer embedSrv.Close()

	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		audioPaths = append(audioPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"こんにちは"}`))
	}))
	defer audioSrv.Close()

	clearAIEnv(t)
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", textSrv.URL)
	t.Setenv("AI_EMBEDDING_PROVIDER", "local")
	t.Setenv("AI_EMBEDDING_BASE_URL", embedSrv.URL)
	t.Setenv("AI_AUDIO_PROVIDER", "local")
	t.Setenv("AI_AUDIO_BASE_URL", audioSrv.URL)

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := cli.ChatCompletionJSON(ctx, "sys", "user", 0, 16); err != nil {
		t.Fatalf("text(chat): %v", err)
	}
	// Responses API も同じテキスト推論先へ行くこと。
	// ここを検証していなかったため、doResponses に残っていた旧 apiKey ガードで
	// local プロバイダのテキスト系が全滅していたのを見落としていた
	if _, err := cli.Responses(ctx, "input"); err != nil {
		t.Fatalf("text(responses): %v", err)
	}
	if _, err := cli.Embedding(ctx, "text"); err != nil {
		t.Fatalf("embedding: %v", err)
	}
	if _, err := cli.Transcribe(ctx, []byte("audio"), "a.webm"); err != nil {
		t.Fatalf("audio(stt): %v", err)
	}
	if _, err := cli.TTS(ctx, "こんにちは", "alloy"); err != nil {
		t.Fatalf("audio(tts): %v", err)
	}

	if len(textPaths) != 2 || textPaths[0] != "/chat/completions" || textPaths[1] != "/responses" {
		t.Errorf("text へのリクエスト = %v", textPaths)
	}
	if len(embedPaths) != 1 || embedPaths[0] != "/embeddings" {
		t.Errorf("embedding へのリクエスト = %v", embedPaths)
	}
	if len(audioPaths) != 2 || audioPaths[0] != "/audio/transcriptions" || audioPaths[1] != "/audio/speech" {
		t.Errorf("audio へのリクエスト = %v", audioPaths)
	}
}

// TestLocalEndpointsNeverReceiveRealKey は OPENAI_API_KEY を残したまま local へ
// 切り替えたときに、実キーがローカル推論先へ送信されないことを検証する（#1293）。
//
// 「キーはタスク定義に残したまま provider だけ local にする」のが最も自然な移行手順で、
// provider 基準でキーを選ぶ実装だと実キーが外部へ漏れる。
func TestLocalEndpointsNeverReceiveRealKey(t *testing.T) {
	const realKey = "sk-REAL-SECRET"
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/embeddings":
			_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1]}],"usage":{"prompt_tokens":1}}`))
		case "/audio/transcriptions":
			_, _ = w.Write([]byte(`{"text":"ok"}`))
		default:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
		}
	}))
	defer srv.Close()

	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", realKey)
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", srv.URL)
	// 埋め込み・音声は未指定（テキストの provider と base URL を継承する）

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := cli.ChatCompletionJSON(ctx, "sys", "user", 0, 16); err != nil {
		t.Fatalf("text: %v", err)
	}
	if _, err := cli.Embedding(ctx, "text"); err != nil {
		t.Fatalf("embedding: %v", err)
	}
	if _, err := cli.Transcribe(ctx, []byte("audio"), "a.webm"); err != nil {
		t.Fatalf("audio: %v", err)
	}

	if len(seen) != 3 {
		t.Fatalf("リクエスト数=%d want 3", len(seen))
	}
	for _, auth := range seen {
		if strings.Contains(auth, realKey) {
			t.Fatalf("実キーがローカル推論先へ送信された: %q", auth)
		}
		if auth != "Bearer "+localPlaceholderAPIKey {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer "+localPlaceholderAPIKey)
		}
	}
}

// TestRealtimeDegradesWithLocalAudio は音声をローカル化した構成で Realtime が
// 縮退することを検証する（#1293）。
//
// Realtime は OpenAI 固有 API で互換サーバーには存在しない。URL が固定のまま
// ダミーキーを本家へ送ると 401 になるだけなので、呼ぶ前に縮退させる。
func TestRealtimeDegradesWithLocalAudio(t *testing.T) {
	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("AI_AUDIO_PROVIDER", "local")
	t.Setenv("AI_AUDIO_BASE_URL", "http://whisper:9000/v1")

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cli.CreateRealtimeClientSecret(context.Background(), RealtimeSessionRequest{}); !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("err = %v, want ErrAIUnavailable", err)
	}
	// 音声が OpenAI のままならガードは通る（実際のリクエストは送らない）
	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-test")
	cli2, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if err := cli2.ensureRealtime(); err != nil {
		t.Fatalf("OpenAI 構成で縮退した: %v", err)
	}
}

// TestPartialAvailability は系統ごとに独立して縮退することを検証する（#1293）。
// テキストだけローカル化した構成で、テキストは動き、埋め込みは縮退する。
func TestPartialAvailability(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer srv.Close()

	clearAIEnv(t)
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", srv.URL)
	// 埋め込みだけ明示的に OpenAI に残す（キーが無いので縮退する）
	t.Setenv("AI_EMBEDDING_PROVIDER", "openai")

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := cli.ChatCompletionJSON(ctx, "sys", "user", 0, 16); err != nil {
		t.Fatalf("テキストはローカルで動くべき: %v", err)
	}
	if _, err := cli.Embedding(ctx, "text"); !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("埋め込みは縮退すべき: err = %v", err)
	}
	if !cli.Degraded() {
		t.Error("一部縮退しているのに Degraded() が false")
	}
}

// TestNilClientReturnsErrAIUnavailable は nil クライアントでも panic せず縮退することを検証する。
// DI 漏れで nil が渡ってもサーバーが落ちないようにするため。
func TestNilClientReturnsErrAIUnavailable(t *testing.T) {
	var cli *Client
	if _, err := cli.Embedding(context.Background(), "x"); !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("err = %v, want ErrAIUnavailable", err)
	}
	if _, err := cli.ChatCompletionJSON(context.Background(), "s", "u", 0, 8); !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("err = %v, want ErrAIUnavailable", err)
	}
	if !cli.Degraded() {
		t.Error("nil クライアントは Degraded() = true であるべき")
	}
}

// TestKeyFor はキーの受け渡し規則をテーブル駆動で固定する（#1293）。
//
// 「実キーをどこへ送るか」は誤ると本番シークレットが外部へ出るため、
// 判定規則そのものをテストで固定する。
func TestKeyFor(t *testing.T) {
	const realKey = "sk-real"
	tests := []struct {
		name     string
		provider string
		baseURL  string
		explicit bool
		want     string
	}{
		{
			name: "openai + 本家: 実キー", provider: providerOpenAI,
			baseURL: defaultOpenAIBaseURL, want: realKey,
		},
		{
			name: "openai + 明示指定のプロキシ: 実キー（operator の意図）", provider: providerOpenAI,
			baseURL: "https://gw.example.test/v1", explicit: true, want: realKey,
		},
		{
			name: "openai + 継承した base URL: ダミー", provider: providerOpenAI,
			baseURL: "http://ollama:11434/v1", explicit: false, want: localPlaceholderAPIKey,
		},
		{
			name: "local: 常にダミー", provider: providerLocal,
			baseURL: "http://ollama:11434/v1", explicit: true, want: localPlaceholderAPIKey,
		},
		{
			name: "local + 本家を明示しても実キーは渡さない", provider: providerLocal,
			baseURL: defaultOpenAIBaseURL, explicit: true, want: localPlaceholderAPIKey,
		},
		{
			// http:// の typo で実キーが平文送信されるのを防ぐ
			name: "本家ホストでも http は本家扱いしない", provider: providerOpenAI,
			baseURL: "http://api.openai.com/v1", explicit: false, want: localPlaceholderAPIKey,
		},
		{
			name: "似たホスト名は本家扱いしない", provider: providerOpenAI,
			baseURL: "https://api.openai.com.evil.test/v1", explicit: false, want: localPlaceholderAPIKey,
		},
		{
			name: "ポート付きの本家は本家扱い", provider: providerOpenAI,
			baseURL: "https://api.openai.com:443/v1", want: realKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keyFor(tt.provider, tt.baseURL, tt.explicit, realKey); got != tt.want {
				t.Fatalf("keyFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestGatewayInheritedByAllSystems はゲートウェイ経由の設定が3系統すべてに効き、
// 実キーが渡ることを検証する（#1293）。
//
// 継承しないと埋め込みと STT/TTS だけ OpenAI 本家へ直行するため、
// egress を制限した環境では失敗し、制限が無い環境では意図しない直接送信になる。
func TestGatewayInheritedByAllSystems(t *testing.T) {
	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-real")
	t.Setenv("AI_TEXT_BASE_URL", "https://gw.corp.example/v1")

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range []string{cli.BaseURL(), cli.EmbeddingBaseURL(), cli.AudioBaseURL()} {
		if got != "https://gw.corp.example/v1" {
			t.Errorf("base URL = %q, want ゲートウェイ", got)
		}
	}
	// 明示指定した宛先なので実キーを渡す（そうしないと 401 で機能しない）
	if cli.textKey != "sk-real" || cli.audioKey != "sk-real" {
		t.Errorf("textKey=%q audioKey=%q, want sk-real", cli.textKey, cli.audioKey)
	}
}

// TestExplicitOpenAIEmbeddingGoesToOfficial はテキストをローカル化しても
// 埋め込みを明示的に OpenAI に残せることを検証する（#1293）。
func TestExplicitOpenAIEmbeddingGoesToOfficial(t *testing.T) {
	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-real")
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", "http://ollama:11434/v1")
	t.Setenv("AI_EMBEDDING_PROVIDER", "openai")

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if cli.EmbeddingBaseURL() != defaultOpenAIBaseURL {
		t.Errorf("EmbeddingBaseURL() = %q, want %q（明示した provider が尊重されるべき）",
			cli.EmbeddingBaseURL(), defaultOpenAIBaseURL)
	}
	// テキストはローカルなのでダミーキー、埋め込みは本家なので実キー
	if cli.textKey != localPlaceholderAPIKey {
		t.Errorf("textKey = %q, want %q", cli.textKey, localPlaceholderAPIKey)
	}
	if cli.embedC == cli.c {
		t.Error("推論先が違うのに SDK クライアントを共有している")
	}
}
