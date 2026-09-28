package flywheel

import (
	"fmt"
	"math"
	"strings"

	"Backend/internal/models"
)

// 履歴書レビュー・面接レポートから user_weight_scores への変換（#1528）。
//
// 以前の実装は次の点で根拠が薄かった。
//   - 履歴書は score >= 70 のときだけ加点し、70未満は何も書かなかった（上方バイアス）
//   - 履歴書の総合スコアを「技術志向」へ流していた（書類の完成度と技術志向を結ぶ根拠が無い）
//   - 加点時に 100 を超える値をリポジトリへ渡し、リポジトリ側の clamp に救われていた
//   - 面接はルーブリック 0〜5 を ×20 するだけで、取り得る値が6段階しか無かった
//
// ここでは写像を純関数として1箇所に置き、対応の根拠を各行のコメントに残す。
// DB に触らないので、テストと後述のドライランから同じ式を確認できる。

// neutralScore は「情報が無い」ときの中立値。10カテゴリはすべて 0〜100 で、
// マッチングは |ユーザー - 企業| で距離を測るため中央の 50 が無情報点になる。
const neutralScore = 50

// categoryScore は写像結果（カテゴリ名は正典キーのみ）。
type categoryScore struct {
	category string
	score    int
}

// ── 履歴書レビュー → 10カテゴリ ───────────────────────────────────────────

// resumeScoreMapping は履歴書レビューの総合スコアを反映するカテゴリと強さ。
//
// weight は「レビュースコアの動きをどれだけそのカテゴリへ伝えるか」で、
// 1.0 ならスコア全域（0〜100）をそのまま、0.6 なら中立50寄りに圧縮して伝える。
// 説明できないカテゴリは載せない。載っていないカテゴリは履歴書では動かさない。
var resumeScoreMapping = []struct {
	category string
	weight   float64
}{
	// 細部志向: レビュー指摘の主対象は誤字脱字・書式崩れ・年月や数値の不整合であり、
	// 書類の完成度は「細部への注意」がほぼそのまま現れる。等倍で反映する。
	{"細部志向", 1.0},
	// コミュニケーション力: 職務経歴書は読み手に伝えるための文書で、構成・簡潔さ・
	// 根拠の示し方がレビュー観点に含まれる。ただし測れるのは書面での伝達だけで
	// 対話は含まれないため、中立50寄りに圧縮して弱く反映する。
	{"コミュニケーション力", 0.6},
	// 技術志向には反映しない（#1528）。書類の完成度と技術への関心・専門性の深さを
	// 結び付ける根拠が無い。以前は総合スコアをそのまま流していたが、
	// 書類が綺麗なだけで技術志向が高いことにされ、マッチングを歪めていた。
}

// critical 指摘のペナルティ。1件あたり criticalPenaltyPerItem 点引き、
// criticalPenaltyMax で止める。上限を置くのは、critical が10件あっても
// 「書類が荒い」以上の情報は増えず、レビュースコア本体の寄与を消し切って
// 全員を0点に潰してしまうため。
const (
	criticalPenaltyPerItem = 5
	criticalPenaltyMax     = 20
)

// mapResumeScore は履歴書レビューの総合スコアと critical 件数から
// 各カテゴリの反映値（0〜100）を返す。
//
// 式: 50 + (score - 50) * weight - penalty * weight
// 中立50を基準に上下対称なので、スコアが低ければ低く反映される。
func mapResumeScore(score, criticalCount int) []categoryScore {
	score = clampInt(score, 0, 100)
	penalty := min(max(criticalCount, 0)*criticalPenaltyPerItem, criticalPenaltyMax)

	out := make([]categoryScore, 0, len(resumeScoreMapping))
	for _, m := range resumeScoreMapping {
		v := neutralScore + (float64(score-neutralScore)-float64(penalty))*m.weight
		out = append(out, categoryScore{
			category: m.category,
			// 呼び出し側でも 0〜100 を守る。リポジトリ側の clamp に頼ると
			// 「意図して100を超えた値を渡している」のか事故なのか読めなくなる。
			score: clampInt(int(math.Round(v)), 0, 100),
		})
	}
	return out
}

// ── 面接レポート → 10カテゴリ ─────────────────────────────────────────────

// 面接スコアの分解能を上げるための係数（#1528）。
//
// 主軸はルーブリック 0〜5（×20）で、補正はルーブリック1段の半分（±5点）に留める。
// 補正でルーブリックの順序が入れ替わらないため、「4より3の方が高く出る」ことは起きない。
const (
	rubricStep = 20 // ルーブリック1段 = 20点
	// interviewAdjustSpan は補正の全幅。signal 0〜1 を ±(span/2) に割り当てる。
	interviewAdjustSpan = rubricStep / 2 // ±5点
	// 「十分」と見なす量。これ以上は頭打ちにする。
	evidenceFullRunes = 120 // 根拠発言の引用として十分な長さ
	answerFullRunes   = 80  // 1回答あたりの十分な長さ
	answerFullTurns   = 6   // 面接として十分なやり取りの回数
	// 補正の内訳。evidence は項目ごとの情報なのでターン数より重く見る。
	evidenceSignalWeight  = 0.6
	verbositySignalWeight = 0.4
)

// InterviewTranscriptStats は面接の発話から得られる連続量。
// レポート本体（InterviewReport）には残らないので、発話を持つ呼び出し元が組み立てる。
type InterviewTranscriptStats struct {
	UserTurns      int // 受験者の発話ターン数
	AvgAnswerRunes int // 受験者1発話あたりの平均文字数
}

// NewInterviewTranscriptStats は発話ログから補正用の統計を作る。
func NewInterviewTranscriptStats(utterances []models.InterviewUtterance) InterviewTranscriptStats {
	turns, total := 0, 0
	for _, u := range utterances {
		if u.Role != "user" {
			continue
		}
		text := strings.TrimSpace(u.Text)
		if text == "" {
			continue
		}
		turns++
		total += len([]rune(text))
	}
	if turns == 0 {
		return InterviewTranscriptStats{}
	}
	return InterviewTranscriptStats{UserTurns: turns, AvgAnswerRunes: total / turns}
}

// mapInterviewScore はルーブリックスコア（0〜5）を 0〜100 へ写す。
//
// ルーブリックが主軸で、evidence の量と回答量で ±5点だけ動かす。
// 6段階しか取れなかった値が band ごとに11段階になり、マッチングの入力として使える。
func mapInterviewScore(rubric int, evidence string, stats InterviewTranscriptStats) int {
	base := clampInt(rubric, 0, 5) * rubricStep
	// signal 0.5 を無補正の基準点にする。
	adjusted := float64(base) + (interviewSignal(evidence, stats)-0.5)*float64(interviewAdjustSpan)
	return clampInt(int(math.Round(adjusted)), 0, 100)
}

// interviewSignal は補正量の元になる 0〜1 の連続量を返す。
//
// evidence は「有無と長さ」だけを見る。内容が実際の発話と一致しているかの照合は
// #1527 で実装中で、現時点の evidence には未検証のものが混ざる。
// 内容の正しさに依存させると、その未検証の文章がスコアを動かしてしまうため、
// #1527 がマージされるまでは長さのみを使う。
func interviewSignal(evidence string, stats InterviewTranscriptStats) float64 {
	ev := signalRatio(len([]rune(strings.TrimSpace(evidence))), evidenceFullRunes)

	if stats.UserTurns <= 0 {
		// 発話統計が無いのは「話していない」ではなく「記録が無い」。
		// 0 扱いで減点すると記録漏れが受験者の不利になるので中立(0.5)にする。
		return evidenceSignalWeight*ev + verbositySignalWeight*0.5
	}
	verbosity := 0.5*signalRatio(stats.AvgAnswerRunes, answerFullRunes) +
		0.5*signalRatio(stats.UserTurns, answerFullTurns)
	return evidenceSignalWeight*ev + verbositySignalWeight*verbosity
}

// signalRatio は v/full を 0〜1 に収めて返す。
func signalRatio(v, full int) float64 {
	if v <= 0 || full <= 0 {
		return 0
	}
	return math.Min(float64(v)/float64(full), 1)
}

// ── 移動平均 ─────────────────────────────────────────────────────────────

// 移動平均の重み。1回の面接・レビューでプロファイルを塗り替えないための緩和。
const (
	movingAvgExistingWeight = 0.7
	movingAvgNewWeight      = 0.3
)

// blendScore は既存値と新しい値を移動平均で混ぜる（0〜100 に収める）。
func blendScore(existing, newValue int) int {
	blended := float64(existing)*movingAvgExistingWeight + float64(newValue)*movingAvgNewWeight
	return clampInt(int(math.Round(blended)), 0, 100)
}

// ── ドライラン ───────────────────────────────────────────────────────────

// FormatScoreMappingDryRun は新旧の写像を並べた表を返す（DB には一切触らない）。
//
// 入力（履歴書の総合スコアと critical 件数、面接のルーブリックと発話量）の全域を
// 掃くので、既存行を読んで分布を取るより変化を漏れなく見られる。
// `go test ./internal/services/flywheel/ -run DryRun -v` で出力を確認する。
func FormatScoreMappingDryRun() string {
	var b strings.Builder

	b.WriteString("=== 履歴書レビュー → user_weight_scores ===\n")
	b.WriteString("score critical | 旧(細部/コミュ/技術)     | 新(細部/コミュ)\n")
	for _, score := range []int{0, 30, 50, 70, 85, 100} {
		for _, crit := range []int{0, 3, 5} {
			b.WriteString(fmt.Sprintf("%5d %8d | %-24s | %s\n",
				score, crit, formatLegacyResume(score, crit), formatCategoryScores(mapResumeScore(score, crit))))
		}
	}

	b.WriteString("\n=== 面接レポート → user_weight_scores ===\n")
	b.WriteString("rubric evidence文字数 平均回答文字数 ターン | 旧 | 新\n")
	for _, rubric := range []int{0, 1, 2, 3, 4, 5} {
		for _, c := range []struct {
			evidenceRunes int
			stats         InterviewTranscriptStats
		}{
			{0, InterviewTranscriptStats{}},
			{0, InterviewTranscriptStats{UserTurns: 2, AvgAnswerRunes: 15}},
			{60, InterviewTranscriptStats{UserTurns: 5, AvgAnswerRunes: 50}},
			{200, InterviewTranscriptStats{UserTurns: 10, AvgAnswerRunes: 120}},
		} {
			evidence := strings.Repeat("あ", c.evidenceRunes)
			b.WriteString(fmt.Sprintf("%6d %14d %14d %6d | %3d | %3d\n",
				rubric, c.evidenceRunes, c.stats.AvgAnswerRunes, c.stats.UserTurns,
				rubric*rubricStep, mapInterviewScore(rubric, evidence, c.stats)))
		}
	}
	return b.String()
}

// formatLegacyResume は修正前の履歴書写像が書いていた値を再現する（比較用）。
// 旧実装: score>=70 のとき 3カテゴリへ score+round((score-70)/3)、
// critical>=3 のとき 2カテゴリへ score-10*critical。どちらでもなければ無書き込み。
func formatLegacyResume(score, criticalCount int) string {
	var parts []string
	if score >= 70 {
		v := clampInt(score+int(math.Round(float64(score-70)/3)), 0, 100)
		parts = append(parts, fmt.Sprintf("加点 %d/%d/%d", v, v, v))
	}
	if criticalCount >= 3 {
		v := clampInt(score-10*criticalCount, 0, 100)
		parts = append(parts, fmt.Sprintf("減点 %d/%d/-", v, v))
	}
	if len(parts) == 0 {
		return "書き込みなし"
	}
	return strings.Join(parts, " ")
}

func formatCategoryScores(scores []categoryScore) string {
	parts := make([]string, 0, len(scores))
	for _, s := range scores {
		parts = append(parts, fmt.Sprintf("%d", s.score))
	}
	return strings.Join(parts, "/")
}
