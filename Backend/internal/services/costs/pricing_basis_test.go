package costs

import (
	"bytes"
	"log"
	"maps"
	"strings"
	"testing"

	openaiPkg "Backend/internal/openai"
)

// #2004: 推定値と実価格を表の構造で区別し、単価表に無いモデルが黙って集計されないようにする。

func TestPriceBasis(t *testing.T) {
	cases := []struct{ model, want string }{
		{"gpt-4o-mini", PriceBasisListed},
		{"gpt-4o-mini-2024-07-18", PriceBasisListed},
		{"GPT-4o", PriceBasisListed},
		{"gpt-5.2", PriceBasisEstimated},
		{"gpt-5.2-2025-12-11", PriceBasisEstimated},
		{"gpt-4o-2024-08-06", PriceBasisListed}, // 日付付きの版は同じモデル
		// 日付の無い版名（- と4桁の数字）も同じモデル（#2151 のレビュー）
		{"gpt-4-0613", PriceBasisListed},
		{"gpt-4-1106-preview", PriceBasisListed},
		{"gpt-3.5-turbo-0125", PriceBasisListed},
		{"gpt-3.5-turbo-1106", PriceBasisListed},
		// 数字が4桁でない・数字で始まらないものは別モデル
		{"gpt-4-32k", PriceBasisInherited},
		{"gpt-4-turbox", PriceBasisInherited},
		{"gpt-4.1-mini", PriceBasisUnknown},
		{"totally-new-model", PriceBasisUnknown},
		// 前方一致で別モデルの単価に当たったものは、公開価格と区別する（#2151 のレビュー）
		{"gpt-4o-realtime-preview", PriceBasisInherited},
		{"o3-mini", PriceBasisInherited},
		{"gpt-4o-mini-transcribe", PriceBasisInherited},
		// 推定値の表でも、別モデルに当たったものは inherited（#2151 のレビュー）
		{"gpt-5.2-mini", PriceBasisInherited},
	}
	for _, tc := range cases {
		if got := PriceBasis(tc.model); got != tc.want {
			t.Errorf("PriceBasis(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}

func TestCalculateCost_EstimatedAndUnknown(t *testing.T) {
	// 推定値は estimatedPricing の単価、未登録は gpt-4o 単価（過小評価しない）で計算する
	if got := calculateCost("openai", "gpt-5.2", 1_000_000, 1_000_000); got != 12.50 {
		t.Errorf("gpt-5.2 = %v, want 12.50", got)
	}
	if got := calculateCost("openai", "unknown-model-for-cost-test", 1_000_000, 1_000_000); got != 12.50 {
		t.Errorf("未登録 = %v, want 12.50（0円にしない）", got)
	}
	if got := calculateCost("local", "unknown-model-for-cost-test", 1_000_000, 1_000_000); got != 0 {
		t.Errorf("ローカル推論 = %v, want 0", got)
	}
}

// 推定値のモデルを AI_MODEL_PRICING_JSON で上書きしたら、運用側が確認した単価として listed になる。
func TestApplyModelPricingOverride_EstimatedBecomesListed(t *testing.T) {
	savedListed, savedEstimated := maps.Clone(modelPricing), maps.Clone(estimatedPricing)
	t.Cleanup(func() { modelPricing, estimatedPricing = savedListed, savedEstimated })

	applyModelPricingOverride(`{"gpt-5.2":[1.25,10.0]}`)

	if got := PriceBasis("gpt-5.2"); got != PriceBasisListed {
		t.Errorf("上書き後の PriceBasis = %q, want listed", got)
	}
	// estimatedPricing は「まだ実価格を確認していないモデル」の一覧でもあるので、上書きしたら外れる
	if _, still := estimatedPricing["gpt-5.2"]; still {
		t.Error("上書きしたのに、推定値の一覧に残っている")
	}
	if got := calculateCost("openai", "gpt-5.2", 1_000_000, 1_000_000); got != 11.25 {
		t.Errorf("上書き後の単価で計算されていない: %v", got)
	}
}

func TestWarnPricingOnce_LogsOncePerModel(t *testing.T) {
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })

	for range 3 {
		calculateCost("openai", "warn-once-unknown-model", 100, 100)
		calculateCost("openai", "gpt-4o-mini", 100, 100)
	}

	got := buf.String()
	if n := strings.Count(got, "warn-once-unknown-model"); n != 1 {
		t.Errorf("単価未登録のログが %d 回（1回のはず）: %q", n, got)
	}
	if !strings.Contains(got, "単価表に無い") {
		t.Errorf("単価未登録であることがログで分からない: %q", got)
	}
	if strings.Contains(got, "gpt-4o-mini") {
		t.Errorf("公開価格のモデルでログを出した: %q", got)
	}
}

func TestSummaryPriceBasis(t *testing.T) {
	cases := []struct {
		model string
		cost  float64
		want  string
	}{
		{"gpt-4o-mini", 1.0, PriceBasisListed},
		{"gpt-5.2", 1.0, PriceBasisEstimated},
		{"gpt-4.1-mini", 1.0, PriceBasisUnknown},
		{"gpt-oss-20b", 0, ""}, // ローカル推論は単価を使っていないので「単価未登録」と出さない
		{"o3-mini", 1.0, PriceBasisInherited},
		// 音声の単価（分・文字数）で課金するモデルは、トークン単価表を使っていない（#2151 のレビュー）
		{"gpt-4o-mini-transcribe", 1.0, ""},
		{"whisper-1", 1.0, ""},
		{"tts-1", 1.0, ""},
		{"gpt-4o-mini-tts", 1.0, ""}, // -tts を含む名前も音声のモデル（#2151 のレビュー）
	}
	for _, tc := range cases {
		if got := summaryPriceBasis(tc.model, tc.cost); got != tc.want {
			t.Errorf("summaryPriceBasis(%q, %v) = %q, want %q", tc.model, tc.cost, got, tc.want)
		}
	}
}

func captureCostLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })
	return &buf
}

// 大文字・小文字の違いで同じ警告を2回出さない（#2151 のレビュー）。
func TestWarnPricingOnce_CaseInsensitive(t *testing.T) {
	buf := captureCostLog(t)
	calculateCost("openai", "Case-Unknown-Model", 100, 100)
	calculateCost("openai", "case-unknown-model", 100, 100)
	if n := strings.Count(strings.ToLower(buf.String()), "case-unknown-model"); n != 1 {
		t.Errorf("警告が %d 回（1回のはず）: %q", n, buf.String())
	}
}

// 別モデルの単価に当たったときは、どのモデルの単価かをログに出す。
func TestWarnPricingOnce_InheritedNamesTheSourceModel(t *testing.T) {
	buf := captureCostLog(t)
	calculateCost("openai", "o3-mini-warn-test", 100, 100)
	if got := buf.String(); !strings.Contains(got, `別モデル "o3"`) {
		t.Errorf("どのモデルの単価かがログで分からない: %q", got)
	}
}

// 音声の利用ではトークン単価表を引かず、トークン単価の警告も出さない（#2151 のレビュー）。
func TestUsageCostUSD_AudioDoesNotWarnTokenPricing(t *testing.T) {
	buf := captureCostLog(t)
	cost := usageCostUSD(openaiPkg.Usage{Provider: "openai", Model: "gpt-transcribe-audio-test", AudioSeconds: 60})
	if cost != calculateAudioCost("openai", 60, 0) {
		t.Errorf("音声の単価で計算されていない: %v", cost)
	}
	if buf.Len() != 0 {
		t.Errorf("音声の利用でトークン単価の警告が出た: %q", buf.String())
	}
}

// 未登録モデルの代わりの単価は、上書きした gpt-4o の単価に追従する（#2151 のレビュー）。
func TestUnknownModelFollowsOverriddenGPT4o(t *testing.T) {
	savedListed, savedEstimated := maps.Clone(modelPricing), maps.Clone(estimatedPricing)
	t.Cleanup(func() { modelPricing, estimatedPricing = savedListed, savedEstimated })

	applyModelPricingOverride(`{"gpt-4o":[1.0,4.0]}`)

	if got := calculateCost("openai", "totally-new-model", 1_000_000, 1_000_000); got != 5.0 {
		t.Errorf("未登録モデルが上書き後の gpt-4o 単価で計算されていない: %v", got)
	}
}

// 入力・出力の両方が0の上書きは受け付けない（0円で黙って集計しない。#2151 のレビュー）。
func TestApplyModelPricingOverride_RejectsZeroPrices(t *testing.T) {
	savedListed, savedEstimated := maps.Clone(modelPricing), maps.Clone(estimatedPricing)
	t.Cleanup(func() { modelPricing, estimatedPricing = savedListed, savedEstimated })
	buf := captureCostLog(t)

	before := modelPricing["gpt-4o"]
	applyModelPricingOverride(`{"gpt-4o":[0,0],"gpt-4o-mini":[0,0.6]}`)

	if got := modelPricing["gpt-4o"]; got != before {
		t.Errorf("両方0の上書きが入った: %v", got)
	}
	if got := modelPricing["gpt-4o-mini"]; got != [2]float64{0, 0.6} {
		t.Errorf("片方だけ0の上書きは受け付ける: %v", got)
	}
	if !strings.Contains(buf.String(), `"gpt-4o" を無視しました`) {
		t.Errorf("無視したことがログに出ていない: %q", buf.String())
	}
}

// 推定値の表の別モデルに当たったときは、推定値であることもログで分かる。
func TestWarnPricingOnce_InheritedFromEstimated(t *testing.T) {
	buf := captureCostLog(t)
	calculateCost("openai", "gpt-5.2-mini-warn-test", 100, 100)
	if got := buf.String(); !strings.Contains(got, `別モデル "gpt-5.2" の単価（推定値）`) {
		t.Errorf("推定値の別モデルの単価であることがログで分からない: %q", got)
	}
}
