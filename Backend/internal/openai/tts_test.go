package openai

import (
	"context"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranscribe_DefaultModelIsGPT4oTranscribe(t *testing.T) {
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("failed to parse content type: %v", err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		form, err := mr.ReadForm(10 << 20)
		if err != nil {
			t.Fatalf("failed to read multipart form: %v", err)
		}
		if v := form.Value["model"]; len(v) > 0 {
			gotModel = v[0]
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"テスト"}`))
	}))
	defer server.Close()

	t.Setenv("OPENAI_WHISPER_MODEL", "")
	cli := NewWithBaseURL(server.URL, "gpt-4o-mini")
	if _, err := cli.Transcribe(context.Background(), []byte("dummy"), "audio.webm"); err != nil {
		t.Fatalf("Transcribe returned error: %v", err)
	}
	if gotModel != "gpt-4o-transcribe" {
		t.Fatalf("Transcribe model = %q, want %q", gotModel, "gpt-4o-transcribe")
	}
}

func TestTranscribe_RespectsEnvOverride(t *testing.T) {
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		form, _ := mr.ReadForm(10 << 20)
		if v := form.Value["model"]; len(v) > 0 {
			gotModel = v[0]
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"テスト"}`))
	}))
	defer server.Close()

	t.Setenv("OPENAI_WHISPER_MODEL", "whisper-1")
	cli := NewWithBaseURL(server.URL, "gpt-4o-mini")
	if _, err := cli.Transcribe(context.Background(), []byte("dummy"), "audio.webm"); err != nil {
		t.Fatalf("Transcribe returned error: %v", err)
	}
	if gotModel != "whisper-1" {
		t.Fatalf("Transcribe model = %q, want %q", gotModel, "whisper-1")
	}
}
