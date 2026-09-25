package costs

import (
	"math"
	"testing"

	"Backend/internal/openai"
)

// TestCalculateCost_LongestPrefixWins は単価表の前方一致が最長一致になることを固定する（#1124）。
//
// 以前は map を range して最初に一致したもので break していた。Go の map の反復順序は
// ランダムなので "gpt-4o-mini" が "gpt-4o" に当たると 16.7倍の単価で記録され、
// しかも実行ごとに結果が変わっていた。実測（本番相当DB）では gpt-4o-mini の
// 892万入力トークンが $26.32 として記録されており、正しくは $1.79 だった。
func TestCalculateCost_LongestPrefixWins(t *testing.T) {
	const inTok, outTok = 1_000_000, 1_000_000

	tests := []struct {
		name  string
		model string
		want  float64
	}{
		// 長い方のキーに当たること（短い方に食われない）
		{name: "gpt-4o-mini は mini 単価", model: "gpt-4o-mini", want: 0.15 + 0.60},
		{name: "gpt-4o は 4o 単価", model: "gpt-4o", want: 2.50 + 10.00},
		{name: "gpt-4-turbo は turbo 単価", model: "gpt-4-turbo", want: 10.00 + 30.00},
		{name: "gpt-4 は gpt-4 単価", model: "gpt-4", want: 30.00 + 60.00},
		{name: "o1-mini は o1-mini 単価", model: "o1-mini", want: 3.00 + 12.00},
		{name: "o1 は o1 単価", model: "o1", want: 15.00 + 60.00},
		// バージョン付き・日付付きも最長一致
		{name: "日付付き mini", model: "gpt-4o-mini-2024-07-18", want: 0.15 + 0.60},
		{name: "日付付き 4o", model: "gpt-4o-2024-08-06", want: 2.50 + 10.00},
		{name: "検索プレビュー mini", model: "gpt-4o-mini-search-preview", want: 0.15 + 0.60},
		{name: "検索プレビュー 4o", model: "gpt-4o-search-preview", want: 2.50 + 10.00},
		{name: "gpt-5-search-api", model: "gpt-5-search-api", want: 2.50 + 10.00},
		// 大文字・空白
		{name: "大文字は正規化される", model: "  GPT-4O-MINI  ", want: 0.15 + 0.60},
		// 未知モデルは過小評価しない
		{name: "未知モデルは gpt-4o 単価", model: "some-unknown-model", want: 2.50 + 10.00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCost("openai", tt.model, inTok, outTok)
			if got != tt.want {
				t.Errorf("calculateCost(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// TestCalculateCost_IsDeterministic は同じ入力が常に同じ結果になることを確認する。
// map の反復順序に依存していた頃は、同一プロセス内でも呼び出しごとに変わり得た。
func TestCalculateCost_IsDeterministic(t *testing.T) {
	const runs = 200
	first := calculateCost("openai", "gpt-4o-mini", 1_000_000, 0)
	for i := range runs {
		if got := calculateCost("openai", "gpt-4o-mini", 1_000_000, 0); got != first {
			t.Fatalf("%d 回目で結果が変わった: %v != %v", i+1, got, first)
		}
	}
	// mini 単価であることも併せて固定する（決定的だが間違った値、を防ぐ）
	if first != 0.15 {
		t.Errorf("gpt-4o-mini の入力100万トークン = %v, want 0.15", first)
	}
}

// TestModelPricing_NoAmbiguousEntryIsCheaperThanItsPrefix は
// 「短いキーの方が高い」組み合わせを検出する。
//
// 最長一致にしたので現状は問題ないが、単価表に短いキーを足したときに
// 既存の長いキーが食われる事故を早期に気づけるようにする。
func TestModelPricing_LongerKeyResolvesToItself(t *testing.T) {
	for model := range modelPricing {
		t.Run(model, func(t *testing.T) {
			want := modelPricing[model]
			got := calculateCost("openai", model, 1_000_000, 1_000_000)
			expected := want[0] + want[1]
			if got != expected {
				t.Errorf("%q が自分の単価に解決されていない: %v, want %v", model, got, expected)
			}
		})
	}
}

// TestUsageCostUSD_WebSearchToolFee は web_search のツール料が課金額に入ることを固定する。
//
// web_search はトークンとは別に1コール単位($10/1,000コール)で課金される。
// calculateCost はトークン単価しか持っていなかったため、検索コストの85%を
// 占めるツール料が api_call_logs に一切現れていなかった。実測では 2026-08 の
// web_search 772 コールぶん $7.72 が記録から抜けていた。
//
// モデル名では判別できない（同じ gpt-4o-mini に通常のチャットが混ざる）ので、
// 呼び出し回数を持つ WebSearchCalls で判定する。
func TestUsageCostUSD_WebSearchToolFee(t *testing.T) {
	// gpt-4o-mini で web_search 1回ぶんの典型的な形。
	// 入力8,000トークンは検索結果の固定課金分。
	const inTok, outTok = 9000, 500
	tokenCost := float64(inTok)*0.15/1_000_000 + float64(outTok)*0.60/1_000_000
	const toolFee = 0.010

	tests := []struct {
		name  string
		usage openai.Usage
		want  float64
	}{
		{
			name:  "web_search なしはトークン課金のみ",
			usage: openai.Usage{Model: "gpt-4o-mini", PromptTokens: inTok, CompletionTokens: outTok, Provider: "openai"},
			want:  tokenCost,
		},
		{
			name:  "web_search 1回でツール料が乗る",
			usage: openai.Usage{Model: "gpt-4o-mini", PromptTokens: inTok, CompletionTokens: outTok, Provider: "openai", WebSearchCalls: 1},
			want:  tokenCost + toolFee,
		},
		{
			name:  "web_search 2回なら2回ぶん",
			usage: openai.Usage{Model: "gpt-4o-mini", PromptTokens: inTok, CompletionTokens: outTok, Provider: "openai", WebSearchCalls: 2},
			want:  tokenCost + 2*toolFee,
		},
		{
			name:  "provider 未設定は OpenAI 扱いでツール料が乗る",
			usage: openai.Usage{Model: "gpt-4o-mini", PromptTokens: inTok, CompletionTokens: outTok, WebSearchCalls: 1},
			want:  tokenCost + toolFee,
		},
		{
			name:  "ローカル推論はツール料もトークン課金も 0",
			usage: openai.Usage{Model: "gpt-oss-20b", PromptTokens: inTok, CompletionTokens: outTok, Provider: "local", WebSearchCalls: 1},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usageCostUSD(tt.usage)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("usageCostUSD() = %.9f, want %.9f", got, tt.want)
			}
		})
	}
}

// TestUsageCostUSD_WebSearchDominatesTokens は「モデルを安くしても検索コストは
// 下がらない」ことを数字で固定する。ツール料が抜けているとこの事実が見えない。
func TestUsageCostUSD_WebSearchDominatesTokens(t *testing.T) {
	u := openai.Usage{Model: "gpt-4o-mini", PromptTokens: 8000, CompletionTokens: 0, Provider: "openai", WebSearchCalls: 1}
	total := usageCostUSD(u)
	tokens := 8000 * 0.15 / 1_000_000

	if share := (total - tokens) / total; share < 0.8 {
		t.Errorf("ツール料の占める割合 = %.2f, want >= 0.80（合計 %.6f / トークン %.6f）", share, total, tokens)
	}
}
