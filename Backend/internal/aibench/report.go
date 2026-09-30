package aibench

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// PrintSummary は集計結果を人が読める形で出す。
//
// LLM の出力本文は出さない。ES・履歴書・面接ログは学生本人の記述であり、
// ゴールデンセットが合成データでも、出力をターミナルやCIログへ流す運用は作らない。
// 内容の確認は -out の JSON（リポジトリ外）で行う。
func PrintSummary(w io.Writer, s *Summary) {
	fmt.Fprintf(w, "\n=== %s / model=%s ===\n", s.Target, s.Model)
	fmt.Fprintf(w, "実行日時 %s  呼び出し先 %s\n", s.GeneratedAt, s.Endpoint)
	if s.ManifestSHA256 != "" {
		fmt.Fprintf(w, "入力 %s (sha256 %s)\n", s.ManifestPath, s.ManifestSHA256[:12])
	}
	fmt.Fprintf(w, "件数 %d × %d回 = %d呼び出し\n", s.Cases, s.Runs, len(s.Observations))

	fmt.Fprintf(w, "\n%-16s %s\n", "指標", "値")
	fmt.Fprintf(w, "%-16s %.1f%%%s\n", "破損率", s.BrokenRate*100, formatCounts(s.BrokenByReason))
	if s.UnmeasuredRuns > 0 {
		fmt.Fprintf(w, "%-16s %d回（通信エラー等。破損率とレイテンシからは除外）\n", "計測できず", s.UnmeasuredRuns)
	}
	fmt.Fprintf(w, "%-16s 平均 %.3f / 最大 %.3f%s\n", "再現性(σ)", s.MeanScoreStdDev, s.MaxScoreStdDev,
		formatUnstable(s.UnstableCaseIDs))
	fmt.Fprintf(w, "%-16s %+.3f（1.000が完全一致）%s\n", "弁別力(順位相関)",
		s.LabelRankCorrelation, formatByLabel(s.MeanScoreByLabel))
	fmt.Fprintf(w, "%-16s %.1f%%%s\n", "指示遵守率", s.ComplianceRate*100, formatCounts(s.ViolationCounts))
	fmt.Fprintf(w, "%-16s p50 %dms / p95 %dms\n", "レイテンシ", s.LatencyP50MS, s.LatencyP95MS)
	costNote := ""
	if s.CostIsEstimated {
		costNote = "（トークン数が概算）"
	}
	fmt.Fprintf(w, "%-16s 1件 $%.5f / 合計 $%.4f%s\n", "コスト", s.CostPerCallUSD, s.TotalCostUSD, costNote)

	fmt.Fprintf(w, "\n%-14s %-6s %6s %8s %8s\n", "ケース", "ラベル", "破損", "平均", "σ")
	for _, c := range s.CaseDetails {
		fmt.Fprintf(w, "%-14s %-6s %6s %8.3f %8.3f\n", c.CaseID, c.Label,
			fmt.Sprintf("%d/%d", c.BrokenRuns, c.Runs), c.MeanScore, c.ScoreStdDev)
	}
}

func formatCounts(m map[string]int) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return "  [" + strings.Join(parts, " ") + "]"
}

func formatByLabel(m map[string]float64) string {
	if len(m) == 0 {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, k := range []string{LabelGood, LabelMid, LabelBad} {
		if v, ok := m[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%.3f", k, v))
		}
	}
	return "  [" + strings.Join(parts, " ") + "]"
}

func formatUnstable(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return fmt.Sprintf("  [σ>%.2f: %s]", UnstableStdDevThreshold, strings.Join(ids, ","))
}

// Regression は前回実行からの劣化1件。
type Regression struct {
	Metric string
	Prev   float64
	Cur    float64
	Reason string
}

// 劣化とみなす変化量。ここを緩めると気付けず、厳しくすると毎回赤くなる。
// 温度0.2〜0.4で回すため、同じプロンプトでも実行ごとに数%は動く。
const (
	brokenRateWorseBy   = 0.05 // 破損率が5ポイント以上増えた
	complianceWorseBy   = 0.05 // 指示遵守率が5ポイント以上減った
	correlationWorseBy  = 0.10 // 順位相関が0.1以上下がった
	stddevWorseBy       = 0.05 // 再現性のσが0.05以上増えた
	latencyWorseByRatio = 1.20 // p95 が1.2倍以上になった
	costWorseByRatio    = 1.20 // 1件あたりコストが1.2倍以上になった
)

// Diff は前回結果と比べて劣化を洗い出す。
//
// 差分を取る前に target とマニフェストの一致を確認する。別の入力で測った
// 結果を並べても意味が無いどころか、「改善した」という誤った結論を出す。
func Diff(prev, cur *Summary) (regs []Regression, warnings []string) {
	if prev.Target != cur.Target {
		warnings = append(warnings, fmt.Sprintf("target が違う（前回 %s / 今回 %s）", prev.Target, cur.Target))
	}
	if prev.ManifestSHA256 != "" && cur.ManifestSHA256 != "" && prev.ManifestSHA256 != cur.ManifestSHA256 {
		warnings = append(warnings, "マニフェストが違う（入力が変わっているため数値の比較は不可）")
	}
	if prev.Model != cur.Model {
		warnings = append(warnings, fmt.Sprintf("モデルが違う（前回 %s / 今回 %s）", prev.Model, cur.Model))
	}
	if prev.Cases != cur.Cases {
		warnings = append(warnings, fmt.Sprintf("件数が違う（前回 %d / 今回 %d）", prev.Cases, cur.Cases))
	}

	// 閾値ちょうどの差は劣化として扱う。0.15-0.10 が 0.049999... になる類の
	// 誤差で判定が変わると、同じ数字でも実行ごとに検知結果が変わる。
	const eps = 1e-9
	add := func(metric string, p, c, limit float64, higherIsWorse bool) {
		worse := c-p >= limit-eps
		if !higherIsWorse {
			worse = p-c >= limit-eps
		}
		if worse {
			regs = append(regs, Regression{Metric: metric, Prev: p, Cur: c,
				Reason: fmt.Sprintf("変化量 %.3f が閾値 %.3f を超えた", absDiff(p, c), limit)})
		}
	}
	add("破損率", prev.BrokenRate, cur.BrokenRate, brokenRateWorseBy, true)
	add("指示遵守率", prev.ComplianceRate, cur.ComplianceRate, complianceWorseBy, false)
	add("弁別力(順位相関)", prev.LabelRankCorrelation, cur.LabelRankCorrelation, correlationWorseBy, false)
	add("再現性(平均σ)", prev.MeanScoreStdDev, cur.MeanScoreStdDev, stddevWorseBy, true)

	if prev.LatencyP95MS > 0 && float64(cur.LatencyP95MS) >= float64(prev.LatencyP95MS)*latencyWorseByRatio-eps {
		regs = append(regs, Regression{Metric: "レイテンシ(p95)", Prev: float64(prev.LatencyP95MS), Cur: float64(cur.LatencyP95MS),
			Reason: fmt.Sprintf("%.2f倍に増えた", float64(cur.LatencyP95MS)/float64(prev.LatencyP95MS))})
	}
	if prev.CostPerCallUSD > 0 && cur.CostPerCallUSD >= prev.CostPerCallUSD*costWorseByRatio-eps {
		regs = append(regs, Regression{Metric: "コスト(1件)", Prev: prev.CostPerCallUSD, Cur: cur.CostPerCallUSD,
			Reason: fmt.Sprintf("%.2f倍に増えた", cur.CostPerCallUSD/prev.CostPerCallUSD)})
	}
	return regs, warnings
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

// PrintDiff は前回との比較を出す。戻り値は劣化を検知したか。
func PrintDiff(w io.Writer, prev, cur *Summary) bool {
	regs, warnings := Diff(prev, cur)
	fmt.Fprintf(w, "=== 比較: %s ===\n", cur.Target)
	fmt.Fprintf(w, "前回 %s (model=%s)\n今回 %s (model=%s)\n", prev.GeneratedAt, prev.Model, cur.GeneratedAt, cur.Model)
	for _, m := range warnings {
		fmt.Fprintf(w, "⚠ %s\n", m)
	}

	fmt.Fprintf(w, "\n%-18s %10s %10s %10s\n", "指標", "前回", "今回", "差")
	row := func(name string, p, c float64, pct bool) {
		if pct {
			fmt.Fprintf(w, "%-18s %9.1f%% %9.1f%% %+9.1f%%\n", name, p*100, c*100, (c-p)*100)
			return
		}
		fmt.Fprintf(w, "%-18s %10.3f %10.3f %+10.3f\n", name, p, c, c-p)
	}
	row("破損率", prev.BrokenRate, cur.BrokenRate, true)
	row("指示遵守率", prev.ComplianceRate, cur.ComplianceRate, true)
	row("弁別力(順位相関)", prev.LabelRankCorrelation, cur.LabelRankCorrelation, false)
	row("再現性(平均σ)", prev.MeanScoreStdDev, cur.MeanScoreStdDev, false)
	fmt.Fprintf(w, "%-18s %10d %10d %+10d\n", "p95(ms)", prev.LatencyP95MS, cur.LatencyP95MS, cur.LatencyP95MS-prev.LatencyP95MS)
	fmt.Fprintf(w, "%-18s %10.5f %10.5f %+10.5f\n", "コスト/件($)", prev.CostPerCallUSD, cur.CostPerCallUSD, cur.CostPerCallUSD-prev.CostPerCallUSD)

	if len(regs) == 0 {
		fmt.Fprintln(w, "\n劣化は検知されませんでした")
		return false
	}
	fmt.Fprintln(w, "\n劣化を検知しました:")
	for _, r := range regs {
		fmt.Fprintf(w, "  %s: %.3f → %.3f（%s）\n", r.Metric, r.Prev, r.Cur, r.Reason)
	}
	return true
}
