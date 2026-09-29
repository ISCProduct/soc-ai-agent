package sttbench

import (
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

// 補助語の有無を測り分けるには、空のときに prompt を送らないことが前提になる。
// 空文字の prompt を送ると「補助語なし」の測定が成立しない。
func TestBuildTranscribeForm(t *testing.T) {
	tests := []struct {
		name       string
		hints      string
		wantPrompt string
		wantExists bool
	}{
		{name: "補助語なしなら prompt を付けない", hints: "", wantExists: false},
		{name: "補助語ありなら prompt を付ける", hints: "御社, Go, AWS", wantPrompt: "御社, Go, AWS", wantExists: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Audio{ID: "x", Path: "audio/x.wav", Data: []byte("RIFFdummy")}
			buf, contentType, err := buildTranscribeForm("gpt-4o-mini-transcribe", tt.hints, a)
			if err != nil {
				t.Fatalf("buildTranscribeForm: %v", err)
			}
			fields, files := parseForm(t, buf.String(), contentType)

			if got, ok := fields["prompt"]; ok != tt.wantExists || got != tt.wantPrompt {
				t.Errorf("prompt = %q (存在 %v), want %q (存在 %v)", got, ok, tt.wantPrompt, tt.wantExists)
			}
			if fields["model"] != "gpt-4o-mini-transcribe" {
				t.Errorf("model = %q", fields["model"])
			}
			// 言語を固定しないとモデルが英語で書き起こすことがあり、CERが意味を失う
			if fields["language"] != "ja" {
				t.Errorf("language = %q, want ja", fields["language"])
			}
			if files["file"] != "x.wav" {
				t.Errorf("file のファイル名 = %q, want x.wav", files["file"])
			}
		})
	}
}

// parseForm は multipart 本体をフィールド名 -> 値、ファイル名の対応に分解する。
func parseForm(t *testing.T, body, contentType string) (map[string]string, map[string]string) {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("Content-Type の解析に失敗: %v", err)
	}
	r := multipart.NewReader(strings.NewReader(body), params["boundary"])
	form, err := r.ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("multipart の解析に失敗: %v", err)
	}
	fields := map[string]string{}
	for k, v := range form.Value {
		fields[k] = v[0]
	}
	files := map[string]string{}
	for k, v := range form.File {
		files[k] = v[0].Filename
	}
	return fields, files
}
