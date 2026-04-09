package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// Transcribe は音声データを Whisper でテキストに変換します
func (cli *Client) Transcribe(ctx context.Context, audio []byte, filename string) (string, error) {
	if cli.apiKey == "" {
		return "", errors.New("openai api key is not set")
	}

	model := os.Getenv("OPENAI_WHISPER_MODEL")
	if model == "" {
		model = "whisper-1"
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", model)
	_ = w.WriteField("language", "ja")
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.BaseURL()+"/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("whisper error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Text), nil
}

// TTS は OpenAI TTS API でテキストを音声に変換し、mp3 バイト列を返します
func (cli *Client) TTS(ctx context.Context, text, voice string) ([]byte, error) {
	if cli.apiKey == "" {
		return nil, errors.New("openai api key is not set")
	}

	model := os.Getenv("OPENAI_TTS_MODEL")
	if model == "" {
		model = "tts-1"
	}
	if voice == "" {
		voice = os.Getenv("OPENAI_TTS_VOICE")
	}
	if voice == "" {
		voice = "alloy"
	}

	payload := map[string]interface{}{
		"model": model,
		"input": text,
		"voice": voice,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.BaseURL()+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cli.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts error %d: %s", resp.StatusCode, string(errBody))
	}

	return io.ReadAll(resp.Body)
}

// ChatInterview は面接官として会話し、次のメッセージを返します
func (cli *Client) ChatInterview(ctx context.Context, systemPrompt string, history []map[string]string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type request struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float32   `json:"temperature"`
	}

	model := os.Getenv("OPENAI_INTERVIEW_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	msgs := []message{{Role: "system", Content: systemPrompt}}
	for _, h := range history {
		msgs = append(msgs, message{Role: h["role"], Content: h["content"]})
	}

	payload := request{
		Model:       model,
		Messages:    msgs,
		MaxTokens:   200,
		Temperature: 0.7,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.BaseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("chat error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", errors.New("no choices returned from chat API")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}
