package aibench

import (
	"fmt"
	"io"
	"math"
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

	// 1件も計測できていない実行で指標を出さない（#1634）。
	//
	// 残高切れで全件が失敗した実行でも「破損率 0.0% / 弁別力 +0.000」という
	// 体裁の整った表が出ていた。どちらも品質が良いときと見分けが付かない数字で、
	// 設定の問題を品質の問題と読み違える。指標ではなく失敗として出す。
	if s.MeasuredRuns == 0 {
		fmt.Fprintf(w, "\n計測できた呼び出しが1件もありません。指標は表示しません。\n")
		fmt.Fprintf(w, "計測できず %d回%s\n", s.UnmeasuredRuns, formatCounts(s.BrokenByReason))
		fmt.Fprintf(w, "APIキー・残高・ネットワークを確認してから測り直してください。\n")
		return
	}

	fmt.Fprintf(w, "\n%-16s %s\n", "指標", "値")
	fmt.Fprintf(w, "%-16s %.1f%%%s\n", "破損率", s.BrokenRate*100, formatCounts(s.BrokenByReason))
	if s.UnmeasuredRuns > 0 {
		fmt.Fprintf(w, "%-16s %d回（通信エラー等。破損率とレイテンシからは除外）\n", "計測できず", s.UnmeasuredRuns)
	}
	fmt.Fprintf(w, "%-16s 平均 %.3f / 最大 %.3f%s\n", "再現性(σ)", s.MeanScoreStdDev, s.MaxScoreStdDev,
		formatUnstable(s.UnstableCaseIDs))
	fmt.Fprintf(w, "%-16s %+.3f（1.000が完全一致）%s\n", "弁別力(順位相関)",
		s.LabelRankCorrelation, formatByLabel(s.MeanScoreByLabel))
	// 交絡は弁別力の直下に必ず3行で出す。弁別力だけを見ると、長さを測っている
	// だけの測定を「内容の質を測れている」と読んでしまう（#1593）。
	//
	// 「ラベルvs文字数」を省くと「スコアvs文字数」の大小が読めない。長さに
	// 完全に盲目な採点器なら「スコアvs文字数」は 弁別力×「ラベルvs文字数」
	// 付近に出るので、その積を下回っていれば長さが押し上げている。
	fmt.Fprintf(w, "%-16s %+.3f%s\n", "交絡(スコアvs文字数)", s.LengthRankCorrelation,
		lengthWarning(s.LabelRankCorrelation, s.LengthRankCorrelation))
	fmt.Fprintf(w, "%-16s %+.3f（セット側の交絡。長さに盲目な採点器なら上は %+.3f 付近）\n",
		"ラベルvs文字数", s.LabelLengthRankCorrelation,
		s.LabelRankCorrelation*s.LabelLengthRankCorrelation)
	fmt.Fprintf(w, "%-16s %.1f%%%s\n", "指示遵守率", s.ComplianceRate*100, formatCounts(s.ViolationCounts))
	fmt.Fprintf(w, "%-16s p50 %dms / p95 %dms\n", "レイテンシ", s.LatencyP50MS, s.LatencyP95MS)
	costNote := ""
	if s.CostIsEstimated {
		costNote = "（トークン数が概算）"
	}
	fmt.Fprintf(w, "%-16s 1件 $%.5f / 合計 $%.4f%s\n", "コスト", s.CostPerCallUSD, s.TotalCostUSD, costNote)

	if len(s.LengthStrata) > 0 {
		// 層化は交絡を自動で消さない。層内でもラベルと文字数が相関していれば
		// 同じ曖昧さが残るので、層内の「ラベルvs文字数」を必ず併記する。
		// それが小さい層でだけ「長さでは説明できない」と読む。
		fmt.Fprintf(w, "\n文字数で層化した弁別力（層内の「ラベルvs文字数」が小さい層だけが根拠になる）\n")
		fmt.Fprintf(w, "%-14s %5s %-22s %8s %12s %s\n",
			"層(文字数)", "件数", "ラベル構成", "順位相関", "ラベルvs文字数", "平均スコア")
		for _, st := range s.LengthStrata {
			fmt.Fprintf(w, "%-14s %5d %-22s %+8.3f %+12.3f %s\n",
				fmt.Sprintf("%d-%d", st.MinChars, st.MaxChars), st.Cases,
				formatLabelCounts(st.LabelCounts), st.LabelRankCorrelation,
				st.LabelLengthRankCorrelation,
				strings.TrimSpace(formatByLabel(st.MeanScoreByLabel)))
		}
	}

	fmt.Fprintf(w, "\n%-14s %-6s %6s %8s %8s %6s\n", "ケース", "ラベル", "破損", "平均", "σ", "文字数")
	for _, c := range s.CaseDetails {
		fmt.Fprintf(w, "%-14s %-6s %6s %8.3f %8.3f %6d\n", c.CaseID, c.Label,
			fmt.Sprintf("%d/%d", c.BrokenRuns, c.Runs), c.MeanScore, c.ScoreStdDev, c.InputChars)
	}
}

// lengthWarning は文字数との相関がラベルとの相関に迫っているときに注意書きを返す。
//
// 閾値 0.1 は「実質的に同じ」と読む幅。ラベル相関 +0.95 に対して文字数相関 +0.95 なら、
// 「内容の質を測れている」仮説と「長さを測っているだけ」仮説が同じ数字を予測するので、
// その弁別力は前者の証拠にならない。
//
// 絶対値で比べるのは、負の交絡も同じく交絡だから。ラベル相関 +1.00 に対して
// 文字数相関 −1.00 は「スコアが長さだけで決まっている」状態で、符号だけ見て
// 通すとその実行を無警告で根拠に使ってしまう。
func lengthWarning(labelCorr, lengthCorr float64) string {
	if math.Abs(lengthCorr) < math.Abs(labelCorr)-0.1 {
		return ""
	}
	return "  ⚠ ラベルとの相関に迫っている。弁別力を「内容の質」の証拠として読めない"
}

func formatLabelCounts(m map[string]int) string {
	parts := make([]string, 0, 3)
	for _, k := range []string{LabelGood, LabelMid, LabelBad} {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, " ")
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
)

// コストは比率ではなく月額換算の絶対増で判定する（#1598）。
// 比率だけだと安い工程の +$0.00026/件（1.42倍）が赤になり、高い工程の
// +$0.0095/件（1.19倍）が緑になって、判断を取り違える。
// 想定件数は全工程で共通の暫定値。実運用の件数が分かったら更新する。
const (
	assumedMonthlyCalls = 10000 // 1工程あたりの想定月間処理件数
	costWorseMonthlyUSD = 10.0  // 月額換算でこれ以上増えたら劣化
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
	// 交絡も劣化ゲートに入れる。弁別力だけをゲートにすると、ゲートが
	// 「どんな手段でも弁別力を上げろ」を報酬にしてしまい、長さへの依存を
	// 強めた変更が緑で通る（#1593 が止めたかったことそのもの）。
	// 絶対値で比べるのは負の交絡も交絡だから（lengthWarning と同じ理由）。
	if lengthComparable(prev, cur) {
		add("交絡(スコアvs文字数)", math.Abs(prev.LengthRankCorrelation), math.Abs(cur.LengthRankCorrelation),
			correlationWorseBy, true)
	} else {
		warnings = append(warnings, "交絡(vs文字数)は比較できない（#1593 より前の結果で未計測。0.000 を交絡なしと読まないこと）")
	}
	add("再現性(平均σ)", prev.MeanScoreStdDev, cur.MeanScoreStdDev, stddevWorseBy, true)

	if prev.LatencyP95MS > 0 && float64(cur.LatencyP95MS) >= float64(prev.LatencyP95MS)*latencyWorseByRatio-eps {
		regs = append(regs, Regression{Metric: "レイテンシ(p95)", Prev: float64(prev.LatencyP95MS), Cur: float64(cur.LatencyP95MS),
			Reason: fmt.Sprintf("%.2f倍に増えた", float64(cur.LatencyP95MS)/float64(prev.LatencyP95MS))})
	}
	// 前回が $0 でも絶対増は判定できる。ゲートを比率から絶対増に変えた時点で
	// 「前回 > 0」の前提は要らなくなったが、比率時代のガードが残っていて、
	// ローカルモデルや無料経路の $0 と比べると月 +$500 でも緑で通っていた。
	// これはこのゲートが見逃してはいけない増加そのものである（#1598）。
	if !costComparable(prev, cur) {
		warnings = append(warnings, "コストは比較できない（前回は1件も計測できておらず、$0 は「無料だった」ではなく「測れていない」）")
	} else {
		perCall := cur.CostPerCallUSD - prev.CostPerCallUSD
		monthly := perCall * assumedMonthlyCalls
		if monthly >= costWorseMonthlyUSD-eps {
			// 前回が $0 だと比率は定義できない。+Inf倍と出すより理由から落とす。
			ratio := ""
			if prev.CostPerCallUSD > 0 {
				ratio = fmt.Sprintf("%.2f倍・", cur.CostPerCallUSD/prev.CostPerCallUSD)
			}
			regs = append(regs, Regression{Metric: "コスト(1件)", Prev: prev.CostPerCallUSD, Cur: cur.CostPerCallUSD,
				Reason: fmt.Sprintf("%s1件 %+.5f$・月%d件で %+.2f$ が閾値 %.2f$ 以上",
					ratio, perCall, assumedMonthlyCalls, monthly, costWorseMonthlyUSD)})
		}
	}
	return regs, warnings
}

// lengthComparable は交絡の指標を両側で比較できるかを返す。
//
// #1593 より前の結果JSONは length_rank_correlation を持たず、デコードすると
// 0.000 になる。それを「交絡なし」として比べると 0.000 → 0.920 (+0.920) と出て
// 必ず劣化を誤検知する。未計測が片側にでもあれば比較しない。
func lengthComparable(prev, cur *Summary) bool {
	return prev.LengthMetricsMeasured && cur.LengthMetricsMeasured
}

// costComparable は前回のコストを比較に使えるかを返す。
//
// 1件も計測できていない実行（#1634）の $0 は「無料だった」ではなく
// 「測れていない」。そのまま比べると今回のコスト全額が増加として出て、
// 設定の問題を費用の劣化と読み違える。
//
// 確かめるのは前回が $0 のときだけでよい。$0 より大きければ計測できているし、
// 計測できた実行の $0（無料経路・ローカルモデル）は比較に使える。
// 今回も $0 なら増加のしようが無いので、警告を出さず比較できるものとして扱う。
// コストを記録していない古い結果JSONどうしを並べたときに、増えてもいないのに
// 毎回「比較できない」と出るのを避ける。
func costComparable(prev, cur *Summary) bool {
	return prev.CostPerCallUSD > 0 || prev.MeasuredRuns > 0 || cur.CostPerCallUSD == 0
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
	// 未計測の側があるときは行自体を出さない。0.000 と並べると「交絡が無かった
	// のに増えた」と読める表になる。
	if lengthComparable(prev, cur) {
		row("交絡(スコアvs文字数)", prev.LengthRankCorrelation, cur.LengthRankCorrelation, false)
		row("ラベルvs文字数", prev.LabelLengthRankCorrelation, cur.LabelLengthRankCorrelation, false)
	}
	row("再現性(平均σ)", prev.MeanScoreStdDev, cur.MeanScoreStdDev, false)
	fmt.Fprintf(w, "%-18s %10d %10d %+10d\n", "p95(ms)", prev.LatencyP95MS, cur.LatencyP95MS, cur.LatencyP95MS-prev.LatencyP95MS)
	// 前回が未計測のときは行自体を出さない。交絡（#1593）と同じ理由で、
	// 「0.00 → 500.00 +500.00」と並べながら「劣化は検知されませんでした」と
	// 続く表は、警告を読み飛ばすと増加を見逃したようにも、無視したようにも読める。
	if costComparable(prev, cur) {
		fmt.Fprintf(w, "%-18s %10.5f %10.5f %+10.5f\n", "コスト/件($)", prev.CostPerCallUSD, cur.CostPerCallUSD, cur.CostPerCallUSD-prev.CostPerCallUSD)
		fmt.Fprintf(w, "%-18s %10.2f %10.2f %+10.2f\n", fmt.Sprintf("月%d件換算($)", assumedMonthlyCalls),
			prev.CostPerCallUSD*assumedMonthlyCalls, cur.CostPerCallUSD*assumedMonthlyCalls,
			(cur.CostPerCallUSD-prev.CostPerCallUSD)*assumedMonthlyCalls)
	}

	if len(regs) == 0 {
		fmt.Fprintln(w, "\n劣化は検知されませんでした")
		return false
	}
	fmt.Fprintln(w, "\n劣化を検知しました:")
	for _, r := range regs {
		// %.3f だと 1件 $0.00062 → $0.00088 が「0.001 → 0.001」になり、
		// なぜ劣化と判定したかが読めない。%g は有効数字で出すので、
		// サブセントのコストも ms 単位のレイテンシも同じ書式で扱える（#1598）。
		fmt.Fprintf(w, "  %s: %.5g → %.5g（%s）\n", r.Metric, r.Prev, r.Cur, r.Reason)
	}
	return true
}
