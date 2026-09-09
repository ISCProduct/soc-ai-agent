package openai

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// multipartFields はリクエストのフォーム値を取り出す（fileは除く）
func multipartFields(t *testing.T, r *http.Request) map[string]string {
	t.Helper()
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("Content-Type: %v", err)
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	fields := map[string]string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		if part.FileName() != "" {
			continue
		}
		b, _ := io.ReadAll(part)
		fields[part.FormName()] = string(b)
	}
	return fields
}

// フォールバックが指定モデルへ送っていること。
// ここが壊れるとminiで再送し続け、精度は変わらないのに費用だけ倍になる。
func TestTranscribeWithModel_SendsGivenModel(t *testing.T) {
	var got map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = multipartFields(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"御社の事業に興味があります"}`))
	}))
	defer server.Close()

	// 環境変数の既定モデルより引数が優先されること
	t.Setenv("OPENAI_WHISPER_MODEL", "gpt-4o-mini-transcribe")
	cli := NewWithBaseURL(server.URL, "gpt-4o-mini")
	text, err := cli.TranscribeWithModel(context.Background(), []byte("audio"), "audio.webm", "御社", "gpt-4o-transcribe")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if text != "御社の事業に興味があります" {
		t.Errorf("text = %q", text)
	}
	if got["model"] != "gpt-4o-transcribe" {
		t.Errorf("model = %q, want gpt-4o-transcribe", got["model"])
	}
	if got["language"] != "ja" {
		t.Errorf("language = %q, want ja", got["language"])
	}
	if got["prompt"] != "御社" {
		t.Errorf("prompt = %q, want 御社", got["prompt"])
	}
}

// 補助語が空なら prompt を送らない（従来どおりの動作）。
func TestTranscribeWithHints_OmitsEmptyPrompt(t *testing.T) {
	var got map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = multipartFields(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"はい"}`))
	}))
	defer server.Close()

	t.Setenv("OPENAI_WHISPER_MODEL", "")
	cli := NewWithBaseURL(server.URL, "gpt-4o-mini")
	if _, err := cli.TranscribeWithHints(context.Background(), []byte("audio"), "audio.webm", ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := got["prompt"]; ok {
		t.Errorf("prompt を送っている: %q", got["prompt"])
	}
	if got["model"] != "gpt-4o-mini-transcribe" {
		t.Errorf("既定モデル = %q", got["model"])
	}
}

// 補助語には企業名が含まれるため、エラー本文をそのまま漏らさない。
func TestTranscribeWithModel_ErrorDoesNotLeakHints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid prompt: 株式会社テスト"}}`))
	}))
	defer server.Close()

	cli := NewWithBaseURL(server.URL, "gpt-4o-mini")
	_, err := cli.TranscribeWithModel(context.Background(), []byte("a"), "a.webm", "株式会社テスト", "gpt-4o-transcribe")
	if err == nil {
		t.Fatal("エラーにならない")
	}
	if strings.Contains(err.Error(), "株式会社テスト") {
		t.Errorf("企業名が漏れている: %v", err)
	}
}
