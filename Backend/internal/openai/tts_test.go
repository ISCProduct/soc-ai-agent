package openai

import (
	"context"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranscribe_Model(t *testing.T) {
	tests := []struct {
		name      string
		envModel  string
		wantModel string
	}{
		{name: "default is gpt-4o-transcribe", envModel: "", wantModel: "gpt-4o-transcribe"},
		{name: "respects env override", envModel: "whisper-1", wantModel: "whisper-1"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
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

			t.Setenv("OPENAI_WHISPER_MODEL", tt.envModel)
			cli := NewWithBaseURL(server.URL, "gpt-4o-mini")
			if _, err := cli.Transcribe(context.Background(), []byte("dummy"), "audio.webm"); err != nil {
				t.Fatalf("Transcribe returned error: %v", err)
			}
			if gotModel != tt.wantModel {
				t.Fatalf("Transcribe model = %q, want %q", gotModel, tt.wantModel)
			}
		})
	}
}
