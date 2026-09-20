package costs

import (
	"math"
	"testing"
)

// TestCalculateCost はモデル別単価の計算を検証する（#1294 DesignDoc §6）。
// ローカル推論が 0 になること、未知モデルを過小評価しないことが要点。
func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		model      string
		prompt     int
		completion int
		want       float64
	}{
		{"gpt-4o", "openai", "gpt-4o", 1_000_000, 1_000_000, 12.50},
		{"gpt-4o-mini は mini 単価（最長一致）", "openai", "gpt-4o-mini", 1_000_000, 1_000_000, 0.75},
		{"バージョン付きも最長一致", "openai", "gpt-4o-mini-2024-07-18", 1_000_000, 0, 0.15},
		{"未知モデルは gpt-4o 単価（過小評価しない）", "openai", "未知のモデル", 1_000_000, 0, 2.50},
		{"ローカル推論は0", "local", "llama-3", 1_000_000, 1_000_000, 0},
		{"provider未設定はopenai扱い", "", "gpt-4o", 1_000_000, 0, 2.50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCost(tt.provider, tt.model, tt.prompt, tt.completion)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("calculateCost()=%.6f, 期待=%.6f", got, tt.want)
			}
		})
	}
}

// TestApplyModelPricingOverride は env による単価差し替えを検証する。
// 不正な設定でサーバーを起動不能にしないこと（無視してログ）も含む。
func TestApplyModelPricingOverride(t *testing.T) {
	original := modelPricing["gpt-4o"]
	t.Cleanup(func() {
		modelPricing["gpt-4o"] = original
		delete(modelPricing, "new-model")
	})

	tests := []struct {
		name  string
		raw   string
		model string
		want  [2]float64
	}{
		{"既存モデルの単価を上書き", `{"gpt-4o":[1.0,4.0]}`, "gpt-4o", [2]float64{1.0, 4.0}},
		{"新しいモデルを追加", `{"new-model":[0.5,1.5]}`, "new-model", [2]float64{0.5, 1.5}},
		{"大文字は正規化して扱う", `{"GPT-4O":[2.0,8.0]}`, "gpt-4o", [2]float64{2.0, 8.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyModelPricingOverride(tt.raw)
			if got := modelPricing[tt.model]; got != tt.want {
				t.Errorf("modelPricing[%q]=%v, 期待=%v", tt.model, got, tt.want)
			}
		})
	}

	// 不正な入力は既定値を壊さない
	invalid := []struct {
		name string
		raw  string
	}{
		{"壊れたJSON", `{"gpt-4o":`},
		{"要素数が足りない", `{"gpt-4o":[1.0]}`},
		{"負の単価", `{"gpt-4o":[-1.0,4.0]}`},
		{"空のモデル名", `{"":[1.0,4.0]}`},
	}
	modelPricing["gpt-4o"] = [2]float64{2.50, 10.00}
	for _, tt := range invalid {
		t.Run("無視: "+tt.name, func(t *testing.T) {
			applyModelPricingOverride(tt.raw)
			if got := modelPricing["gpt-4o"]; got != [2]float64{2.50, 10.00} {
				t.Errorf("不正な設定で単価が壊れた: %v", got)
			}
		})
	}
}

// TestRealtimeAudioRateDefaults は音声の既定単価が現行価格であることを固定する（#1193 / #1294）。
// 旧価格のままだとコストを3倍以上に見積もり、ローカル化の効果判断が狂う。
func TestRealtimeAudioRateDefaults(t *testing.T) {
	rates := loadTokenRates()

	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"音声入力", rates.audioInput, 32.0},
		{"音声出力", rates.audioOutput, 64.0},
		{"キャッシュ済み音声入力", rates.cachedAudioInput, 0.40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if math.Abs(tt.got-tt.want) > 1e-9 {
				t.Errorf("既定単価=%.2f, 期待=%.2f", tt.got, tt.want)
			}
		})
	}
}

// TestCalculateAudioCost は音声経路（秒・文字）の単価計算を検証する（#1294）。
// トークン課金ではないため、単価表とは別の計算になる。
func TestCalculateAudioCost(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		seconds    float64
		characters int
		want       float64
	}{
		{"STT 60秒 = 1分ぶん", "openai", 60, 0, 0.006},
		{"STT 30秒 = 半分", "openai", 30, 0, 0.003},
		{"TTS 100万文字", "openai", 0, 1_000_000, 15.0},
		{"STTとTTSの合算", "openai", 60, 1_000_000, 15.006},
		{"ローカル推論は0", "local", 60, 1_000_000, 0},
		{"使用量0なら0", "openai", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateAudioCost(tt.provider, tt.seconds, tt.characters)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("calculateAudioCost()=%.6f, 期待=%.6f", got, tt.want)
			}
		})
	}
}
