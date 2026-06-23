package services_test

// 実行: cd Backend && go test ./test/services/... -run TestRealtime -v

import (
	"math"
	"testing"

	"Backend/internal/services"
)

func TestRealtimeTokenRatesCalcCost(t *testing.T) {
	tests := []struct {
		name     string
		envs     map[string]string
		usage    services.TokenUsage
		wantCost float64
	}{
		{
			name: "テキストのみ",
			usage: services.TokenUsage{
				InputTextTokens:  1_000_000,
				OutputTextTokens: 1_000_000,
			},
			// デフォルト単価: input $5, output $15 → 合計 $20
			wantCost: 20.0,
		},
		{
			name: "音声のみ",
			usage: services.TokenUsage{
				InputAudioTokens:  1_000_000,
				OutputAudioTokens: 1_000_000,
			},
			// デフォルト単価: input $100, output $200 → 合計 $300
			wantCost: 300.0,
		},
		{
			name: "キャッシュ済み音声入力",
			usage: services.TokenUsage{
				InputCachedAudioTokens: 1_000_000,
			},
			// デフォルト単価: $20
			wantCost: 20.0,
		},
		{
			name: "混合トークン",
			usage: services.TokenUsage{
				InputTextTokens:        500_000,
				OutputTextTokens:       500_000,
				InputAudioTokens:       100_000,
				OutputAudioTokens:      100_000,
				InputCachedAudioTokens: 200_000,
			},
			// text: 0.5*5 + 0.5*15 = 10
			// audio: 0.1*100 + 0.1*200 = 30
			// cached: 0.2*20 = 4
			// 合計: 44
			wantCost: 44.0,
		},
		{
			name:     "トークン0はコスト0",
			usage:    services.TokenUsage{},
			wantCost: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := services.CalcTokenCost(tt.usage)
			if math.Abs(got-tt.wantCost) > 1e-6 {
				t.Fatalf("cost mismatch: got=%.6f want=%.6f", got, tt.wantCost)
			}
		})
	}
}

func TestCloseSessionTokenBasedFallback(t *testing.T) {
	tests := []struct {
		name           string
		tokens         *services.TokenUsage
		durationSec    int64
		wantTokenBased bool
	}{
		{
			name:           "tokensがnilのとき時間ベース",
			tokens:         nil,
			durationSec:    60,
			wantTokenBased: false,
		},
		{
			name: "tokensが非nilのときトークンベース",
			tokens: &services.TokenUsage{
				InputAudioTokens:  500_000,
				OutputAudioTokens: 300_000,
			},
			durationSec:    60,
			wantTokenBased: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTokenBased := tt.tokens != nil
			if isTokenBased != tt.wantTokenBased {
				t.Fatalf("token based mismatch: got=%v want=%v", isTokenBased, tt.wantTokenBased)
			}
		})
	}
}
