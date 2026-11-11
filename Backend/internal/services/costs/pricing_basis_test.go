package costs

import (
	"bytes"
	"log"
	"maps"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	openaiPkg "Backend/internal/openai"
)

// #2004: 推定値と実価格を表の構造で区別し、単価表に無いモデルが黙って集計されないようにする。

func TestPriceBasis(t *testing.T) {
	cases := []struct{ model, want string }{
		{"gpt-4o-mini", PriceBasisListed},
		{"gpt-4o-mini-2024-07-18", PriceBasisListed},
		{"GPT-4o", PriceBasisListed},
		{"gpt-5.2", PriceBasisListed}, // 2026-11-11 に公式の単価で確定（#2004）
		{"gpt-5.2-2025-12-11", PriceBasisListed},
		{"gpt-4.1-mini", PriceBasisListed},
		{"o3-mini", PriceBasisListed},
		{"gpt-4o-realtime-preview", PriceBasisListed},
		{"gpt-4o-mini-transcribe", PriceBasisListed},
		// 推定値（公式に単価の記載が無い）。gpt-4 の版の規則より、明示したキーが優先される
		{"gpt-4-1106-preview", PriceBasisEstimated},
		{"gpt-4o-2024-08-06", PriceBasisListed}, // 日付付きの版は同じモデル
		// 日付の無い版名（- と4桁の数字）も同じモデル（#2151 のレビュー）
		{"gpt-4-0613", PriceBasisListed},
		{"gpt-4-0125-preview", PriceBasisEstimated},
		{"gpt-3.5-turbo-0125", PriceBasisListed},
		{"gpt-3.5-turbo-1106", PriceBasisListed},
		// 数字が4桁でない・数字で始まらないものは別モデル
		{"gpt-4-32k", PriceBasisInherited},
		{"gpt-4-turbox", PriceBasisInherited},
		{"totally-new-model", PriceBasisUnknown},
		// 前方一致で別モデルの単価に当たったものは、公開価格と区別する（#2151 のレビュー）
		{"o3-pro", PriceBasisInherited},
		// 推定値の表でも、別モデルに当たったものは inherited（#2151 のレビュー）
		{"gpt-4-1106-preview-vision", PriceBasisInherited},
	}
	for _, tc := range cases {
		if got := PriceBasis(tc.model); got != tc.want {
			t.Errorf("PriceBasis(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}

func TestCalculateCost_EstimatedAndUnknown(t *testing.T) {
	// 推定値は estimatedPricing の単価、未登録は gpt-4o 単価（過小評価しない）で計算する
	if got := calculateCost("openai", "gpt-4-1106-preview", 1_000_000, 1_000_000); got != 40.00 {
		t.Errorf("gpt-4-1106-preview = %v, want 40.00", got)
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

	applyModelPricingOverride(`{"gpt-4-1106-preview":[12.0,36.0]}`)

	if got := PriceBasis("gpt-4-1106-preview"); got != PriceBasisListed {
		t.Errorf("上書き後の PriceBasis = %q, want listed", got)
	}
	// estimatedPricing は「まだ実価格を確認していないモデル」の一覧でもあるので、上書きしたら外れる
	if _, still := estimatedPricing["gpt-4-1106-preview"]; still {
		t.Error("上書きしたのに、推定値の一覧に残っている")
	}
	if got := calculateCost("openai", "gpt-4-1106-preview", 1_000_000, 1_000_000); got != 48.00 {
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
		{"gpt-4-1106-preview", 1.0, PriceBasisEstimated},
		{"totally-new-model", 1.0, PriceBasisUnknown},
		{"gpt-oss-20b", 0, ""}, // ローカル推論は単価を使っていないので「単価未登録」と出さない
		{"o3-pro", 1.0, PriceBasisInherited},
		// 音声の単価（分・文字数）で課金するモデルは、トークン単価表を使っていない（#2151 のレビュー）
		{"gpt-4o-mini-transcribe", 1.0, ""},
		{"whisper-1", 1.0, ""},
		{"tts-1", 1.0, ""},
		{"gpt-4o-mini-tts", 1.0, ""}, // -tts を含む名前も音声のモデル（#2151 のレビュー）
	}
	for _, tc := range cases {
		if got := summaryPriceBasis(tc.model, tc.cost, "", true); got != tc.want {
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
	calculateCost("openai", "o3-pro-warn-test", 100, 100)
	if got := buf.String(); !strings.Contains(got, `別モデル "o3"`) {
		t.Errorf("どのモデルの単価かがログで分からない: %q", got)
	}
}

// 音声の利用ではトークン単価表を引かず、トークン単価の警告も出さない（#2151 のレビュー）。
// gpt-transcribe はトークン単価表に無いが、分単価の表にはあるので、どちらの警告も出ない。
func TestUsageCostUSD_AudioDoesNotWarnTokenPricing(t *testing.T) {
	fixAudioRateEnv(t)
	buf := captureCostLog(t)
	cost := usageCostUSD(openaiPkg.Usage{Provider: "openai", Model: "gpt-transcribe", AudioSeconds: 60})
	if cost != calculateAudioCost("openai", "gpt-transcribe", 60, 0) {
		t.Errorf("音声の単価で計算されていない: %v", cost)
	}
	if buf.Len() != 0 {
		t.Errorf("音声の利用でトークン単価の警告が出た: %q", buf.String())
	}
}

// 記録する料金は、使ったモデルの分単価で計算する（#2173 の受け入れ条件）。
func TestUsageCostUSD_STTUsesModelRate(t *testing.T) {
	fixAudioRateEnv(t)
	cases := []struct {
		name string
		u    openaiPkg.Usage
		want float64
	}{
		{"gpt-transcribe で1分 = $0.0045", openaiPkg.Usage{Provider: "openai", Model: "gpt-transcribe", AudioSeconds: 60}, 0.0045},
		{"gpt-4o-mini-transcribe で1分 = $0.003", openaiPkg.Usage{Provider: "openai", Model: "gpt-4o-mini-transcribe", AudioSeconds: 60}, 0.003},
		{"表に無いモデルで1分 = $0.006", openaiPkg.Usage{Provider: "openai", Model: "stt-usage-unknown-model", AudioSeconds: 60}, 0.006},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apiCallLogFromUsage(tc.u, time.Now()).CostUSD; math.Abs(got-tc.want) > 1e-12 {
				t.Errorf("CostUSD = %v, want %v", got, tc.want)
			}
		})
	}

	// STT_COST_PER_MINUTE_USD を設定していれば、その値で記録する
	t.Setenv(sttCostOverrideEnv, "0.01")
	if got := usageCostUSD(openaiPkg.Usage{Provider: "openai", Model: "gpt-transcribe", AudioSeconds: 60}); got != 0.01 {
		t.Errorf("env を設定したときの料金 = %v, want 0.01", got)
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
	calculateCost("openai", "gpt-4-1106-preview-warn-test", 100, 100)
	if got := buf.String(); !strings.Contains(got, `別モデル "gpt-4-1106-preview" の単価（推定値）`) {
		t.Errorf("推定値の別モデルの単価であることがログで分からない: %q", got)
	}
}

// 公式の価格ページ（2026-11-11 確認）の単価になっていること。値を変えたら、確認日とあわせてここも直す（#2004）。
func TestModelPricing_MatchesOfficialPrices(t *testing.T) {
	if os.Getenv("AI_MODEL_PRICING_JSON") != "" {
		t.Skip("AI_MODEL_PRICING_JSON で単価表を上書きしているため、公式の単価との比較はしない")
	}
	want := map[string][2]float64{
		"gpt-5":                   {1.25, 10.00},
		"gpt-5.2":                 {1.75, 14.00},
		"gpt-4.1":                 {2.00, 8.00},
		"gpt-4.1-mini":            {0.40, 1.60},
		"gpt-4o":                  {2.50, 10.00},
		"gpt-4o-mini":             {0.15, 0.60},
		"o3":                      {2.00, 8.00},
		"o3-mini":                 {1.10, 4.40},
		"o1":                      {15.00, 60.00},
		"o1-mini":                 {1.10, 4.40},
		"gpt-4-turbo":             {10.00, 30.00},
		"gpt-4":                   {30.00, 60.00},
		"gpt-3.5-turbo":           {0.50, 1.50},
		"gpt-4o-transcribe":       {2.50, 10.00},
		"gpt-4o-mini-transcribe":  {1.25, 5.00},
		"gpt-4o-realtime-preview": {5.00, 20.00},
		"gpt-5-search-api":        {1.25, 10.00},
		"text-embedding-3-small":  {0.02, 0.02},
		"text-embedding-3-large":  {0.13, 0.13},
	}
	for model, w := range want {
		got, ok := modelPricing[model]
		if !ok || got != w {
			t.Errorf("modelPricing[%q] = %v (ok=%v), want %v", model, got, ok, w)
		}
	}
	// 公式の単価が無いのは gpt-4-turbo のプレビュー版（3つ）だけ
	if len(estimatedPricing) != 3 {
		t.Errorf("推定値の表 = %v（gpt-4-turbo のプレビュー版3つのはず）", estimatedPricing)
	}
}

// 記録した時点の根拠を api_call_logs に残す（#2004）。音声の単価・ローカル推論・トークン0の行はトークン単価表を使わないので空。
func TestRecordedPriceBasis(t *testing.T) {
	cases := []struct {
		name string
		u    openaiPkg.Usage
		want string
	}{
		{"公開価格", openaiPkg.Usage{Provider: "openai", Model: "gpt-4o-mini", PromptTokens: 100}, PriceBasisListed},
		{"provider が空でも OpenAI 扱い", openaiPkg.Usage{Model: "gpt-4o-mini", PromptTokens: 100}, PriceBasisListed},
		{"推定値", openaiPkg.Usage{Provider: "openai", Model: "gpt-4-1106-preview", PromptTokens: 100}, PriceBasisEstimated},
		{"別モデルの単価", openaiPkg.Usage{Provider: "openai", Model: "o3-pro", CompletionTokens: 100}, PriceBasisInherited},
		{"単価表に無い", openaiPkg.Usage{Provider: "openai", Model: "totally-new-model", PromptTokens: 100}, PriceBasisUnknown},
		{"音声（秒数）", openaiPkg.Usage{Provider: "openai", Model: "gpt-transcribe", AudioSeconds: 30}, ""},
		{"音声（文字数）", openaiPkg.Usage{Provider: "openai", Model: "tts-1", Characters: 100}, ""},
		{"ローカル推論", openaiPkg.Usage{Provider: "local", Model: "gpt-oss-20b", PromptTokens: 100}, ""},
		// Web 検索のツール料だけの行は、トークン単価を使っていないので根拠を付けない
		{"トークン0（ツール料だけ）", openaiPkg.Usage{Provider: "openai", Model: "gpt-4o-mini", WebSearchCalls: 1}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apiCallLogFromUsage(tc.u, time.Now()).PriceBasis; got != tc.want {
				t.Errorf("PriceBasis = %q, want %q", got, tc.want)
			}
		})
	}
}

// 内訳では、記録した時点の根拠を使い、記録の無い行もあるときは今の単価表での判定と比べて注意が必要なほうを出す（#2004）。
func TestSummaryPriceBasis_PrefersRecorded(t *testing.T) {
	cases := []struct {
		name, model string
		cost        float64
		recorded    string
		unrecorded  bool
		want        string
	}{
		// 記録時は推定値だったが、今は公開価格になったモデル。過去の行の表示は記録時の値のまま
		{"記録時の推定値を優先", "gpt-4o-mini", 1.0, PriceBasisEstimated, false, PriceBasisEstimated},
		{"記録時の公開価格を優先", "totally-new-model", 1.0, PriceBasisListed, false, PriceBasisListed},
		{"記録が無ければ今の単価表", "totally-new-model", 1.0, "", true, PriceBasisUnknown},
		// トークン単価を使った行が無い（Web 検索のツール料だけ）なら、今の単価表の根拠も付けない（#2166 の Codex のレビュー）
		{"トークン単価を使った行が無ければ付けない", "totally-new-model", 1.0, "", false, ""},
		// 古い行と新しい行が混ざる期間は、注意が必要なほうを出す
		{"混在: 今の単価表のほうが注意が必要", "totally-new-model", 1.0, PriceBasisListed, true, PriceBasisUnknown},
		{"混在: 記録のほうが注意が必要", "gpt-4o-mini", 1.0, PriceBasisInherited, true, PriceBasisInherited},
		{"混在: 同じなら記録の値", "gpt-4-1106-preview", 1.0, PriceBasisEstimated, true, PriceBasisEstimated},
		{"金額0なら記録があっても付けない", "gpt-4o-mini", 0, PriceBasisUnknown, false, ""},
		{"音声のモデルは記録があっても付けない", "gpt-4o-mini-tts", 1.0, PriceBasisInherited, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := summaryPriceBasis(tc.model, tc.cost, tc.recorded, tc.unrecorded); got != tc.want {
				t.Errorf("summaryPriceBasis = %q, want %q", got, tc.want)
			}
		})
	}
}
