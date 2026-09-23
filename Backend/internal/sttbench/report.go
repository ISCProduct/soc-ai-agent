package sttbench

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// PrintSummary はモデル単位の結果を人が読める形で出す。
//
// 認識結果の本文は出さない。学生の発話が入りうる経路と同じ扱いに揃える。
// 失敗したケースのIDだけを示し、本文はJSON出力（リポジトリ外）で確認する。
func PrintSummary(w io.Writer, model string, s *ModelSummary) {
	fmt.Fprintf(w, "%-26s %8s %8s %10s %10s %6s %8s\n", "ケース", "CER", "意味CER", "固有名詞", "数値", "失敗", "ms")
	for _, c := range s.Cases {
		kw := "-"
		if n := len(c.KeywordsHit) + len(c.KeywordsMiss); n > 0 {
			kw = fmt.Sprintf("%d/%d", len(c.KeywordsHit), n)
		}
		num := "-"
		if c.NumbersTotal > 0 {
			num = fmt.Sprintf("%d/%d", c.NumbersMatched, c.NumbersTotal)
		}
		mark := ""
		if c.Failed {
			mark = "✗"
		}
		fmt.Fprintf(w, "%-26s %8.3f %8.3f %10s %10s %6s %8d\n", c.ID, c.CER, c.SemanticCER, kw, num, mark, c.LatencyMS)
	}
	fmt.Fprintf(w, "  平均CER %.3f(意味 %.3f) / 固有名詞 %.1f%% / 数値 %.1f%% / 失敗率 %.1f%% / 平均 %dms\n",
		s.MeanCER, s.MeanSemanticCER, s.KeywordAccuracy*100, s.NumberAccuracy*100, s.FailureRate*100, s.MeanLatencyMS)

	// 落としたキーワードは、どの語で失敗したかが分かると対策に直結する
	miss := map[string]int{}
	for _, c := range s.Cases {
		for _, k := range c.KeywordsMiss {
			miss[k]++
		}
	}
	if len(miss) > 0 {
		keys := make([]string, 0, len(miss))
		for k := range miss {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s(%d)", k, miss[k]))
		}
		fmt.Fprintf(w, "  取り逃した語: %s\n", strings.Join(parts, ", "))
	}

	// 由来別（合成 / 実発話）。合成だけの結果を本番品質の根拠にしないため、
	// 実発話が混ざっているときは分けて見せる（#1484）。
	printGroups(w, "由来別", s.BySource)
	// 録音条件別。母数が小さい条件が全体平均に埋もれるのを防ぐ。
	printGroups(w, "録音条件別", s.ByCondition)
}

// printGroups は内訳を1グループ1行で出す。母数なしの指標は "-" で示す。
func printGroups(w io.Writer, title string, groups []GroupSummary) {
	// 1グループしか無い（＝全部同じ由来/条件）なら、全体平均と同じなので出さない
	if len(groups) <= 1 {
		return
	}
	fmt.Fprintf(w, "  [%s]\n", title)
	for _, g := range groups {
		fmt.Fprintf(w, "    %-16s 件数%3d  CER %.3f  固有名詞 %s  数値 %s  失敗率 %.1f%%\n",
			g.Key, g.Cases, g.MeanCER,
			pctOrDash(g.KeywordAccuracy), pctOrDash(g.NumberAccuracy), g.FailureRate*100)
	}
}

// pctOrDash は -1（母数なし）を "-" に、それ以外を百分率にする。
func pctOrDash(v float64) string {
	if v < 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", v*100)
}

// PrintComparison はモデル間の差を並べる。
// 「どちらが安いか」ではなく「何を失うか」が読み取れる並びにする。
func PrintComparison(w io.Writer, models []string, r Report) {
	fmt.Fprintf(w, "\n=== 比較 ===\n")
	fmt.Fprintf(w, "%-26s %8s %8s %10s %8s %8s %12s\n",
		"モデル", "平均CER", "意味CER", "固有名詞", "数値", "失敗率", "$/面接30分")
	for _, m := range models {
		m = strings.TrimSpace(m)
		s := r.Models[m]
		if s == nil {
			continue
		}
		// 面接30分ぶんの発話を10分と仮定した概算。
		// 学生が話す時間は面接時間そのものではない。
		const speechMinPerInterview = 10.0
		fmt.Fprintf(w, "%-26s %8.3f %8.3f %9.1f%% %7.1f%% %7.1f%% %12.4f\n",
			m, s.MeanCER, s.MeanSemanticCER, s.KeywordAccuracy*100, s.NumberAccuracy*100,
			s.FailureRate*100, s.EstCostPerMinUSD*speechMinPerInterview)
	}
}
