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
	t.Setenv(ttsCostOverrideEnv, "")
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
		name      string
		model     string
		want      float64
		wantBasis string
	}{
		{"gpt-transcribe", "gpt-transcribe", 0.0045, PriceBasisListed},
		// トークン課金のモデルの分単価は公式の概算（est.）なので estimated（#2229 のレビュー）
		{"gpt-4o-transcribe", "gpt-4o-transcribe", 0.006, PriceBasisEstimated},
		{"gpt-4o-transcribe-diarize", "gpt-4o-transcribe-diarize", 0.006, PriceBasisEstimated},
		{"gpt-4o-mini-transcribe", "gpt-4o-mini-transcribe", 0.003, PriceBasisEstimated},
		{"whisper-1", "whisper-1", 0.006, PriceBasisListed},
		{"whisper-1", "whisper-1", 0.006, PriceBasisListed},
		{"大文字・前後の空白はそろえる", " GPT-Transcribe ", 0.0045, PriceBasisListed},
		// 日付付きの版名は同じモデル
		{"日付付きの版名（mini）", "gpt-4o-mini-transcribe-2025-12-15", 0.003, PriceBasisEstimated},
		{"日付付きの版名（4o）", "gpt-4o-transcribe-2025-03-20", 0.006, PriceBasisEstimated},
		{"日付付きの版名（gpt-transcribe）", "gpt-transcribe-2026-03-01", 0.0045, PriceBasisListed},
		{"派生名（:）", "gpt-4o-mini-transcribe:ft-test", 0.003, PriceBasisEstimated},
		// 名前の似た別モデルを取り違えない（前方一致・部分一致で別の単価に当てない）。表に無いので既定値で、根拠は estimated（#2224）
		{"表に無い派生モデルは既定値", "gpt-4o-mini-transcribe-diarize", 0.006, PriceBasisEstimated},
		{"表に無い上位版は gpt-transcribe の単価にしない", "gpt-transcribe-pro", 0.006, PriceBasisEstimated},
		{"表に無いモデルは既定値", "stt-cost-table-test-unknown", 0.006, PriceBasisEstimated},
		{"空のモデル名は既定値", "", 0.006, PriceBasisEstimated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, basis := sttCostPerMinuteUSD(tt.model)
			if math.Abs(got-tt.want) > 1e-12 || basis != tt.wantBasis {
				t.Errorf("sttCostPerMinuteUSD(%q)=%v/%q, 期待=%v/%q", tt.model, got, basis, tt.want, tt.wantBasis)
			}
		})
	}
}

// STT_COST_PER_MINUTE_USD を設定していれば、モデルに関係なくその値で計算する（今までどおり。#2173）。
func TestSTTCostPerMinuteUSD_EnvOverridesTable(t *testing.T) {
	buf := captureCostLog(t)
	t.Setenv(sttCostOverrideEnv, " 0.01 ")
	for _, model := range []string{"gpt-transcribe", "gpt-4o-mini-transcribe", "whisper-1", "stt-env-override-unknown"} {
		// 上書きは運用側が確認した単価なので、表に無いモデルでも listed（AI_MODEL_PRICING_JSON と同じ。#2224）
		if got, basis := sttCostPerMinuteUSD(model); got != 0.01 || basis != PriceBasisListed {
			t.Errorf("sttCostPerMinuteUSD(%q)=%v/%q, 期待=0.01/listed（env を優先）", model, got, basis)
		}
	}
	// 上書きした単価で計算したので、表に無いモデルでも「既定値で計算した」というログは出さない
	if buf.Len() != 0 {
		t.Errorf("env で上書きしたのに表に無いモデルのログが出た: %q", buf.String())
	}

	// 解釈できない値は無視して、表の単価で計算する
	t.Setenv(sttCostOverrideEnv, "not-a-number")
	if got, basis := sttCostPerMinuteUSD("gpt-transcribe"); got != 0.0045 || basis != PriceBasisListed {
		t.Errorf("解釈できない env で sttCostPerMinuteUSD=%v/%q, 期待=0.0045/listed（表の単価）", got, basis)
	}
	// 解釈できない値は上書きしたことにならない。表に無いモデルは既定値の estimated のまま
	if got, basis := sttCostPerMinuteUSD("stt-env-invalid-unknown"); got != 0.006 || basis != PriceBasisEstimated {
		t.Errorf("解釈できない env で表に無いモデル=%v/%q, 期待=0.006/estimated", got, basis)
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

// 公式の価格ページ（2026-11-11 確認。gpt-4o-transcribe-diarize は 2026-11-13）の分単価になっていること。値を変えたら、確認日とあわせてここも直す（#2173）。
func TestSTTPricing_MatchesOfficialPrices(t *testing.T) {
	if os.Getenv(sttCostOverrideEnv) != "" {
		t.Skip("STT_COST_PER_MINUTE_USD で分単価を上書きしているため、公式の単価との比較はしない")
	}
	want := map[string]float64{
		"gpt-transcribe":            0.0045,
		"gpt-4o-transcribe":         0.006,
		"gpt-4o-transcribe-diarize": 0.006,
		"gpt-4o-mini-transcribe":    0.003,
		"whisper-1":                 0.006,
	}
	if !maps.Equal(sttPricingPerMinute, want) {
		t.Errorf("sttPricingPerMinute = %v, want %v", sttPricingPerMinute, want)
	}
	// 表に無いモデルは、以前の一律の既定値（表の最高値）で計算する
	if sttFallbackCostPerMinuteUSD != 0.006 {
		t.Errorf("sttFallbackCostPerMinuteUSD = %v, want 0.006", sttFallbackCostPerMinuteUSD)
	}
}

// PublishedSTTCostPerMinuteUSD は表の公開価格だけを返し、env の上書きは見ない（評価ハーネスが公開価格で見積もるため。#2224）。
func TestPublishedSTTCostPerMinuteUSD(t *testing.T) {
	t.Setenv(sttCostOverrideEnv, "0.5")
	tests := []struct {
		model  string
		want   float64
		wantOK bool
	}{
		{"gpt-transcribe", 0.0045, true},
		{"gpt-4o-transcribe-diarize", 0.006, true},
		{"gpt-4o-mini-transcribe-2025-12-15", 0.003, true},
		{"gpt-transcribe-pro", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got, ok := PublishedSTTCostPerMinuteUSD(tt.model)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("PublishedSTTCostPerMinuteUSD(%q)=%v/%v, 期待=%v/%v", tt.model, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TTS の文字単価はモデルに関係なく同じ。根拠は、既定値が公開価格そのものである tts-1 と env の上書きは listed、
// それ以外のモデルは $15 がそのモデルの単価ではないので estimated（#2224）。
func TestTTSCostPer1MCharsUSD(t *testing.T) {
	fixAudioRateEnv(t)
	tests := []struct {
		name      string
		model     string
		wantBasis string
	}{
		{"tts-1 は公開価格", "tts-1", PriceBasisListed},
		{"大文字・前後の空白はそろえる", " TTS-1 ", PriceBasisListed},
		{"日付付きの版名は同じモデル", "tts-1-2026-01-01", PriceBasisListed},
		{"tts-1-hd は単価が違うので推定", "tts-1-hd", PriceBasisEstimated},
		{"gpt-4o-mini-tts はトークン課金なので推定", "gpt-4o-mini-tts", PriceBasisEstimated},
		{"空のモデル名は推定", "", PriceBasisEstimated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, basis := ttsCostPer1MCharsUSD(tt.model)
			if got != 15.0 || basis != tt.wantBasis {
				t.Errorf("ttsCostPer1MCharsUSD(%q)=%v/%q, 期待=15/%q", tt.model, got, basis, tt.wantBasis)
			}
		})
	}
}

// TTS_COST_PER_1M_CHARS_USD を設定していれば、その値で計算し、運用側が確認した単価として listed にする（#2224）。
// 解釈できない値は無視して既定値に戻る（以前の shared.GetFloatEnv と同じ）。
func TestTTSCostPer1MCharsUSD_EnvOverride(t *testing.T) {
	t.Setenv(ttsCostOverrideEnv, " 30 ")
	for _, model := range []string{"tts-1", "tts-1-hd"} {
		if got, basis := ttsCostPer1MCharsUSD(model); got != 30 || basis != PriceBasisListed {
			t.Errorf("ttsCostPer1MCharsUSD(%q)=%v/%q, 期待=30/listed（env を優先）", model, got, basis)
		}
	}
	t.Setenv(ttsCostOverrideEnv, "not-a-number")
	if got, basis := ttsCostPer1MCharsUSD("tts-1-hd"); got != 15 || basis != PriceBasisEstimated {
		t.Errorf("解釈できない env で ttsCostPer1MCharsUSD=%v/%q, 期待=15/estimated", got, basis)
	}
}

// audioCost は料金と根拠を同じ計算で返す。秒数と文字数の両方がある行は注意が必要なほうの根拠（#2224）。
func TestAudioCost_Basis(t *testing.T) {
	fixAudioRateEnv(t)
	tests := []struct {
		name       string
		provider   string
		model      string
		seconds    float64
		characters int
		wantCost   float64
		wantBasis  string
	}{
		{"STT 表にある", "openai", "gpt-transcribe", 60, 0, 0.0045, PriceBasisListed},
		{"STT 表に無い", "openai", "stt-audio-cost-unknown", 60, 0, 0.006, PriceBasisEstimated},
		{"TTS tts-1", "openai", "tts-1", 0, 1_000_000, 15.0, PriceBasisListed},
		{"TTS tts-1 以外", "openai", "tts-1-hd", 0, 1_000_000, 15.0, PriceBasisEstimated},
		// 両方ある行: STT は listed、TTS は（tts-1 ではないので）estimated → 注意が必要な estimated
		{"両方: STT が listed、TTS が estimated", "openai", "gpt-transcribe", 60, 1_000_000, 15.0045, PriceBasisEstimated},
		// 両方ある行: STT は（表に無いので）estimated、TTS は listed → estimated。空を listed と取り違えて逆にしない
		{"両方: STT が estimated、TTS が listed", "openai", "tts-1", 60, 1_000_000, 15.006, PriceBasisEstimated},
		{"30秒は切り上げず半分", "openai", "gpt-transcribe", 30, 0, 0.00225, PriceBasisListed},
		{"ローカル推論は0で根拠なし", "local", "gpt-transcribe", 60, 1_000_000, 0, ""},
		{"使用量0なら0で根拠なし", "openai", "gpt-transcribe", 0, 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, basis := audioCost(tt.provider, tt.model, tt.seconds, tt.characters)
			if math.Abs(got-tt.wantCost) > 1e-9 || basis != tt.wantBasis {
				t.Errorf("audioCost()=%.6f/%q, 期待=%.6f/%q", got, basis, tt.wantCost, tt.wantBasis)
			}
		})
	}
}
