package openai

import "testing"

func TestClientTextFallbackModel(t *testing.T) {
	tests := []struct {
		name      string
		client    *Client
		wantModel string
		wantOK    bool
	}{
		{name: "nil", client: nil},
		{name: "再送しない構成", client: &Client{DefaultModel: "local-model"}},
		{
			name:   "音声だけ再送する構成はテキストの費用に入れない",
			client: &Client{fallbacks: map[string]*fallbackTransport{"audio": {fallbackModel: "gpt-transcribe"}}},
		},
		{
			name:      "再送先のモデルを返す",
			client:    &Client{DefaultModel: "local-model", fallbacks: map[string]*fallbackTransport{"text": {fallbackModel: "gpt-4o-mini"}}},
			wantModel: "gpt-4o-mini",
			wantOK:    true,
		},
		{
			name:      "モデル指定が無ければリクエストのモデル名のまま再送する",
			client:    &Client{DefaultModel: "local-model", fallbacks: map[string]*fallbackTransport{"text": {}}},
			wantModel: "local-model",
			wantOK:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, ok := tt.client.TextFallbackModel()
			if model != tt.wantModel || ok != tt.wantOK {
				t.Fatalf("TextFallbackModel() = (%q, %v), want (%q, %v)", model, ok, tt.wantModel, tt.wantOK)
			}
		})
	}
}
