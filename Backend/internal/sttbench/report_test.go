package sttbench

import (
	"slices"
	"strings"
	"testing"
)

// 全件 API エラーの出力が「完璧な結果」に見えないことを文字列で固定する。
func TestPrintSummary_母数0の指標はダッシュで出す(t *testing.T) {
	s := &ModelSummary{
		Cases:           []CaseResult{{ID: "a", Errored: true}},
		ErroredCases:    1,
		KeywordAccuracy: -1,
		NumberAccuracy:  -1,
		CostBasis:       "openai_published_price",
		Provider:        "openai",
		ByCondition: []GroupSummary{
			{Key: "clean", Cases: 1, Errored: 1, KeywordAccuracy: -1, NumberAccuracy: -1},
			{Key: "noisy", Cases: 2, Errored: 0, MeanCER: 0.25, KeywordAccuracy: -1, NumberAccuracy: -1, FailureRate: 0.5},
		},
	}
	var b strings.Builder
	PrintSummary(&b, "m", s)
	got := b.String()

	for _, want := range []string{
		"  平均CER -(意味 -) / 固有名詞 - / 数値 - / 認識失敗率 - / 平均 -\n",
		"  APIエラー 1件（平均の母数は 0件 / 全 1件）\n",
		"CER -  固有名詞 -  数値 -  認識失敗率 -\n",
		"CER 0.250  固有名詞 -  数値 -  認識失敗率 50.0%\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q がありません:\n%s", want, got)
		}
	}
	if strings.Contains(got, "0.000") {
		t.Errorf("母数0なのに 0.000 が出ています:\n%s", got)
	}
}

func TestPrintSummary_母数があれば値を出す(t *testing.T) {
	s := &ModelSummary{
		Cases:           []CaseResult{{ID: "a", CER: 0.1, SemanticCER: 0.05, LatencyMS: 12}},
		MeanCER:         0.1,
		MeanSemanticCER: 0.05,
		KeywordAccuracy: -1,
		NumberAccuracy:  -1,
		MeanLatencyMS:   12,
	}
	var b strings.Builder
	PrintSummary(&b, "m", s)

	want := "  平均CER 0.100(意味 0.050) / 固有名詞 - / 数値 - / 認識失敗率 0.0% / 平均 12ms\n"
	if !strings.Contains(b.String(), want) {
		t.Errorf("出力に %q がありません:\n%s", want, b.String())
	}
}

func TestPrintComparison_母数0のモデルをダッシュで出す(t *testing.T) {
	r := Report{Models: map[string]*ModelSummary{
		"down": {Cases: []CaseResult{{ID: "a", Errored: true}}, ErroredCases: 1, KeywordAccuracy: -1, NumberAccuracy: -1},
	}}
	var b strings.Builder
	PrintComparison(&b, []string{"down"}, r)

	if strings.Contains(b.String(), "0.000") || strings.Contains(b.String(), "0.0%") {
		t.Errorf("母数0なのに 0 の指標が出ています:\n%s", b.String())
	}
}

func TestUnevaluatedModels(t *testing.T) {
	errored := CaseResult{ID: "e", Errored: true}
	ok := CaseResult{ID: "o"}
	tests := []struct {
		name   string
		models map[string]*ModelSummary
		want   []string
	}{
		{name: "全件成功", models: map[string]*ModelSummary{"m": {Cases: []CaseResult{ok}}}},
		{name: "一部だけAPIエラーなら評価できている", models: map[string]*ModelSummary{"m": {Cases: []CaseResult{ok, errored}, ErroredCases: 1}}},
		{
			name: "全件APIエラーのモデルを名前順に返す",
			models: map[string]*ModelSummary{
				"z":  {Cases: []CaseResult{errored}, ErroredCases: 1},
				"a":  {Cases: []CaseResult{errored, errored}, ErroredCases: 2},
				"ok": {Cases: []CaseResult{ok}},
			},
			want: []string{"a", "z"},
		},
		{name: "ケースが無いモデルも評価できていない", models: map[string]*ModelSummary{"m": {}}, want: []string{"m"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UnevaluatedModels(Report{Models: tt.models}); !slices.Equal(got, tt.want) {
				t.Errorf("UnevaluatedModels = %v, want %v", got, tt.want)
			}
		})
	}
}

// 分単価が公式の概算（est.）のモデルも費用を出し、目安であることを説明に書く（#2229 のレビュー）。
func TestPrintReport_公開単価の目安(t *testing.T) {
	s := &ModelSummary{
		Cases:            []CaseResult{{ID: "a", CER: 0.1}},
		MeanCER:          0.1,
		KeywordAccuracy:  -1,
		NumberAccuracy:   -1,
		Provider:         "openai",
		CostBasis:        "openai_estimated_price",
		EstCostPerMinUSD: 0.003,
	}
	var b strings.Builder
	PrintSummary(&b, "gpt-4o-mini-transcribe", s)
	if !strings.Contains(b.String(), "OpenAI 公開単価の目安") {
		t.Errorf("目安であることが説明に無い:\n%s", b.String())
	}

	var c strings.Builder
	PrintComparison(&c, []string{"gpt-4o-mini-transcribe"}, Report{Models: map[string]*ModelSummary{"gpt-4o-mini-transcribe": s}})
	// 発話10分ぶん × $0.003/分
	if !strings.Contains(c.String(), "0.0300") {
		t.Errorf("目安の単価でも面接1回の費用を出す:\n%s", c.String())
	}
}
