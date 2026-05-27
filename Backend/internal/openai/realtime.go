package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// RealtimeSessionRequest POST /v1/realtime/sessions のリクエストボディ
type RealtimeSessionRequest struct {
	Model                    string                 `json:"model"`
	Modalities               []string               `json:"modalities,omitempty"`
	Voice                    string                 `json:"voice,omitempty"`
	Instructions             string                 `json:"instructions,omitempty"`
	InputAudioTranscription  map[string]any `json:"input_audio_transcription,omitempty"`
	TurnDetection            map[string]any `json:"turn_detection,omitempty"`
	MaxResponseOutputTokens  any            `json:"max_response_output_tokens,omitempty"`
}

// RealtimeSessionResponse POST /v1/realtime/sessions のレスポンス
type RealtimeSessionResponse struct {
	ID           string `json:"id"`
	ClientSecret struct {
		Value     string `json:"value"`
		ExpiresAt int64  `json:"expires_at"`
	} `json:"client_secret"`
}

func (cli *Client) CreateRealtimeClientSecret(ctx context.Context, session RealtimeSessionRequest) (*RealtimeSessionResponse, error) {
	if cli.apiKey == "" {
		return nil, errors.New("openai api key is not set")
	}
	body, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/realtime/sessions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cli.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New(string(respBody))
	}

	var parsed RealtimeSessionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	if parsed.ClientSecret.Value == "" {
		return nil, errors.New("missing client_secret.value in realtime session response")
	}
	return &parsed, nil
}
