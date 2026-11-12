package costs

import (
	"maps"
	"math"
	"os"
	"strings"
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

// fixAudioRateEnv は、音声の単価を上書きする env を未設定に固定する。手元や CI の設定に左右されないようにする。
func fixAudioRateEnv(t *testing.T) {
	t.Helper()
	t.Setenv(sttCostOverrideEnv, "")
	t.Setenv("TTS_COST_PER_1M_CHARS_USD", "")
}

// TestCalculateAudioCost は音声経路（秒・文字）の単価計算を検証する（#1294）。
// トークン課金ではないため、単価表とは別の計算になる。STT はモデルの分単価で計算する（#2173）。
func TestCalculateAudioCost(t *testing.T) {
	fixAudioRateEnv(t)
	tests := []struct {
		name       string
		provider   string
		model      string
		seconds    float64
		characters int
		want       float64
	}{
		{"STT 60秒 = 1分ぶん（gpt-transcribe）", "openai", "gpt-transcribe", 60, 0, 0.0045},
		{"STT 30秒 = 半分（gpt-4o-mini-transcribe）", "openai", "gpt-4o-mini-transcribe", 30, 0, 0.0015},
		{"STT 60秒（gpt-4o-transcribe）", "openai", "gpt-4o-transcribe", 60, 0, 0.006},
		{"TTS 100万文字", "openai", "tts-1", 0, 1_000_000, 15.0},
		// TTS の文字単価は、モデル名が STT の表のどれでも変わらない
		{"TTS はモデル名で単価が変わらない", "openai", "gpt-4o-mini-transcribe", 0, 1_000_000, 15.0},
		{"STTとTTSの合算", "openai", "gpt-transcribe", 60, 1_000_000, 15.0045},
		{"ローカル推論は0", "local", "gpt-transcribe", 60, 1_000_000, 0},
		{"provider未設定はopenai扱い", "", "gpt-transcribe", 60, 0, 0.0045},
		{"使用量0なら0", "openai", "gpt-transcribe", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateAudioCost(tt.provider, tt.model, tt.seconds, tt.characters)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("calculateAudioCost()=%.6f, 期待=%.6f", got, tt.want)
			}
		})
	}
}

// TestSTTCostPerMinuteUSD は、STT の分単価をモデル名で引けることを検証する（#2173）。
// 日付付きの版名は同じモデル、名前の似た別モデルは取り違えない。
func TestSTTCostPerMinuteUSD(t *testing.T) {
	fixAudioRateEnv(t)
	tests := []struct {
		name  string
		model string
		want  float64
	}{
		{"gpt-transcribe", "gpt-transcribe", 0.0045},
		{"gpt-4o-transcribe", "gpt-4o-transcribe", 0.006},
		{"gpt-4o-mini-transcribe", "gpt-4o-mini-transcribe", 0.003},
		{"whisper-1", "whisper-1", 0.006},
		{"大文字・前後の空白はそろえる", " GPT-Transcribe ", 0.0045},
		// 日付付きの版名は同じモデル
		{"日付付きの版名（mini）", "gpt-4o-mini-transcribe-2025-12-15", 0.003},
		{"日付付きの版名（4o）", "gpt-4o-transcribe-2025-03-20", 0.006},
		{"日付付きの版名（gpt-transcribe）", "gpt-transcribe-2026-03-01", 0.0045},
		{"派生名（:）", "gpt-4o-mini-transcribe:ft-test", 0.003},
		// 名前の似た別モデルを取り違えない（前方一致・部分一致で別の単価に当てない）
		{"表に無い派生モデルは既定値", "gpt-4o-transcribe-diarize", 0.006},
		{"表に無い上位版は gpt-transcribe の単価にしない", "gpt-transcribe-pro", 0.006},
		{"表に無いモデルは既定値", "stt-cost-table-test-unknown", 0.006},
		{"空のモデル名は既定値", "", 0.006},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sttCostPerMinuteUSD(tt.model); math.Abs(got-tt.want) > 1e-12 {
				t.Errorf("sttCostPerMinuteUSD(%q)=%v, 期待=%v", tt.model, got, tt.want)
			}
		})
	}
}

// STT_COST_PER_MINUTE_USD を設定していれば、モデルに関係なくその値で計算する（今までどおり。#2173）。
func TestSTTCostPerMinuteUSD_EnvOverridesTable(t *testing.T) {
	buf := captureCostLog(t)
	t.Setenv(sttCostOverrideEnv, " 0.01 ")
	for _, model := range []string{"gpt-transcribe", "gpt-4o-mini-transcribe", "whisper-1", "stt-env-override-unknown"} {
		if got := sttCostPerMinuteUSD(model); got != 0.01 {
			t.Errorf("sttCostPerMinuteUSD(%q)=%v, 期待=0.01（env を優先）", model, got)
		}
	}
	// 上書きした単価で計算したので、表に無いモデルでも「既定値で計算した」というログは出さない
	if buf.Len() != 0 {
		t.Errorf("env で上書きしたのに表に無いモデルのログが出た: %q", buf.String())
	}

	// 解釈できない値は無視して、表の単価で計算する
	t.Setenv(sttCostOverrideEnv, "not-a-number")
	if got := sttCostPerMinuteUSD("gpt-transcribe"); got != 0.0045 {
		t.Errorf("解釈できない env で sttCostPerMinuteUSD=%v, 期待=0.0045（表の単価）", got)
	}
}

// 表に無い STT モデルは、モデルごとに1回だけログに出す。大文字・小文字の違いで2回出さない（#2173）。
func TestSTTCostPerMinuteUSD_UnknownLogsOncePerModel(t *testing.T) {
	fixAudioRateEnv(t)
	buf := captureCostLog(t)
	for range 3 {
		sttCostPerMinuteUSD("STT-Warn-Once-Model")
		sttCostPerMinuteUSD("stt-warn-once-model")
		sttCostPerMinuteUSD("other-stt-warn-model")
		sttCostPerMinuteUSD("gpt-transcribe")
		sttCostPerMinuteUSD("gpt-4o-mini-transcribe-2025-12-15")
	}
	got := strings.ToLower(buf.String())
	if n := strings.Count(got, `"stt-warn-once-model"`); n != 1 {
		t.Errorf("表に無いモデルのログが %d 回（1回のはず）: %q", n, buf.String())
	}
	// 1回だけなのはモデルごと。別のモデルはそれぞれ出す
	if n := strings.Count(got, `"other-stt-warn-model"`); n != 1 {
		t.Errorf("別の表に無いモデルのログが %d 回（1回のはず）: %q", n, buf.String())
	}
	if !strings.Contains(got, "分単価の表に無い") || !strings.Contains(got, "$0.006/分") {
		t.Errorf("表に無いことと、使った単価がログで分からない: %q", buf.String())
	}
	if strings.Contains(got, "transcribe") {
		t.Errorf("表にあるモデルでログを出した: %q", buf.String())
	}
}

// 公式の価格ページ（2026-11-11 確認）の分単価になっていること。値を変えたら、確認日とあわせてここも直す（#2173）。
func TestSTTPricing_MatchesOfficialPrices(t *testing.T) {
	if os.Getenv(sttCostOverrideEnv) != "" {
		t.Skip("STT_COST_PER_MINUTE_USD で分単価を上書きしているため、公式の単価との比較はしない")
	}
	want := map[string]float64{
		"gpt-transcribe":         0.0045,
		"gpt-4o-transcribe":      0.006,
		"gpt-4o-mini-transcribe": 0.003,
		"whisper-1":              0.006,
	}
	if !maps.Equal(sttPricingPerMinute, want) {
		t.Errorf("sttPricingPerMinute = %v, want %v", sttPricingPerMinute, want)
	}
	// 表に無いモデルは、以前の一律の既定値（表の最高値）で計算する
	if sttFallbackCostPerMinuteUSD != 0.006 {
		t.Errorf("sttFallbackCostPerMinuteUSD = %v, want 0.006", sttFallbackCostPerMinuteUSD)
	}
}
