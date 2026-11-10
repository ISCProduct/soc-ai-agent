package aifailover

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// LocalLLM は OpenAI 互換の /v1/chat/completions を持つローカルの LLM（Ollama など）。
type LocalLLM struct {
	BaseURL string // 例 http://192.168.0.10:11434/v1
	Model   string // 例 qwen3:1.7b
	HTTP    *http.Client
}

// Chat は面接官の応答を作る。history は openai.Client.ChatInterview と同じ形
// （role は user / assistant だけを通す）。
func (l *LocalLLM) Chat(ctx context.Context, systemPrompt string, history []map[string]string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := []message{{Role: "system", Content: systemPrompt}}
	for _, h := range history {
		role := h["role"]
		if role != "user" && role != "assistant" {
			role = "user"
		}
		msgs = append(msgs, message{Role: role, Content: h["content"]})
	}
	body, err := json.Marshal(map[string]any{
		"model":       l.Model,
		"messages":    msgs,
		"max_tokens":  200,
		"temperature": 0.7,
		"stream":      false,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(l.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	respBody, err := do(l.HTTP, req)
	if err != nil {
		return "", fmt.Errorf("local llm: %w", err)
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("local llm: %w", err)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return "", errors.New("local llm: empty response")
	}
	return stripThinking(result.Choices[0].Message.Content), nil
}

// stripThinking は Qwen3 などが出す <think>…</think> を落とす。面接官の発話として読み上げないため。
func stripThinking(text string) string {
	for {
		start := strings.Index(text, "<think>")
		if start < 0 {
			break
		}
		end := strings.Index(text[start:], "</think>")
		if end < 0 {
			text = text[:start]
			break
		}
		text = text[:start] + text[start+end+len("</think>"):]
	}
	return strings.TrimSpace(text)
}

// LocalSTT は Whisper.cpp の server（POST /inference）。
type LocalSTT struct {
	URL  string // 例 http://192.168.0.10:8080
	HTTP *http.Client
}

// Transcribe は音声を文字にする。hints は Whisper.cpp の prompt に渡す（固有名詞の補助）。
func (l *LocalSTT) Transcribe(ctx context.Context, audio []byte, filename, hints string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}
	for key, value := range map[string]string{"response_format": "json", "language": "ja", "prompt": hints} {
		if value == "" {
			continue
		}
		if err := w.WriteField(key, value); err != nil {
			return "", err
		}
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(l.URL, "/")+"/inference", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	respBody, err := do(l.HTTP, req)
	if err != nil {
		return "", fmt.Errorf("local stt: %w", err)
	}
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("local stt: %w", err)
	}
	return strings.TrimSpace(result.Text), nil
}

// LocalTTS は VOICEVOX エンジン（POST /audio_query → POST /synthesis、WAV を返す）。
type LocalTTS struct {
	URL     string // 例 http://192.168.0.10:50021
	Speaker string // VOICEVOX の話者 ID
	HTTP    *http.Client
}

// Speak は読み上げの WAV を返す。
func (l *LocalTTS) Speak(ctx context.Context, text string) ([]byte, error) {
	base := strings.TrimRight(l.URL, "/")
	query := url.Values{"text": {text}, "speaker": {l.Speaker}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/audio_query?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	audioQuery, err := do(l.HTTP, req)
	if err != nil {
		return nil, fmt.Errorf("local tts audio_query: %w", err)
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, base+"/synthesis?"+url.Values{"speaker": {l.Speaker}}.Encode(), bytes.NewReader(audioQuery))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	wav, err := do(l.HTTP, req)
	if err != nil {
		return nil, fmt.Errorf("local tts synthesis: %w", err)
	}
	if !IsWAV(wav) {
		return nil, errors.New("local tts: WAV ではない応答")
	}
	return wav, nil
}

// IsWAV は RIFF/WAVE のヘッダーを持つかを返す。応答の Content-Type を決めるのに使う。
func IsWAV(b []byte) bool {
	return len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WAVE"
}

func do(client *http.Client, req *http.Request) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}
