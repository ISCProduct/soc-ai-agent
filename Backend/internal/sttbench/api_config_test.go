package sttbench

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func clearAudioProviderEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AI_TEXT_PROVIDER", "AI_TEXT_BASE_URL",
		"AI_AUDIO_PROVIDER", "AI_AUDIO_BASE_URL", "OPENAI_API_KEY",
	} {
		t.Setenv(key, "")
	}
}

func TestAPIConfigFromEnv_LocalAudioProviderWithoutOpenAIKey(t *testing.T) {
	clearAudioProviderEnv(t)
	t.Setenv("AI_AUDIO_PROVIDER", "local")
	t.Setenv("AI_AUDIO_BASE_URL", "http://localhost:9000/v1/")

	config, err := APIConfigFromEnv()
	if err != nil {
		t.Fatalf("APIConfigFromEnv() error = %v", err)
	}
	if config.Provider != "local" || config.BaseURL != "http://localhost:9000/v1" {
		t.Fatalf("config = %#v", config)
	}
	if config.apiKey != localAPIKey {
		t.Fatalf("local api key = %q, want placeholder", config.apiKey)
	}
}

func TestAPIConfigFromEnv_InheritsLocalTextProvider(t *testing.T) {
	clearAudioProviderEnv(t)
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", "http://localhost:11434/v1")

	config, err := APIConfigFromEnv()
	if err != nil {
		t.Fatalf("APIConfigFromEnv() error = %v", err)
	}
	if config.Provider != "local" || config.BaseURL != "http://localhost:11434/v1" {
		t.Fatalf("config = %#v", config)
	}
}

func TestAPIConfigFromEnv_RequiresLocalBaseURL(t *testing.T) {
	clearAudioProviderEnv(t)
	t.Setenv("AI_AUDIO_PROVIDER", "local")

	if _, err := APIConfigFromEnv(); err != ErrMissingLocalBaseURL {
		t.Fatalf("APIConfigFromEnv() error = %v, want %v", err, ErrMissingLocalBaseURL)
	}
}

func TestTranscribe_RoutesToLocalAudioProvider(t *testing.T) {
	clearAudioProviderEnv(t)
	var gotPath, gotAuthorization, gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm() error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		gotModel = r.FormValue("model")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"こんにちは"}`))
	}))
	defer server.Close()

	t.Setenv("AI_AUDIO_PROVIDER", "local")
	t.Setenv("AI_AUDIO_BASE_URL", server.URL+"/v1")
	t.Setenv("OPENAI_API_KEY", "test-secret")
	audio := Audio{Data: []byte("audio"), Path: "sample.wav"}
	text, _, err := transcribe("whisper-local", "", audio)
	if err != nil {
		t.Fatalf("transcribe() error = %v", err)
	}
	if text != "こんにちは" {
		t.Errorf("text = %q", text)
	}
	if gotPath != "/v1/audio/transcriptions" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuthorization != "Bearer "+localAPIKey {
		t.Errorf("authorization = %q", gotAuthorization)
	}
	if gotModel != "whisper-local" {
		t.Errorf("model = %q", gotModel)
	}
}

func TestAPIConfigFromEnv_DoesNotSendOpenAIKeyOverPlainHTTP(t *testing.T) {
	clearAudioProviderEnv(t)
	t.Setenv("AI_AUDIO_PROVIDER", "openai")
	t.Setenv("AI_AUDIO_BASE_URL", "http://audio-gateway.example/v1")
	t.Setenv("OPENAI_API_KEY", "test-secret")

	if _, err := APIConfigFromEnv(); err == nil {
		t.Fatal("expected unsafe endpoint error")
	}
}

func TestAPIConfigFromEnv_RequiresOpenAIKey(t *testing.T) {
	clearAudioProviderEnv(t)
	if _, err := APIConfigFromEnv(); err != ErrMissingOpenAIAPIKey {
		t.Fatalf("APIConfigFromEnv() error = %v, want %v", err, ErrMissingOpenAIAPIKey)
	}
}

func TestTranscribe_RequiresProviderConfigurationBeforeNetwork(t *testing.T) {
	clearAudioProviderEnv(t)
	t.Setenv("AI_AUDIO_PROVIDER", "local")
	_, _, err := transcribe("whisper-local", "", Audio{Data: []byte("audio"), Path: "sample.wav"})
	if err != ErrMissingLocalBaseURL {
		t.Fatalf("transcribe() error = %v, want %v", err, ErrMissingLocalBaseURL)
	}
}

func TestLocalCostBasisExcludesCompute(t *testing.T) {
	clearAudioProviderEnv(t)
	t.Setenv("AI_AUDIO_PROVIDER", "local")
	t.Setenv("AI_AUDIO_BASE_URL", "http://localhost:9000/v1")
	orig := transcribeFn
	transcribeFn = func(_, _ string, _ Audio) (string, int64, error) {
		return "ok", 1, nil
	}
	t.Cleanup(func() { transcribeFn = orig })

	summary := RunModel("whisper-local", nil, nil, "")
	if summary.Provider != "local" || summary.CostBasis != "local_api_only_excludes_compute" {
		t.Fatalf("summary provider/cost basis = %q/%q", summary.Provider, summary.CostBasis)
	}
	if summary.EstCostPerMinUSD != 0 {
		t.Errorf("local API estimate = %v, want 0", summary.EstCostPerMinUSD)
	}
}

func TestPrintComparisonLabelsLocalCostAsAPIOnly(t *testing.T) {
	var output bytes.Buffer
	PrintComparison(&output, []string{"whisper-local"}, Report{
		Models: map[string]*ModelSummary{
			"whisper-local": {
				Model:            "whisper-local",
				Provider:         "local",
				CostBasis:        "local_api_only_excludes_compute",
				EstCostPerMinUSD: 0,
			},
		},
	})
	got := output.String()
	if !strings.Contains(got, "0.0000*") || !strings.Contains(got, "計算資源・電力の費用を含まない") {
		t.Fatalf("local cost output is missing its caveat: %q", got)
	}
}

func TestOpenAIConfigUsesAPIKeyOnlyForHTTPSOrLoopback(t *testing.T) {
	clearAudioProviderEnv(t)
	t.Setenv("AI_AUDIO_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AI_AUDIO_BASE_URL", "http://127.0.0.1:8080/v1")
	if _, err := APIConfigFromEnv(); err != nil {
		t.Fatalf("loopback endpoint rejected: %v", err)
	}

	t.Setenv("AI_AUDIO_BASE_URL", "https://gateway.example/v1")
	if _, err := APIConfigFromEnv(); err != nil {
		t.Fatalf("HTTPS endpoint rejected: %v", err)
	}
}
