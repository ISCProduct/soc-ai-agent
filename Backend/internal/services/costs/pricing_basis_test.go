package costs

import (
	"bytes"
	"log"
	"maps"
	"strings"
	"testing"
)

// #2004: 推定値と実価格を表の構造で区別し、単価表に無いモデルが黙って集計されないようにする。

func TestPriceBasis(t *testing.T) {
	cases := []struct{ model, want string }{
		{"gpt-4o-mini", PriceBasisListed},
		{"gpt-4o-mini-2024-07-18", PriceBasisListed},
		{"GPT-4o", PriceBasisListed},
		{"gpt-5.2", PriceBasisEstimated},
		{"gpt-5.2-2025-12-11", PriceBasisEstimated},
		{"gpt-4.1-mini", PriceBasisUnknown},
		{"gpt-4o-realtime-preview", PriceBasisListed}, // gpt-4o の前方一致に当たる（単価はモデル自体の値ではない点に注意）
		{"totally-new-model", PriceBasisUnknown},
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
	}
	for _, tc := range cases {
		if got := summaryPriceBasis(tc.model, tc.cost); got != tc.want {
			t.Errorf("summaryPriceBasis(%q, %v) = %q, want %q", tc.model, tc.cost, got, tc.want)
		}
	}
}
