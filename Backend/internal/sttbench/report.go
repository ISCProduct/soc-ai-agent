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
		// APIエラーと認識失敗は別物なので記号を分ける。
		// 「APIが落ちていた」のか「聞き取れなかった」のかで打つ手が違う。
		mark := ""
		switch {
		case c.Errored:
			mark = "E"
		case c.Failed:
			mark = "✗"
		}
		fmt.Fprintf(w, "%-26s %8.3f %8.3f %10s %10s %6s %8d\n", c.ID, c.CER, c.SemanticCER, kw, num, mark, c.LatencyMS)
	}
	// 平均の母数は「APIが成功し、かつ認識失敗でもない件数」。
	// APIエラー件数を必ず添えるのは、残高切れで大半が落ちた run を
	// 「CERが改善した」と読まないため。
	scored := len(s.Cases) - s.ErroredCases
	fmt.Fprintf(w, "  平均CER %.3f(意味 %.3f) / 固有名詞 %s / 数値 %s / 認識失敗率 %.1f%% / 平均 %dms\n",
		s.MeanCER, s.MeanSemanticCER, pctOrDash(s.KeywordAccuracy), pctOrDash(s.NumberAccuracy),
		s.FailureRate*100, s.MeanLatencyMS)
	fmt.Fprintf(w, "  APIエラー %d件（平均の母数は %d件 / 全 %d件）\n", s.ErroredCases, scored, len(s.Cases))
	switch s.CostBasis {
	case "local_api_only_excludes_compute":
		fmt.Fprintln(w, "  推論先: local（API費は0。計算資源・電力の費用は含まない）")
	case "unknown_model_price":
		fmt.Fprintln(w, "  推論先: OpenAI（単価未登録のため費用は未算出）")
	default:
		fmt.Fprintf(w, "  推論先: %s（OpenAI 公開単価による概算）\n", s.Provider)
	}
	if s.Aborted {
		fmt.Fprintf(w, "  ⚠ 4xx が %d件連続したため打ち切りました。この結果で判断しないこと\n", maxConsecutive4xx)
	}

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
		fmt.Fprintf(w, "    %-16s 件数%3d(APIエラー%3d)  CER %.3f  固有名詞 %s  数値 %s  認識失敗率 %.1f%%\n",
			g.Key, g.Cases, g.Errored, g.MeanCER,
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
	// 平均の母数（CER等を計算できた件数）とAPIエラー件数を並べる。
	// CER だけを横に並べると、落ちた run のほうが良く見えて判断を誤る。
	fmt.Fprintf(w, "%-26s %8s %8s %10s %8s %10s %8s %8s %12s\n",
		"モデル", "平均CER", "意味CER", "固有名詞", "数値", "認識失敗率", "母数", "APIエラー", "$/面接30分(API)")
	for _, m := range models {
		m = strings.TrimSpace(m)
		s := r.Models[m]
		if s == nil {
			continue
		}
		// 面接30分ぶんの発話を10分と仮定した概算。
		// 学生が話す時間は面接時間そのものではない。
		const speechMinPerInterview = 10.0
		cost := "-"
		if s.CostBasis == "local_api_only_excludes_compute" {
			cost = "0.0000*"
		} else if s.CostBasis == "openai_published_price" {
			cost = fmt.Sprintf("%.4f", s.EstCostPerMinUSD*speechMinPerInterview)
		}
		fmt.Fprintf(w, "%-26s %8.3f %8.3f %10s %8s %9.1f%% %8d %8d %12s\n",
			m, s.MeanCER, s.MeanSemanticCER, pctOrDash(s.KeywordAccuracy), pctOrDash(s.NumberAccuracy),
			s.FailureRate*100, len(s.Cases)-s.ErroredCases, s.ErroredCases, cost)
	}
	hasLocalProvider := false
	for _, summary := range r.Models {
		if summary != nil && summary.CostBasis == "local_api_only_excludes_compute" {
			hasLocalProvider = true
			break
		}
	}
	if hasLocalProvider {
		fmt.Fprintln(w, "* local の金額はAPI費のみ。計算資源・電力の費用を含まない。")
	}
}
